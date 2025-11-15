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

type ComponentHandler struct {
	auditService *services.AuditService
}

func NewComponentHandler() *ComponentHandler {
	return &ComponentHandler{
		auditService: services.NewAuditService(),
	}
}

// getCurrentUser extracts user info from context
func (h *ComponentHandler) getCurrentUser(c *gin.Context) (userID uuid.UUID, username string) {
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
func (h *ComponentHandler) getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.GetHeader("User-Agent")
	return ipAddress, userAgent
}

// GetComponents lists all components for an application
// @Summary      List components
// @Description  Get all components, optionally filtered by application
// @Tags         components
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        application_id  query     string  false  "Filter by application ID"
// @Success      200            {array}   models.Component
// @Failure      401            {object}  map[string]string
// @Router       /components [get]
func (h *ComponentHandler) GetComponents(c *gin.Context) {
	applicationID := c.Query("application_id")

	var components []models.Component
	query := database.DB
	if applicationID != "" {
		query = query.Where("application_id = ?", applicationID)
	}

	if err := query.Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, components)
}

// GetComponent gets a single component (by ID or code)
func (h *ComponentHandler) GetComponent(c *gin.Context) {
	identifier := c.Param("id")

	// Try cache (by ID)
	cacheKey := cache.ComponentKey(identifier)
	var cached models.Component
	if err := cache.Get(cacheKey, &cached); err == nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	var component models.Component
	// Try by ID first, then by code
	if err := database.DB.Preload("Application").First(&component, "id = ?", identifier).Error; err != nil {
		if err := database.DB.Preload("Application").First(&component, "code = ?", identifier).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
			return
		}
	}

	// Cache for 1 hour
	cache.Set(cacheKey, component, 3600*1000000000)
	c.JSON(http.StatusOK, component)
}

// CreateComponent creates a new component
func (h *ComponentHandler) CreateComponent(c *gin.Context) {
	var component models.Component
	if err := c.ShouldBindJSON(&component); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	component.CreatedBy = userID
	component.UpdatedBy = userID

	if err := database.DB.Create(&component).Error; err != nil {
		// Check if it's a unique constraint violation
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Component code already exists for this application"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// Log audit
	h.auditService.LogCreate(
		userID,
		username,
		"component",
		component.ID,
		component.Code,
		component,
		ipAddress,
		userAgent,
	)

	c.JSON(http.StatusCreated, component)
}

// UpdateComponent updates a component
func (h *ComponentHandler) UpdateComponent(c *gin.Context) {
	id := c.Param("id")
	var component models.Component

	if err := database.DB.First(&component, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Store before values for audit
	before := models.Component{
		Name:          component.Name,
		Code:          component.Code,
		Description:   component.Description,
		Structure:     component.Structure,
		DefaultLocale: component.DefaultLocale,
	}

	if err := c.ShouldBindJSON(&component); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	component.UpdatedBy = userID

	if err := database.DB.Save(&component).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store after values for audit
	after := models.Component{
		Name:          component.Name,
		Code:          component.Code,
		Description:   component.Description,
		Structure:     component.Structure,
		DefaultLocale: component.DefaultLocale,
	}

	// Log audit
	h.auditService.LogUpdate(
		userID,
		username,
		"component",
		component.ID,
		component.Code,
		before,
		after,
		ipAddress,
		userAgent,
	)

	// Invalidate cache
	cache.Delete(cache.ComponentKey(id))
	c.JSON(http.StatusOK, component)
}

// DeleteComponent deletes a component
func (h *ComponentHandler) DeleteComponent(c *gin.Context) {
	id := c.Param("id")

	var component models.Component
	if err := database.DB.First(&component, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	if err := database.DB.Delete(&component).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log audit
	h.auditService.LogDelete(
		userID,
		username,
		"component",
		component.ID,
		component.Code,
		component,
		ipAddress,
		userAgent,
	)

	// Invalidate cache
	cache.Delete(cache.ComponentKey(id))
	c.JSON(http.StatusOK, gin.H{"message": "Component deleted"})
}

