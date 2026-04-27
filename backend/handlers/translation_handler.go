package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/middleware"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

type TranslationHandler struct {
	translationService *services.TranslationService
	auditService       services.AuditServicer
}

func NewTranslationHandler() *TranslationHandler {
	return &TranslationHandler{
		translationService: services.NewTranslationService(),
		auditService:       services.NewAuditService(),
	}
}

// getCurrentUser extracts user info from context
func (h *TranslationHandler) getCurrentUser(c *gin.Context) (userID uuid.UUID, username string) {
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

// getClientInfo extracts IP address and user agent
func (h *TranslationHandler) getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.GetHeader("User-Agent")
	return ipAddress, userAgent
}

// GetTranslation retrieves translation for a component
// @Summary      Get translation
// @Description  Get translation data for a component by locale and stage
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string  true   "Component ID"
// @Param        locale   query     string  false  "Locale (default: en)"
// @Param        stage    query     string  false  "Stage (default: production)"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Failure      404      {object}  map[string]string
// @Router       /components/{id}/translations [get]
func (h *TranslationHandler) GetTranslation(c *gin.Context) {
	componentIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		locale = "en" // default
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageProduction // default
	}

	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	translation, err := h.translationService.GetTranslation(componentID, locale, stage)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Translation not found"})
		return
	}

	c.JSON(http.StatusOK, translation)
}

// GetMultipleTranslations retrieves translations for multiple components (aggregator)
// @Summary      Get multiple translations
// @Description  Get translations for multiple components at once. Uses Redis cache efficiently - checks cache first, then database for missing ones. Can use either component_ids or component_codes. When using component_codes, application_code is required to differentiate components with the same code in different applications.
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        component_ids   query     string  false  "Comma-separated component IDs (UUIDs)"
// @Param        application_code query     string  false  "Application code (required when using component_codes)"
// @Param        component_codes query     string  false  "Comma-separated component codes"
// @Param        locale          query     string  false  "Locale (default: en)"
// @Param        stage           query     string  false  "Stage (default: production)"
// @Success      200             {object}  map[string]interface{}  "Map of component_id/code -> translation data"
// @Failure      400             {object}  map[string]string
// @Failure      401             {object}  map[string]string
// @Router       /translations/bulk [get]
func (h *TranslationHandler) GetMultipleTranslations(c *gin.Context) {
	componentIDsStr := c.Query("component_ids")
	componentCodesStr := c.Query("component_codes")
	applicationCode := c.Query("application_code")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	// Must provide either component_ids or component_codes
	if componentIDsStr == "" && componentCodesStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either component_ids or component_codes parameter is required"})
		return
	}

	if locale == "" {
		locale = "en" // default
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageProduction // default
	}

	var translations map[string]*models.TranslationVersion
	var err error

	// Handle component codes
	if componentCodesStr != "" {
		// application_code is required when using component_codes
		if applicationCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "application_code parameter is required when using component_codes"})
			return
		}

		// When authenticated via API key, restrict to that application
		if apiKeyAppID := middleware.GetAPIKeyApplicationID(c); apiKeyAppID != uuid.Nil {
			var app models.Application
			if err := database.DB.Where("code = ?", applicationCode).First(&app).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
				return
			}
			if app.ID != apiKeyAppID {
				c.JSON(http.StatusForbidden, gin.H{"error": "API key does not have access to this application"})
				return
			}
		}

		componentCodeStrings := strings.Split(componentCodesStr, ",")
		componentCodes := make([]string, 0, len(componentCodeStrings))

		for _, codeStr := range componentCodeStrings {
			codeStr = strings.TrimSpace(codeStr)
			if codeStr == "" {
				continue
			}
			componentCodes = append(componentCodes, codeStr)
		}

		if len(componentCodes) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least one valid component code is required"})
			return
		}

		// Get translations by codes (with application filter)
		translations, err = h.translationService.GetMultipleTranslationsByCodes(applicationCode, componentCodes, locale, stage)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Format response: map component_code -> translation data
		response := make(map[string]interface{})
		for code, translation := range translations {
			response[code] = translation.Data
		}
		c.JSON(http.StatusOK, response)
		return
	}

	// When using component_ids, API key auth is not allowed (no application scope)
	if middleware.GetAPIKeyApplicationID(c) != uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "When using API key, application_code and component_codes are required"})
		return
	}

	// Handle component IDs
	componentIDStrings := strings.Split(componentIDsStr, ",")
	componentIDs := make([]uuid.UUID, 0, len(componentIDStrings))

	for _, idStr := range componentIDStrings {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		componentID, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid component ID: %s", idStr)})
			return
		}
		componentIDs = append(componentIDs, componentID)
	}

	if len(componentIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one valid component ID is required"})
		return
	}

	// Get translations using aggregator service
	translations, err = h.translationService.GetMultipleTranslations(componentIDs, locale, stage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Format response: map component_id -> translation data
	response := make(map[string]interface{})
	for componentIDStr, translation := range translations {
		response[componentIDStr] = translation.Data
	}

	c.JSON(http.StatusOK, response)
}

