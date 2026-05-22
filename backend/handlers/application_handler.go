package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/jobs"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/application"
	"github.com/your-org/i18n-center/repository/component"
	"github.com/your-org/i18n-center/repository/job"
	"github.com/your-org/i18n-center/repository/localedeploy"
	"github.com/your-org/i18n-center/repository/translation"
	"github.com/your-org/i18n-center/services"
)

// pqStringArray is a local alias so the file can reference `pq.StringArray`
// in inline struct definitions without `pq.` clutter at use-sites.
type pqStringArray = pq.StringArray

type ApplicationHandler struct {
	auditService  services.AuditServicer
	apps          application.Repository
	components    component.Repository
	addLangJobs   job.AddLanguageRepository
	translateJobs job.TranslateRepository
	deploys       localedeploy.Repository
}

func NewApplicationHandler() *ApplicationHandler {
	return &ApplicationHandler{
		auditService:  services.NewAuditService(),
		apps:          application.New(),
		components:    component.New(),
		addLangJobs:   job.NewAddLanguageRepository(),
		translateJobs: job.NewTranslateRepository(),
		deploys:       localedeploy.New(),
	}
}

// getCurrentUser extracts user info from context (set by auth middleware)
func (h *ApplicationHandler) getCurrentUser(c *gin.Context) (userID uuid.UUID, username string) {
	userIDVal, exists := c.Get("user_id")
	if exists {
		if idStr, ok := userIDVal.(string); ok {
			if id, err := uuid.Parse(idStr); err == nil {
				userID = id
			}
		}
	}

	usernameVal, exists := c.Get("username")
	if exists {
		if name, ok := usernameVal.(string); ok {
			username = name
		}
	}

	return userID, username
}

// getClientInfo extracts IP address and user agent from request
func (h *ApplicationHandler) getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.GetHeader("User-Agent")
	return ipAddress, userAgent
}

// GetApplications lists all applications
// @Summary      List applications
// @Description  Get all applications
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.Application
// @Failure      401  {object}  map[string]string
// @Router       /applications [get]
func (h *ApplicationHandler) GetApplications(c *gin.Context) {
	apps, err := h.apps.List(c.Request.Context(), database.SQLX)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range apps {
		apps[i].PopulateComputed()
	}
	c.JSON(http.StatusOK, apps)
}

// GetApplication gets a single application (by ID or code).
// Cache hits short-circuit the DB lookup. Tries UUID-shape first; falls back
// to GetByCode when the param doesn't parse as UUID, so the same endpoint
// serves both `/applications/{uuid}` and `/applications/{code}`.
func (h *ApplicationHandler) GetApplication(c *gin.Context) {
	identifier := c.Param("id")

	cacheKey := cache.ApplicationKey(identifier)
	var cached application.Application
	if err := cache.Get(cacheKey, &cached); err == nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	ctx := c.Request.Context()
	var app *application.Application
	if id, parseErr := uuid.Parse(identifier); parseErr == nil {
		a, err := h.apps.GetByID(ctx, database.SQLX, id)
		if err == nil {
			app = a
		} else if !errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if app == nil {
		a, err := h.apps.GetByCode(ctx, database.SQLX, identifier)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		app = a
	}

	app.PopulateComputed()

	// Cache for 1 hour — same TTL as the legacy GORM path.
	cache.Set(cacheKey, *app, 3600*1000000000)
	c.JSON(http.StatusOK, app)
}

// ApplicationRequest represents the request payload for creating applications
type ApplicationRequest struct {
	Name             string   `json:"name" binding:"required"`
	Code             string   `json:"code" binding:"required"` // Unique identifier
	Description      string   `json:"description"`
	EnabledLanguages []string `json:"enabled_languages"`
	OpenAIKey        string   `json:"openai_key"` // Accept from frontend
}

// UpdateApplicationRequest represents the request payload for updating applications.
//
// EnabledLanguages is a pointer slice so the FE can omit it (preserve existing
// value) without sending an explicit empty array (which would clear the field).
// The locale management UI lives on the per-app detail page now; the
// application edit form no longer edits the languages list, so the FE just
// leaves the key out and we keep the previously-stored value.
type UpdateApplicationRequest struct {
	Name             string    `json:"name" binding:"required"`
	Code             string    `json:"code"` // Optional on update; keeps existing if omitted
	Description      string    `json:"description"`
	EnabledLanguages *[]string `json:"enabled_languages,omitempty"`
	OpenAIKey        string    `json:"openai_key"`
}

// CreateApplication creates a new application
// @Summary      Create application
// @Description  Create a new application
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        application  body      ApplicationRequest  true  "Application data"
// @Success      201         {object}  models.Application
// @Failure      400         {object}  map[string]string
// @Failure      401         {object}  map[string]string
// @Router       /applications [post]
func (h *ApplicationHandler) CreateApplication(c *gin.Context) {
	var req ApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	app := application.Application{
		Name:             req.Name,
		Code:             req.Code,
		Description:      req.Description,
		EnabledLanguages: req.EnabledLanguages,
		OpenAIKey:        req.OpenAIKey,
		CreatedBy:        userID,
		UpdatedBy:        userID,
	}

	if err := h.apps.Create(c.Request.Context(), database.SQLX, &app); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Application code already exists"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.auditService.LogCreate(
		userID,
		username,
		"application",
		app.ID,
		app.Code,
		app,
		ipAddress,
		userAgent,
	)

	app.PopulateComputed()
	c.JSON(http.StatusCreated, app)
}

