package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/observability"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/application"
	"github.com/your-org/i18n-center/repository/component"
	"github.com/your-org/i18n-center/repository/localedeploy"
	"github.com/your-org/i18n-center/repository/translation"
	"github.com/your-org/i18n-center/services"
)

// BootstrapHandler handles application-level bulk seeding from a locale JSON file.
type BootstrapHandler struct {
	translationService *services.TranslationService
	auditService       services.AuditServicer
	apps               application.Repository
	components         component.Repository
	deploys            localedeploy.Repository
}

func NewBootstrapHandler() *BootstrapHandler {
	return &BootstrapHandler{
		translationService: services.NewTranslationService(),
		auditService:       services.NewAuditService(),
		apps:               application.New(),
		components:         component.New(),
		deploys:            localedeploy.New(),
	}
}

// BootstrapRequest is the request body for bootstrapping an application.
// The top-level keys of Data map to component codes:
//   - Object values  → one component per key (e.g. "paymentPage": { ... })
//   - Primitive values (string/number/bool) → grouped into a "common" component
type BootstrapRequest struct {
	Data map[string]interface{} `json:"data" binding:"required"`
}

// BootstrapResult summarises what was created/updated during the bootstrap.
type BootstrapResult struct {
	ComponentsCreated int      `json:"components_created"`
	ComponentsUpdated int      `json:"components_updated"`
	KeysImported      int      `json:"keys_imported"`
	FlatKeysInCommon  int      `json:"flat_keys_in_common"`
	Components        []string `json:"components"`
}