// GetTranslationsByTag returns translations for all components that have the given tag
// @Summary      Get translations by tag
// @Description  Returns translations for all components that have the given tag. Response is a map of component code -> translation data.
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      string  true   "Application ID (UUID)"
// @Param        tagCode   path      string  true   "Tag code (e.g. checkout, pdp)"
// @Param        locale    query     string  false  "Locale (default: en)"
// @Param        stage     query     string  false  "Stage (default: production)"
// @Success      200       {object}  map[string]interface{}  "Map of component_code -> translation data"
// @Failure      400       {object}  map[string]string
// @Failure      401       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Router       /applications/{id}/translations/by-tag/{tagCode} [get]
func (h *TranslationHandler) GetTranslationsByTag(c *gin.Context) {
	applicationIDStr := c.Param("id")
	tagCode := strings.TrimSpace(strings.ToLower(c.Param("tagCode")))
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		locale = "en"
	}
	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageProduction
	}

	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	if tagCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tag code is required"})
		return
	}
	if apiKeyAppID := middleware.GetAPIKeyApplicationID(c); apiKeyAppID != uuid.Nil && apiKeyAppID != applicationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "API key does not have access to this application"})
		return
	}

	cacheKey := cache.TranslationsByTagKey(applicationIDStr, tagCode, locale, string(stage))
	var response map[string]interface{}
	if err := cache.Get(cacheKey, &response); err == nil {
		c.JSON(http.StatusOK, response)
		return
	}

	var tag models.Tag
	if err := database.DB.First(&tag, "application_id = ? AND code = ?", applicationID, tagCode).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	var componentIDs []uuid.UUID
	if err := database.DB.Table("component_tags").Where("tag_id = ?", tag.ID).Pluck("component_id", &componentIDs).Error; err != nil || len(componentIDs) == 0 {
		empty := gin.H{}
		_ = cache.Set(cacheKey, empty, time.Hour)
		c.JSON(http.StatusOK, empty)
		return
	}

	translations, err := h.translationService.GetMultipleTranslations(componentIDs, locale, stage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var components []models.Component
	if err := database.DB.Where("id IN ?", componentIDs).Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	idToCode := make(map[string]string)
	for _, comp := range components {
		idToCode[comp.ID.String()] = comp.Code
	}

	response = make(map[string]interface{})
	for idStr, tv := range translations {
		if code, ok := idToCode[idStr]; ok && tv != nil {
			response[code] = tv.Data
		}
	}
	_ = cache.Set(cacheKey, response, time.Hour)
	c.JSON(http.StatusOK, response)
}

// GetTranslationsByPage returns translations for all components that have the given page
// @Summary      Get translations by page
// @Description  Returns translations for all components that have the given page. Response is a map of component code -> translation data.
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      string  true   "Application ID (UUID)"
// @Param        pageCode  path      string  true   "Page code (e.g. home, cart)"
// @Param        locale    query     string  false  "Locale (default: en)"
// @Param        stage     query     string  false  "Stage (default: production)"
// @Success      200       {object}  map[string]interface{}  "Map of component_code -> translation data"
// @Failure      400       {object}  map[string]string
// @Failure      401       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Router       /applications/{id}/translations/by-page/{pageCode} [get]
func (h *TranslationHandler) GetTranslationsByPage(c *gin.Context) {
	applicationIDStr := c.Param("id")
	pageCode := strings.TrimSpace(strings.ToLower(c.Param("pageCode")))
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		locale = "en"
	}
	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageProduction
	}

	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	if pageCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page code is required"})
		return
	}
	if apiKeyAppID := middleware.GetAPIKeyApplicationID(c); apiKeyAppID != uuid.Nil && apiKeyAppID != applicationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "API key does not have access to this application"})
		return
	}

	cacheKey := cache.TranslationsByPageKey(applicationIDStr, pageCode, locale, string(stage))
	var response map[string]interface{}
	if err := cache.Get(cacheKey, &response); err == nil {
		c.JSON(http.StatusOK, response)
		return
	}

	var page models.Page
	if err := database.DB.First(&page, "application_id = ? AND code = ?", applicationID, pageCode).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}

	var componentIDs []uuid.UUID
	if err := database.DB.Table("component_pages").Where("page_id = ?", page.ID).Pluck("component_id", &componentIDs).Error; err != nil || len(componentIDs) == 0 {
		empty := gin.H{}
		_ = cache.Set(cacheKey, empty, time.Hour)
		c.JSON(http.StatusOK, empty)
		return
	}

	translations, err := h.translationService.GetMultipleTranslations(componentIDs, locale, stage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var components []models.Component
	if err := database.DB.Where("id IN ?", componentIDs).Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	idToCode := make(map[string]string)
	for _, comp := range components {
		idToCode[comp.ID.String()] = comp.Code
	}

	response = make(map[string]interface{})
	for idStr, tv := range translations {
		if code, ok := idToCode[idStr]; ok && tv != nil {
			response[code] = tv.Data
		}
	}
	_ = cache.Set(cacheKey, response, time.Hour)
	c.JSON(http.StatusOK, response)
}

