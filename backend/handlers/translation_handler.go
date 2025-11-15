package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

type TranslationHandler struct {
	translationService *services.TranslationService
	openAIService      *services.OpenAIService
	auditService       *services.AuditService
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

type SaveTranslationRequest struct {
	Locale string          `json:"locale" binding:"required"`
	Stage  string          `json:"stage" binding:"required"`
	Data   models.JSONB    `json:"data" binding:"required"`
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

	if err := h.translationService.RevertTranslation(componentID, locale, stage); err != nil {
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
				"action": "DEPLOY",
				"component_id": componentID.String(),
				"locale": req.Locale,
				"from_stage": string(fromStage),
				"to_stage": string(toStage),
				"data": sourceTranslation.Data,
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

// AutoTranslate translates a component to target locale using OpenAI
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

	// Get source translation
	stage := models.DeploymentStage(req.Stage)
	sourceTranslation, err := h.translationService.GetTranslation(componentID, req.SourceLocale, stage)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source translation not found"})
		return
	}

	// Get component to find application
	var component models.Component
	if err := database.DB.First(&component, "id = ?", componentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	// Get application for OpenAI key
	var application models.Application
	if err := database.DB.First(&application, "id = ?", component.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	// Initialize OpenAI service
	openAIService := services.NewOpenAIService(application.OpenAIKey)
	if application.OpenAIKey == "" {
		openAIService = services.NewOpenAIService(services.GetDefaultOpenAIKey())
	}

	// Translate JSON structure
	translatedData, err := openAIService.TranslateJSON(sourceTranslation.Data, req.SourceLocale, req.TargetLocale)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Save translated data
	translation, err := h.translationService.SaveTranslation(componentID, req.TargetLocale, stage, translatedData, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log audit
	h.auditService.LogAction(
		userID,
		username,
		"AUTO_TRANSLATE",
		"translation",
		translation.ID,
		component.Code,
		map[string]interface{}{
			"action": "AUTO_TRANSLATE",
			"component_id": componentID.String(),
			"source_locale": req.SourceLocale,
			"target_locale": req.TargetLocale,
			"stage": string(stage),
			"data": translatedData,
		},
		ipAddress,
		userAgent,
	)

	c.JSON(http.StatusOK, translation)
}

type BackfillRequest struct {
	SourceLocale string   `json:"source_locale" binding:"required"`
	TargetLocales []string `json:"target_locales" binding:"required"`
	Stage        string   `json:"stage" binding:"required"`
}

// BackfillTranslations backfills translations for multiple locales
// @Summary      Backfill translations
// @Description  Automatically translate and fill missing locales for a component
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string            true  "Component ID"
// @Param        request  body      BackfillRequest   true  "Backfill request"
// @Success      200      {array}   models.TranslationVersion
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
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

	// Get component and application
	var component models.Component
	if err := database.DB.First(&component, "id = ?", componentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	var application models.Application
	if err := database.DB.First(&application, "id = ?", component.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	// Get source translation
	stage := models.DeploymentStage(req.Stage)
	sourceTranslation, err := h.translationService.GetTranslation(componentID, req.SourceLocale, stage)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source translation not found"})
		return
	}

	// Initialize OpenAI service
	openAIService := services.NewOpenAIService(application.OpenAIKey)
	if application.OpenAIKey == "" {
		openAIService = services.NewOpenAIService(services.GetDefaultOpenAIKey())
	}

	// Translate for each target locale
	results := make([]models.TranslationVersion, 0)
	for _, targetLocale := range req.TargetLocales {
		translatedData, err := openAIService.TranslateJSON(sourceTranslation.Data, req.SourceLocale, targetLocale)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to translate to " + targetLocale + ": " + err.Error()})
			return
		}

		userID, _ := h.getCurrentUser(c)
		translation, err := h.translationService.SaveTranslation(componentID, targetLocale, stage, models.JSONB(translatedData), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save translation for " + targetLocale})
			return
		}

		results = append(results, *translation)
	}

	c.JSON(http.StatusOK, results)
}

// GetVersionComparison gets both versions for comparison
// @Summary      Compare versions
// @Description  Get version 1 (before save) and version 2 (after save) for comparison
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "Component ID"
// @Param        locale  query     string  true   "Locale"
// @Param        stage   query     string  false  "Stage (default: draft)"
// @Success      200     {object}  map[string]interface{}
// @Failure      400     {object}  map[string]string
// @Failure      401     {object}  map[string]string
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

	// Get version 1 (before save)
	var version1 models.TranslationVersion
	result1 := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, 1).First(&version1)

	// Get version 2 (after save)
	var version2 models.TranslationVersion
	result2 := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, 2).First(&version2)

	response := gin.H{
		"version1": nil,
		"version2": nil,
	}

	if result1.Error == nil {
		response["version1"] = gin.H{
			"data":       version1.Data,
			"created_at": version1.CreatedAt,
		}
	}

	if result2.Error == nil {
		response["version2"] = gin.H{
			"data":       version2.Data,
			"created_at": version2.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