// UpdateApplication updates an application
func (h *ApplicationHandler) UpdateApplication(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	ctx := c.Request.Context()
	app, err := h.apps.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Snapshot before-state for the audit log. OpenAIKey intentionally omitted
	// — never include secrets in audit payloads.
	before := application.Application{
		Name:             app.Name,
		Code:             app.Code,
		Description:      app.Description,
		EnabledLanguages: app.EnabledLanguages,
	}

	// Apply patch. Code stays unchanged if blank in the request (preserves the
	// legacy GORM behavior that a missing/empty code didn't clobber the existing one).
	app.Name = req.Name
	if req.Code != "" {
		app.Code = req.Code
	}
	app.Description = req.Description
	// EnabledLanguages is a sticky patch field: nil → preserve, non-nil → set.
	// This lets the FE leave it out entirely on edits while still allowing an
	// explicit `enabled_languages: []` to clear the list when actually intended.
	if req.EnabledLanguages != nil {
		app.EnabledLanguages = *req.EnabledLanguages
	}
	app.UpdatedBy = userID
	if req.OpenAIKey != "" {
		app.OpenAIKey = req.OpenAIKey
	}

	if err := h.apps.Update(ctx, database.SQLX, app); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Application code already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	after := application.Application{
		Name:             app.Name,
		Code:             app.Code,
		Description:      app.Description,
		EnabledLanguages: app.EnabledLanguages,
	}

	h.auditService.LogUpdate(
		userID,
		username,
		"application",
		app.ID,
		app.Code,
		before,
		after,
		ipAddress,
		userAgent,
	)

	app.PopulateComputed()
	cache.Delete(cache.ApplicationKey(idStr))
	c.JSON(http.StatusOK, app)
}