type SaveTranslationRequest struct {
	Locale string       `json:"locale" binding:"required"`
	Stage  string       `json:"stage" binding:"required"`
	Data   models.JSONB `json:"data" binding:"required"`
}

// SaveTranslation saves a translation
// @Summary      Save translation
// @Description  Save translation data for a component
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                  true  "Component ID"
// @Param        request  body      SaveTranslationRequest  true  "Translation data"
// @Success      200      {object}  models.TranslationVersion
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Router       /components/{id}/translations [post]
func (h *TranslationHandler) SaveTranslation(c *gin.Context) {
	componentIDStr := c.Param("id")
	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	var req SaveTranslationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stage := models.DeploymentStage(req.Stage)
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Get component for audit logging
	var component models.Component
	if err := database.DB.First(&component, "id = ?", componentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	// Get existing translation for before/after comparison
	var beforeData models.JSONB
	existingTranslation, _ := h.translationService.GetTranslation(componentID, req.Locale, stage)
	if existingTranslation != nil {
		beforeData = existingTranslation.Data
	}

	translation, err := h.translationService.SaveTranslation(componentID, req.Locale, stage, req.Data, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log audit
	h.auditService.LogUpdate(
		userID,
		username,
		"translation",
		translation.ID,
		component.Code,
		map[string]interface{}{
			"component_id": componentID.String(),
			"locale":       req.Locale,
			"stage":        string(stage),
			"data":         beforeData,
		},
		map[string]interface{}{
			"component_id": componentID.String(),
			"locale":       req.Locale,
			"stage":        string(stage),
			"data":         req.Data,
		},
		ipAddress,
		userAgent,
	)

	c.JSON(http.StatusOK, translation)
}

// RevertTranslation reverts translation to previous version
func (h *TranslationHandler) RevertTranslation(c *gin.Context) {
	componentIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is required"})
		return
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageDraft
	}

	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	userID, _ := h.getCurrentUser(c)
	if err := h.translationService.RevertTranslation(componentID, locale, stage, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Translation reverted"})
}

type DeployRequest struct {
	Locale    string `json:"locale" binding:"required"`
	FromStage string `json:"from_stage" binding:"required"`
	ToStage   string `json:"to_stage" binding:"required"`
}

// DeployTranslation deploys translation from one stage to another
func (h *TranslationHandler) DeployTranslation(c *gin.Context) {
	componentIDStr := c.Param("id")
	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	var req DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fromStage := models.DeploymentStage(req.FromStage)
	toStage := models.DeploymentStage(req.ToStage)

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Get component for audit logging
	var component models.Component
	if err := database.DB.First(&component, "id = ?", componentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	// Get source translation before deploy
	sourceTranslation, _ := h.translationService.GetTranslation(componentID, req.Locale, fromStage)

	if err := h.translationService.DeployToStage(componentID, req.Locale, fromStage, toStage, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get deployed translation
	deployedTranslation, _ := h.translationService.GetTranslation(componentID, req.Locale, toStage)

	// Log audit
	if sourceTranslation != nil && deployedTranslation != nil {
		h.auditService.LogAction(
			userID,
			username,
			"DEPLOY",
			"translation",
			deployedTranslation.ID,
			component.Code,
			map[string]interface{}{
				"action":       "DEPLOY",
				"component_id": componentID.String(),
				"locale":       req.Locale,
				"from_stage":   string(fromStage),
				"to_stage":     string(toStage),
				"data":         sourceTranslation.Data,
			},
			ipAddress,
			userAgent,
		)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Translation deployed"})
}

type AutoTranslateRequest struct {
	SourceLocale string `json:"source_locale" binding:"required"`
	TargetLocale string `json:"target_locale" binding:"required"`
	Stage        string `json:"stage" binding:"required"`
}

// AutoTranslate enqueues an async job to translate a component to a target locale using OpenAI.
// Returns 202 Accepted with a job_id that can be polled via GET /translate-jobs/:job_id.
//
// @Summary      Auto-translate component (async)
// @Description  Enqueue an OpenAI translation job for a single component + target locale. Poll the returned job_id for status.
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string               true  "Component ID"
// @Param        request  body      AutoTranslateRequest true  "Auto-translate request"
// @Success      202      {object}  map[string]string    "job_id"
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Failure      404      {object}  map[string]string
// @Router       /components/{id}/translations/auto-translate [post]
func (h *TranslationHandler) AutoTranslate(c *gin.Context) {
	componentIDStr := c.Param("id")
	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	var req AutoTranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify component + application exist before enqueuing
	var component models.Component
	if err := database.DB.First(&component, "id = ?", componentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	// Verify source translation exists
	stage := models.DeploymentStage(req.Stage)
	if _, err := h.translationService.GetTranslation(componentID, req.SourceLocale, stage); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source translation not found"})
		return
	}

	userID, _ := h.getCurrentUser(c)

	job := models.TranslateJob{
		ApplicationID: component.ApplicationID,
		ComponentID:   componentID,
		JobType:       models.TranslateJobTypeAutoTranslate,
		SourceLocale:  req.SourceLocale,
		TargetLocales: models.StringArray{req.TargetLocale},
		Status:        models.JobStatusPending,
		CreatedBy:     userID,
	}
	if err := database.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue translation job"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  job.ID.String(),
		"status":  job.Status,
		"message": "Translation job enqueued. Poll /translate-jobs/" + job.ID.String() + " for status.",
	})
}

type BackfillRequest struct {
	SourceLocale  string   `json:"source_locale" binding:"required"`
	TargetLocales []string `json:"target_locales" binding:"required"`
	Stage         string   `json:"stage" binding:"required"`
}

// BackfillTranslations enqueues async jobs to translate a component into multiple target locales.
// One TranslateJob is created per target locale. Returns 202 Accepted with all job_ids.
//
// @Summary      Backfill translations (async)
// @Description  Enqueue OpenAI translation jobs for multiple target locales. Poll each job_id for individual status.
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string          true  "Component ID"
// @Param        request  body      BackfillRequest true  "Backfill request"
// @Success      202      {object}  map[string]interface{}  "job_ids array"
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Failure      404      {object}  map[string]string
// @Router       /components/{id}/translations/backfill [post]
func (h *TranslationHandler) BackfillTranslations(c *gin.Context) {
	componentIDStr := c.Param("id")
	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	var req BackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.TargetLocales) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one target_locale is required"})
		return
	}

	// Verify component exists
	var component models.Component
	if err := database.DB.First(&component, "id = ?", componentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	// Verify source translation exists
	stage := models.DeploymentStage(req.Stage)
	if _, err := h.translationService.GetTranslation(componentID, req.SourceLocale, stage); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source translation not found"})
		return
	}

	userID, _ := h.getCurrentUser(c)

	// One job per target locale so each can be tracked and retried independently
	jobIDs := make([]string, 0, len(req.TargetLocales))
	for _, targetLocale := range req.TargetLocales {
		job := models.TranslateJob{
			ApplicationID: component.ApplicationID,
			ComponentID:   componentID,
			JobType:       models.TranslateJobTypeBackfill,
			SourceLocale:  req.SourceLocale,
			TargetLocales: models.StringArray{targetLocale},
			Status:        models.JobStatusPending,
			CreatedBy:     userID,
		}
		if err := database.DB.Create(&job).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue job for locale " + targetLocale})
			return
		}
		jobIDs = append(jobIDs, job.ID.String())
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_ids": jobIDs,
		"count":   len(jobIDs),
		"message": fmt.Sprintf("%d translation jobs enqueued. Poll /translate-jobs/:job_id for each status.", len(jobIDs)),
	})
}