// BootstrapApplication seeds an application with translations from a single locale JSON.
//
// @Summary      Bootstrap application translations
// @Description  Bulk-creates components and seeds draft translations from a locale JSON.
//
//	Object values at the top level become one component each.
//	Primitive (string/number/bool) values are grouped under a "common" component.
//	Existing components are upserted (translation version added, component metadata preserved).
//
// @Tags         bootstrap
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string            true   "Application ID (UUID)"
// @Param        locale  query     string            false  "Locale to seed (default: en)"
// @Param        stage   query     string            false  "Target stage (default: draft)"
// @Param        request body      BootstrapRequest  true   "Full locale JSON keyed by component code"
// @Success      200     {object}  BootstrapResult
// @Failure      400     {object}  map[string]string
// @Failure      401     {object}  map[string]string
// @Failure      404     {object}  map[string]string
// @Failure      500     {object}  map[string]string
// @Router       /applications/{id}/bootstrap [post]
func (h *BootstrapHandler) BootstrapApplication(c *gin.Context) {
	appIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		locale = "en"
	}
	stage := translation.Stage(stageStr)
	if stage == "" {
		stage = translation.StageDraft
	}

	applicationID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	ctx := c.Request.Context()

	// Verify application exists.
	app, err := h.apps.GetByID(ctx, database.SQLX, applicationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req BootstrapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDVal, _ := c.Get("user_id")
	var userID uuid.UUID
	if idStr, ok := userIDVal.(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			userID = id
		}
	}

	result := BootstrapResult{Components: []string{}}

	// Separate object-valued keys from primitive-valued keys.
	// Primitives are folded into a synthetic "common" component so the
	// caller's flat root-level keys (e.g. "copy", "generalErrorMessage")
	// are preserved without polluting the component namespace.
	objectComponents := make(map[string]map[string]interface{})
	flatKeys := make(map[string]interface{})

	for key, value := range req.Data {
		switch v := value.(type) {
		case map[string]interface{}:
			objectComponents[key] = v
		default:
			flatKeys[key] = v
		}
	}

	if len(flatKeys) > 0 {
		objectComponents["common"] = flatKeys
		result.FlatKeysInCommon = len(flatKeys)
	}

	// Upsert each component and append a new translation version. The lookup
	// uses GetByCode which is application-scoped on the unique index. New
	// components are persisted via component.Repository.Create.
	for rawCode, data := range objectComponents {
		code := strings.TrimSpace(strings.ToLower(rawCode))

		var compID uuid.UUID
		existing, lookupErr := h.components.GetByCode(ctx, database.SQLX, code)
		switch {
		case lookupErr == nil && existing != nil && existing.ApplicationID == applicationID:
			compID = existing.ID
			result.ComponentsUpdated++
		default:
			newComp := &component.Component{
				ApplicationID: applicationID,
				Code:          code,
				CreatedBy:     userID,
			}
			createErr := h.components.Create(ctx, database.SQLX, newComp)
			if createErr == nil {
				compID = newComp.ID
				result.ComponentsCreated++
				break
			}
			// Race: another bootstrap request created the same component
			// between our lookup and our insert. Re-fetch and proceed.
			if errors.Is(createErr, repository.ErrConflict) {
				if again, err := h.components.GetByCode(ctx, database.SQLX, code); err == nil && again != nil {
					compID = again.ID
					result.ComponentsUpdated++
					break
				}
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to create component '" + code + "': " + createErr.Error(),
			})
			return
		}

		jsonData := repository.JSONB(data)
		if _, saveErr := h.translationService.SaveTranslation(compID, locale, stage, jsonData, userID); saveErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save translation for '" + code + "': " + saveErr.Error(),
			})
			return
		}

		result.KeysImported += len(data)
		result.Components = append(result.Components, code)
	}

	// Auto-enable the imported locale on the application. Without this, the
	// translation editor's Locale dropdown comes up empty after a bootstrap —
	// the row exists in translation_versions but the application's
	// enabled_languages slice doesn't know about it, so the FE has nothing to
	// render. Case-insensitive dedupe so re-running with the same locale
	// doesn't duplicate the entry.
	enabledLocale := strings.TrimSpace(strings.ToLower(locale))
	languageWasAdded := false
	if enabledLocale != "" {
		alreadyEnabled := false
		for _, l := range app.EnabledLanguages {
			if strings.EqualFold(l, enabledLocale) {
				alreadyEnabled = true
				break
			}
		}
		if !alreadyEnabled {
			newLangs := append(append([]string(nil), app.EnabledLanguages...), enabledLocale)
			if err := h.apps.UpdateEnabledLanguages(ctx, database.SQLX, applicationID, newLangs, userID); err != nil {
				// Non-fatal: components + translations are already persisted.
				// We log so the failure is visible — silently swallowing
				// hid this exact bug earlier when the cache layer masked it.
				observability.Logger.Warn("bootstrap: failed to auto-enable locale",
					zap.String("application_id", applicationID.String()),
					zap.String("locale", enabledLocale),
					zap.Error(err),
				)
			} else {
				languageWasAdded = true
			}
		}
	}

	// Upsert the per-locale deploy ledger row. Without this row, the Pending
	// Deployments view shows "nothing to promote" even after a successful
	// bootstrap — that screen reads from application_locale_deploys, not from
	// translation_versions, so the absence of a ledger entry hides genuinely
	// promotable data.
	//
	// stage_completed = the highest stage this locale has been promoted to.
	// On bootstrap, we set it to the stage that was just imported:
	//   * bootstrap --stage=draft       → stage_completed=draft (needs deploy → staging → production)
	//   * bootstrap --stage=staging     → stage_completed=staging (needs deploy → production)
	//   * bootstrap --stage=production  → stage_completed=production (terminal)
	//
	// Re-bootstrapping with new data BUMPS the ledger BACK to the imported
	// stage — same convention the AddLanguage worker uses when re-translate
	// invalidates an already-deployed locale.
	if enabledLocale != "" {
		deploy := &localedeploy.Deploy{
			ApplicationID:  applicationID,
			Locale:         enabledLocale,
			StageCompleted: string(stage),
		}
		if err := h.deploys.Upsert(ctx, database.SQLX, deploy); err != nil {
			observability.Logger.Warn("bootstrap: failed to upsert deploy ledger",
				zap.String("application_id", applicationID.String()),
				zap.String("locale", enabledLocale),
				zap.String("stage", string(stage)),
				zap.Error(err),
			)
		}
	}

	// Bust the application cache so the next GetApplication doesn't return the
	// pre-bootstrap snapshot. Both branches (components-only change AND
	// enabled_languages change) need this — without it the per-app page shows
	// "Enabled Languages: empty" even though the DB has the locale, and the
	// 1-hour TTL means the user has to wait or manually edit the app to
	// invalidate. We invalidate by both id and code since GetApplication
	// caches under whichever identifier was used.
	cache.Delete(cache.ApplicationKey(applicationID.String()))
	cache.Delete(cache.ApplicationKey(app.Code))
	_ = languageWasAdded // available for future telemetry; logging covers user-visible needs

	ipAddress, userAgent := c.ClientIP(), c.GetHeader("User-Agent")
	h.auditService.LogCreate(userID, "", "bootstrap", applicationID, app.Code, result, ipAddress, userAgent)

	c.JSON(http.StatusOK, result)
}