// DeleteApplication soft-deletes an application.
func (h *ApplicationHandler) DeleteApplication(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	ctx := c.Request.Context()
	app, err := h.apps.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	if err := h.apps.SoftDelete(ctx, database.SQLX, id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.auditService.LogDelete(
		userID,
		username,
		"application",
		app.ID,
		app.Code,
		app,
		ipAddress,
		userAgent,
	)

	cache.Delete(cache.ApplicationKey(idStr))
	c.JSON(http.StatusOK, gin.H{"message": "Application deleted"})
}

// AddLanguageRequest is the body for adding a new language to an application
type AddLanguageRequest struct {
	Locale        string `json:"locale" binding:"required"`
	AutoTranslate bool   `json:"auto_translate"`
}

// AddLanguage adds a new language to an application.
// When auto_translate is false: sync add locale, return 200.
// When auto_translate is true: add locale, create a job (worker will translate), return 202 with job_id for polling.
func (h *ApplicationHandler) AddLanguage(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	var req AddLanguageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Locale = strings.TrimSpace(strings.ToLower(req.Locale))
	if req.Locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is required"})
		return
	}

	userID, _ := h.getCurrentUser(c)
	ctx := c.Request.Context()

	app, err := h.apps.GetByID(ctx, database.SQLX, appID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reject duplicates before we touch any state.
	for _, l := range app.EnabledLanguages {
		if strings.EqualFold(l, req.Locale) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is already enabled"})
			return
		}
	}

	if req.AutoTranslate {
		// Idempotency check: reject if a pending/running job already exists,
		// and surface its job_id so the caller can poll.
		if existing, err := h.addLangJobs.FindActiveByLocale(ctx, database.SQLX, appID, req.Locale); err == nil && existing != nil {
			c.JSON(http.StatusConflict, gin.H{
				"error":      "A translation job for this locale is already in progress",
				"job_id":     existing.ID.String(),
				"status":     existing.Status,
				"status_url": fmt.Sprintf("/api/applications/%s/jobs/%s", appIDStr, existing.ID.String()),
			})
			return
		}

		// Validate an OpenAI key is available before queuing — failing fast here
		// beats a worker run that 500s mid-translate.
		openAIService := services.NewOpenAIService(app.OpenAIKey)
		if app.OpenAIKey == "" {
			openAIService = services.NewOpenAIService(services.GetDefaultOpenAIKey())
		}
		if openAIService.APIKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Auto-translate requires an OpenAI API key. Configure it in Application settings."})
			return
		}

		// Add locale first so it exists even if job creation fails — the locale itself
		// is the source of truth for "language enabled", the job is separate state.
		newLangs := append(app.EnabledLanguages, req.Locale)
		if err := h.apps.UpdateEnabledLanguages(ctx, database.SQLX, appID, newLangs, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update application: " + err.Error()})
			return
		}

		newJob := &job.AddLanguageJob{
			ApplicationID: appID,
			Locale:        req.Locale,
			AutoTranslate: true,
			CreatedBy:     userID,
		}
		if err := h.addLangJobs.Insert(ctx, database.SQLX, newJob); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job: " + err.Error()})
			return
		}

		cache.Delete(cache.ApplicationKey(appIDStr))
		c.Header("Location", fmt.Sprintf("/api/applications/%s/jobs/%s", appIDStr, newJob.ID.String()))
		c.JSON(http.StatusAccepted, gin.H{
			"message":    "Language added. Translation job queued.",
			"job_id":     newJob.ID.String(),
			"locale":     req.Locale,
			"status":     job.StatusPending,
			"status_url": fmt.Sprintf("/api/applications/%s/jobs/%s", appIDStr, newJob.ID.String()),
		})
		return
	}

	// Sync path: add locale only, no translation queued.
	newLangs := append(app.EnabledLanguages, req.Locale)
	if err := h.apps.UpdateEnabledLanguages(ctx, database.SQLX, appID, newLangs, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update application: " + err.Error()})
		return
	}
	cache.Delete(cache.ApplicationKey(appIDStr))
	c.JSON(http.StatusOK, gin.H{
		"message": "Language added",
		"locale":  req.Locale,
	})
}