// GetTranslateJobStatus returns the status of a single TranslateJob by ID.
//
// @Summary      Get translate job status
// @Description  Poll for the status of a single async translation job
// @Tags         translations
// @Produce      json
// @Security     BearerAuth
// @Param        job_id  path      string  true  "Job ID"
// @Success      200     {object}  models.TranslateJob
// @Failure      404     {object}  map[string]string
// @Router       /translate-jobs/{job_id} [get]
func (h *TranslationHandler) GetTranslateJobStatus(c *gin.Context) {
	jobIDStr := c.Param("job_id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	var job models.TranslateJob
	if err := database.DB.First(&job, "id = ?", jobID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ListComponentTranslateJobs returns all TranslateJobs for a component, newest first.
// The dashboard calls this on mount (and every few seconds while any job is running)
// to know whether a translation is in-progress, and to surface errors.
//
// @Summary      List translate jobs for a component
// @Description  Returns all async translation jobs for a component (all statuses). Filter by ?status=pending,running to get active jobs only.
// @Tags         translations
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "Component ID"
// @Param        status  query     string  false  "Comma-separated status filter: pending,running,completed,failed"
// @Success      200     {array}   models.TranslateJob
// @Router       /components/{id}/translate-jobs [get]
func (h *TranslationHandler) ListComponentTranslateJobs(c *gin.Context) {
	componentIDStr := c.Param("id")
	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	q := database.DB.Where("component_id = ?", componentID)

	if statusFilter := c.Query("status"); statusFilter != "" {
		statuses := strings.Split(statusFilter, ",")
		q = q.Where("status IN ?", statuses)
	}

	var jobs []models.TranslateJob
	if err := q.Order("created_at DESC").Limit(50).Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

// GetVersionComparison returns two versions for diff. Default: latest (version1) vs previous (version2). Optional query: version_a, version_b.
// @Summary      Compare versions
// @Description  Get two versions for comparison (default: current vs previous for revert diff)
// @Tags         translations
// @Param        id        path      string  true  "Component ID"
// @Param        locale    query     string  true  "Locale"
// @Param        stage     query     string  false "Stage (default: draft)"
// @Param        version_a query     int     false "First version number (default: latest)"
// @Param        version_b query     int     false "Second version number (default: previous)"
// @Router       /components/{id}/translations/compare [get]
func (h *TranslationHandler) GetVersionComparison(c *gin.Context) {
	componentIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is required"})
		return
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageDraft
	}

	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	versionA, _ := c.GetQuery("version_a")
	versionB, _ := c.GetQuery("version_b")

	response := gin.H{"version1": nil, "version2": nil}

	if versionA != "" && versionB != "" {
		var va, vb int
		if _, err := fmt.Sscanf(versionA, "%d", &va); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version_a"})
			return
		}
		if _, err := fmt.Sscanf(versionB, "%d", &vb); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version_b"})
			return
		}
		v1, err1 := h.translationService.GetVersionByNumber(componentID, locale, stage, va)
		v2, err2 := h.translationService.GetVersionByNumber(componentID, locale, stage, vb)
		if err1 != nil || err2 != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
			return
		}
		response["version1"] = gin.H{"version": v1.Version, "data": v1.Data, "created_at": v1.CreatedAt}
		response["version2"] = gin.H{"version": v2.Version, "data": v2.Data, "created_at": v2.CreatedAt}
		c.JSON(http.StatusOK, response)
		return
	}

	// Default: latest (current) and previous for revert-diff
	versions, err := h.translationService.ListVersions(componentID, locale, stage)
	if err != nil || len(versions) == 0 {
		c.JSON(http.StatusOK, response)
		return
	}
	response["version1"] = gin.H{"version": versions[0].Version, "data": versions[0].Data, "created_at": versions[0].CreatedAt}
	if len(versions) > 1 {
		response["version2"] = gin.H{"version": versions[1].Version, "data": versions[1].Data, "created_at": versions[1].CreatedAt}
	}
	c.JSON(http.StatusOK, response)
}

// ListVersions returns all versions for a component/locale/stage (newest first)
// @Summary      List translation versions
// @Tags         translations
// @Param        id      path      string  true  "Component ID"
// @Param        locale  query     string  true  "Locale"
// @Param        stage   query     string  false "Stage (default: draft)"
// @Router       /components/{id}/translations/versions [get]
func (h *TranslationHandler) ListVersions(c *gin.Context) {
	componentIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is required"})
		return
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageDraft
	}

	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	versions, err := h.translationService.ListVersions(componentID, locale, stage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return slim list for UI (version, created_at; full data optional or fetched on demand)
	list := make([]gin.H, 0, len(versions))
	for _, v := range versions {
		list = append(list, gin.H{
			"version":    v.Version,
			"data":       v.Data,
			"created_at": v.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"versions": list})
}
