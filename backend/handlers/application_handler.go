package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
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

