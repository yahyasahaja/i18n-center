package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/jobs"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
	"gorm.io/gorm"
)

type ApplicationHandler struct {
	auditService *services.AuditService
}

func NewApplicationHandler() *ApplicationHandler {
	return &ApplicationHandler{
		auditService: services.NewAuditService(),
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
	var applications []models.Application
	if err := database.DB.Find(&applications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Set HasOpenAIKey for each application
	for i := range applications {
		applications[i].HasOpenAIKey = applications[i].OpenAIKey != ""
	}

	c.JSON(http.StatusOK, applications)
}

// GetApplication gets a single application (by ID or code)
func (h *ApplicationHandler) GetApplication(c *gin.Context) {
	identifier := c.Param("id")

	// Try cache (by ID)
	cacheKey := cache.ApplicationKey(identifier)
	var cached models.Application
	if err := cache.Get(cacheKey, &cached); err == nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	var application models.Application
	// Try by ID first, then by code
	if err := database.DB.First(&application, "id = ?", identifier).Error; err != nil {
		if err := database.DB.First(&application, "code = ?", identifier).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
			return
		}
	}

	// Set HasOpenAIKey
	application.HasOpenAIKey = application.OpenAIKey != ""

	// Cache for 1 hour
	cache.Set(cacheKey, application, 3600*1000000000)
	c.JSON(http.StatusOK, application)
}

// ApplicationRequest represents the request payload for creating/updating applications
type ApplicationRequest struct {
	Name             string   `json:"name" binding:"required"`
	Code             string   `json:"code" binding:"required"` // Unique identifier
	Description      string   `json:"description"`
	EnabledLanguages []string `json:"enabled_languages"`
	OpenAIKey        string   `json:"openai_key"` // Accept from frontend
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

	application := models.Application{
		Name:             req.Name,
		Code:             req.Code,
		Description:      req.Description,
		EnabledLanguages: models.StringArray(req.EnabledLanguages),
		OpenAIKey:        req.OpenAIKey,
		CreatedBy:        userID,
		UpdatedBy:        userID,
	}

	if err := database.DB.Create(&application).Error; err != nil {
		// Check if it's a unique constraint violation
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Application code already exists"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// Log audit
	h.auditService.LogCreate(
		userID,
		username,
		"application",
		application.ID,
		application.Code,
		application,
		ipAddress,
		userAgent,
	)

	// Set HasOpenAIKey
	application.HasOpenAIKey = application.OpenAIKey != ""
	c.JSON(http.StatusCreated, application)
}

// UpdateApplication updates an application
func (h *ApplicationHandler) UpdateApplication(c *gin.Context) {
	id := c.Param("id")
	var application models.Application

	if err := database.DB.First(&application, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	var req ApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Store before values for audit
	before := models.Application{
		Name:             application.Name,
		Code:             application.Code,
		Description:      application.Description,
		EnabledLanguages: application.EnabledLanguages,
		// Don't log OpenAIKey for security
	}

	// Update fields
	application.Name = req.Name
	application.Code = req.Code
	application.Description = req.Description
	application.EnabledLanguages = models.StringArray(req.EnabledLanguages)
	application.UpdatedBy = userID
	// Only update OpenAIKey if provided (not empty)
	if req.OpenAIKey != "" {
		application.OpenAIKey = req.OpenAIKey
	}

	if err := database.DB.Save(&application).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store after values for audit
	after := models.Application{
		Name:             application.Name,
		Code:             application.Code,
		Description:      application.Description,
		EnabledLanguages: application.EnabledLanguages,
	}

	// Log audit
	h.auditService.LogUpdate(
		userID,
		username,
		"application",
		application.ID,
		application.Code,
		before,
		after,
		ipAddress,
		userAgent,
	)

	// Set HasOpenAIKey
	application.HasOpenAIKey = application.OpenAIKey != ""

	// Invalidate cache
	cache.Delete(cache.ApplicationKey(id))
	c.JSON(http.StatusOK, application)
}

// DeleteApplication deletes an application
func (h *ApplicationHandler) DeleteApplication(c *gin.Context) {
	id := c.Param("id")

	var application models.Application
	if err := database.DB.First(&application, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	if err := database.DB.Delete(&application).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log audit
	h.auditService.LogDelete(
		userID,
		username,
		"application",
		application.ID,
		application.Code,
		application,
		ipAddress,
		userAgent,
	)

	// Invalidate cache
	cache.Delete(cache.ApplicationKey(id))
	c.JSON(http.StatusOK, gin.H{"message": "Application deleted"})
}

// AddLanguageRequest is the body for adding a new language to an application
type AddLanguageRequest struct {
	Locale       string `json:"locale" binding:"required"`
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

	var application models.Application
	if err := database.DB.First(&application, "id = ?", appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	// Check locale not already enabled
	for _, l := range application.EnabledLanguages {
		if strings.EqualFold(l, req.Locale) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is already enabled"})
			return
		}
	}

	if req.AutoTranslate {
		// Validate OpenAI key exists before creating job
		openAIService := services.NewOpenAIService(application.OpenAIKey)
		if application.OpenAIKey == "" {
			openAIService = services.NewOpenAIService(services.GetDefaultOpenAIKey())
		}
		if openAIService.APIKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Auto-translate requires an OpenAI API key. Configure it in Application settings."})
			return
		}

		// Add locale first so it exists even if job fails later
		application.EnabledLanguages = append(application.EnabledLanguages, req.Locale)
		if err := database.DB.Save(&application).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update application: " + err.Error()})
			return
		}

		// Create job; worker will do the translate work (no in-memory state, K8s-safe)
		job := models.AddLanguageJob{
			ApplicationID:  appID,
			Locale:        req.Locale,
			AutoTranslate: true,
			Status:        models.JobStatusPending,
			CreatedBy:    userID,
		}
		if err := database.DB.Create(&job).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job: " + err.Error()})
			return
		}

		cache.Delete(cache.ApplicationKey(appIDStr))
		c.Header("Location", fmt.Sprintf("/api/applications/%s/jobs/%s", appIDStr, job.ID.String()))
		c.JSON(http.StatusAccepted, gin.H{
			"message":   "Language added. Translation job queued.",
			"job_id":    job.ID.String(),
			"locale":    req.Locale,
			"status":    models.JobStatusPending,
			"status_url": fmt.Sprintf("/api/applications/%s/jobs/%s", appIDStr, job.ID.String()),
		})
		return
	}

	// Sync path: add locale only, no translate
	application.EnabledLanguages = append(application.EnabledLanguages, req.Locale)
	if err := database.DB.Save(&application).Error; err != nil {
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

	var deploys []models.ApplicationLocaleDeploy
	if err := database.DB.Where("application_id = ? AND stage_completed != ?", appID, "production").Find(&deploys).Error; err != nil {
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
			"locale":        d.Locale,
			"stage_completed": d.StageCompleted,
			"next_stage":    nextStage,
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

	var deploy models.ApplicationLocaleDeploy
	if err := database.DB.Where("application_id = ? AND locale = ?", appID, req.Locale).First(&deploy).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No pending deploy found for this locale"})
		return
	}

	var fromStage, toStage models.DeploymentStage
	switch deploy.StageCompleted {
	case "draft":
		fromStage = models.StageDraft
		toStage = models.StageStaging
	case "staging":
		fromStage = models.StageStaging
		toStage = models.StageProduction
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is already fully deployed to production"})
		return
	}

	var components []models.Component
	if err := database.DB.Where("application_id = ?", appID).Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	translationService := services.NewTranslationService()
	for _, comp := range components {
		if err := translationService.DeployToStage(comp.ID, req.Locale, fromStage, toStage, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Deploy failed (no changes applied)",
				"detail":  fmt.Sprintf("Component %s: %v", comp.Code, err),
				"retry":   true,
			})
			return
		}
	}

	deploy.StageCompleted = string(toStage)
	if err := database.DB.Save(&deploy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update deploy state: " + err.Error()})
		return
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

	job, err := jobs.GetJobStatus(appID, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	resp := gin.H{
		"job_id": job.ID.String(),
		"locale": job.Locale,
		"status": job.Status,
	}
	if job.Status == models.JobStatusFailed {
		resp["error_message"] = job.ErrorMessage
		resp["error_detail"] = job.ErrorDetail
		resp["retry"] = true
	}
	c.JSON(http.StatusOK, resp)
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

	var application models.Application
	if err := database.DB.First(&application, "id = ?", appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	found := false
	for _, l := range application.EnabledLanguages {
		if strings.EqualFold(l, locale) {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Locale is not enabled for this application"})
		return
	}

	var components []models.Component
	if err := database.DB.Where("application_id = ?", appID).Find(&components).Error; err != nil {
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

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if len(componentIDs) > 0 {
			if err := tx.Where("component_id IN ? AND locale = ?", componentIDs, locale).Delete(&models.TranslationVersion{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("application_id = ? AND locale = ?", appID, locale).Delete(&models.ApplicationLocaleDeploy{}).Error; err != nil {
			return err
		}
		newLangs := make([]string, 0, len(application.EnabledLanguages))
		for _, l := range application.EnabledLanguages {
			if !strings.EqualFold(l, locale) {
				newLangs = append(newLangs, l)
			}
		}
		application.EnabledLanguages = newLangs
		return tx.Save(&application).Error
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
	cache.Delete(cache.ApplicationKey(appIDStr))

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

