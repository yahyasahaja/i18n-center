package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

// BootstrapHandler handles application-level bulk seeding from a locale JSON file.
type BootstrapHandler struct {
	translationService *services.TranslationService
	auditService       *services.AuditService
}

func NewBootstrapHandler() *BootstrapHandler {
	return &BootstrapHandler{
		translationService: services.NewTranslationService(),
		auditService:       services.NewAuditService(),
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
	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageDraft
	}

	applicationID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	// Verify application exists.
	var app models.Application
	if err := database.DB.First(&app, "id = ?", applicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
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

	// Upsert each component and append a new translation version.
	for rawCode, data := range objectComponents {
		code := strings.TrimSpace(strings.ToLower(rawCode))

		var component models.Component
		lookupErr := database.DB.
			Where("application_id = ? AND code = ?", applicationID, code).
			First(&component).Error

		if lookupErr != nil {
			// Component does not exist — create it.
			component = models.Component{
				ApplicationID: applicationID,
				Code:          code,
				CreatedBy:     userID,
			}
			if createErr := database.DB.Create(&component).Error; createErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to create component '" + code + "': " + createErr.Error(),
				})
				return
			}
			result.ComponentsCreated++
		} else {
			result.ComponentsUpdated++
		}

		jsonData := models.JSONB(data)
		if _, saveErr := h.translationService.SaveTranslation(component.ID, locale, stage, jsonData, userID); saveErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save translation for '" + code + "': " + saveErr.Error(),
			})
			return
		}

		result.KeysImported += len(data)
		result.Components = append(result.Components, code)
	}

	ipAddress, userAgent := c.ClientIP(), c.GetHeader("User-Agent")
	h.auditService.LogCreate(userID, "", "bootstrap", applicationID, app.Code, result, ipAddress, userAgent)

	c.JSON(http.StatusOK, result)
}