// GetPendingDeploys returns locales that have draft/staging and can be deployed to the next stage
func (h *ApplicationHandler) GetPendingDeploys(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	deploys, err := h.deploys.ListPendingByApp(c.Request.Context(), database.SQLX, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list := make([]gin.H, 0, len(deploys))
	for _, d := range deploys {
		nextStage := ""
		if d.StageCompleted == "draft" {
			nextStage = "staging"
		} else if d.StageCompleted == "staging" {
			nextStage = "production"
		}
		list = append(list, gin.H{
			"locale":          d.Locale,
			"stage_completed": d.StageCompleted,
			"next_stage":      nextStage,
		})
	}
	c.JSON(http.StatusOK, gin.H{"pending_deploys": list})
}

// DeployLocaleRequest is the body for deploying a locale to the next stage
type DeployLocaleRequest struct {
	Locale string `json:"locale" binding:"required"`
}

// DeployLocale deploys a locale to the next stage (draft->staging or staging->production) for all components. Atomic: on any failure returns error so user can retry.
func (h *ApplicationHandler) DeployLocale(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	var req DeployLocaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Locale = strings.TrimSpace(strings.ToLower(req.Locale))

	userID, _ := h.getCurrentUser(c)
	ctx := c.Request.Context()

	deploy, err := h.deploys.GetByAppLocale(ctx, database.SQLX, appID, req.Locale)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "No pending deploy found for this locale"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var fromStage, toStage translation.Stage
	switch deploy.StageCompleted {
	case "draft":
		fromStage = translation.StageDraft
		toStage = translation.StageStaging
	case "staging":
		fromStage = translation.StageStaging
		toStage = translation.StageProduction
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is already fully deployed to production"})
		return
	}

	components, _, err := h.components.List(ctx, database.SQLX, component.ListFilter{ApplicationID: appID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Per-component translation deploys + the locale-deploys row update move
	// in lockstep — both are sqlx-backed now, so we wrap them in one tx via
	// repository.WithTx. Failure rolls back the whole promotion so the user
	// can safely retry without partial-state ambiguity.
	translationService := services.NewTranslationService()
	if err := repository.WithTx(ctx, database.SQLX, func(tx repository.Queryer) error {
		for _, comp := range components {
			// invalidateCache=false: caller invalidates after the outer tx commits.
			if err := translationService.DeployToStageTx(tx, comp.ID, req.Locale, fromStage, toStage, userID, false); err != nil {
				return fmt.Errorf("component %s: %w", comp.Code, err)
			}
		}
		return h.deploys.SetStage(ctx, tx, appID, req.Locale, string(toStage))
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Deploy failed — no changes persisted, safe to retry",
			"detail": err.Error(),
			"retry":  true,
		})
		return
	}

	// Tx committed — now invalidate caches for every component in this (locale, toStage) cell.
	for _, comp := range components {
		services.InvalidateAfterTranslationWrite(comp.ID, req.Locale, string(toStage))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Deployed %s to %s for all components", req.Locale, toStage),
		"locale":  req.Locale,
		"stage":   string(toStage),
	})
}

// GetAddLanguageJobStatus returns status of an add-language job (for polling after 202).
func (h *ApplicationHandler) GetAddLanguageJobStatus(c *gin.Context) {
	appIDStr := c.Param("id")
	jobIDStr := c.Param("job_id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	statusJob, err := jobs.GetJobStatus(appID, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if statusJob == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	resp := gin.H{
		"job_id":               statusJob.ID.String(),
		"locale":               statusJob.Locale,
		"status":               statusJob.Status,
		"total_components":     statusJob.TotalComponents,
		"completed_components": statusJob.CompletedComponents,
	}
	if statusJob.Status == job.StatusFailed {
		resp["error_message"] = statusJob.ErrorMessage
		resp["error_detail"] = statusJob.ErrorDetail
		resp["retry"] = true
	}
	c.JSON(http.StatusOK, resp)
}

// GetActiveJobs returns all pending/running AddLanguageJobs and TranslateJobs for an application.
func (h *ApplicationHandler) GetActiveJobs(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	ctx := c.Request.Context()

	// Add-language jobs — repo-backed, simple list.
	addJobs, err := h.addLangJobs.ListActiveByApp(ctx, database.SQLX, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Translate jobs — joined against components for display name/code. The
	// join isn't covered by either repository's contract (it's a UI-shaped
	// list), so we keep a one-off raw query inline rather than bloating the
	// repository surface with a method only this endpoint needs.
	type translateJobRow struct {
		JobID         string   `json:"job_id"`
		ComponentID   string   `json:"component_id"`
		ComponentCode string   `json:"component_code"`
		ComponentName string   `json:"component_name"`
		JobType       string   `json:"job_type"`
		TargetLocales []string `json:"target_locales"`
		Status        string   `json:"status"`
	}
	type rawRow struct {
		ID            uuid.UUID      `db:"id"`
		ComponentID   uuid.UUID      `db:"component_id"`
		ComponentCode string         `db:"component_code"`
		ComponentName string         `db:"component_name"`
		JobType       string         `db:"job_type"`
		TargetLocales pqStringArray  `db:"target_locales"`
		Status        string         `db:"status"`
	}
	const activeTranslateJobsQuery = `
		SELECT tj.id, tj.component_id, c.code AS component_code, c.name AS component_name,
		       tj.job_type, tj.target_locales, tj.status
		FROM translate_jobs tj
		JOIN components c ON c.id = tj.component_id AND c.deleted_at IS NULL
		WHERE tj.application_id = $1
		  AND tj.status IN ('pending', 'running')
		  AND tj.deleted_at IS NULL
		ORDER BY tj.created_at ASC
	`
	var rawRows []rawRow
	if err := database.SQLX.SelectContext(ctx, &rawRows, activeTranslateJobsQuery, appID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	translateRows := make([]translateJobRow, 0, len(rawRows))
	for _, r := range rawRows {
		targets := []string{}
		if r.TargetLocales != nil {
			targets = []string(r.TargetLocales)
		}
		translateRows = append(translateRows, translateJobRow{
			JobID:         r.ID.String(),
			ComponentID:   r.ComponentID.String(),
			ComponentCode: r.ComponentCode,
			ComponentName: r.ComponentName,
			JobType:       r.JobType,
			TargetLocales: targets,
			Status:        r.Status,
		})
	}

	addJobList := make([]gin.H, 0, len(addJobs))
	for _, j := range addJobs {
		addJobList = append(addJobList, gin.H{
			"job_id":               j.ID.String(),
			"locale":               j.Locale,
			"status":               j.Status,
			"total_components":     j.TotalComponents,
			"completed_components": j.CompletedComponents,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"add_language_jobs": addJobList,
		"translate_jobs":    translateRows,
	})
}

// DeleteLanguage removes a locale from an application: removes it from enabled_languages,
// deletes all translation versions for that locale for all components in the app, and the locale deploy record.
// Returns 400 if the locale is any component's default_locale.
func (h *ApplicationHandler) DeleteLanguage(c *gin.Context) {
	appIDStr := c.Param("id")
	localeParam := c.Param("locale")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	locale := strings.TrimSpace(strings.ToLower(localeParam))
	if locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is required"})
		return
	}

	ctx := c.Request.Context()
	app, err := h.apps.GetByID(ctx, database.SQLX, appID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	found := false
	for _, l := range app.EnabledLanguages {
		if strings.EqualFold(l, locale) {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Locale is not enabled for this application"})
		return
	}

	components, _, err := h.components.List(ctx, database.SQLX, component.ListFilter{ApplicationID: appID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, comp := range components {
		if strings.EqualFold(comp.DefaultLocale, locale) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Cannot delete locale %s: it is the default locale of component %s", locale, comp.Code),
			})
			return
		}
	}

	componentIDs := make([]uuid.UUID, 0, len(components))
	for _, comp := range components {
		componentIDs = append(componentIDs, comp.ID)
	}

	// Cascade-delete across translation_versions + application_locale_deploys
	// + applications.enabled_languages, all sqlx-backed now, in one tx.
	userIDForUpdate, _ := h.getCurrentUser(c)
	newLangs := make([]string, 0, len(app.EnabledLanguages))
	for _, l := range app.EnabledLanguages {
		if !strings.EqualFold(l, locale) {
			newLangs = append(newLangs, l)
		}
	}
	translationsRepo := translation.New()
	if err := repository.WithTx(ctx, database.SQLX, func(tx repository.Queryer) error {
		for _, compID := range componentIDs {
			if err := translationsRepo.DeleteByComponentLocale(ctx, tx, compID, locale); err != nil {
				return err
			}
		}
		if err := h.deploys.Delete(ctx, tx, appID, locale); err != nil {
			return err
		}
		return h.apps.UpdateEnabledLanguages(ctx, tx, appID, newLangs, userIDForUpdate)
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete language: " + err.Error()})
		return
	}

	for _, compID := range componentIDs {
		for _, stage := range []string{"draft", "staging", "production"} {
			cache.Delete(cache.TranslationKey(compID.String(), locale, stage))
		}
		cache.Delete(cache.ComponentKey(compID.String()))
	}
	services.InvalidateApplicationReadCache(appID)

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogDelete(
		userID,
		username,
		"application_language",
		appID,
		locale,
		map[string]interface{}{"locale": locale},
		ipAddress,
		userAgent,
	)

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Language %s removed", locale), "locale": locale})
}
