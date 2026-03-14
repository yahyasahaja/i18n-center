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

// replaceComponentTagsAndPages sets the component's tags and pages by ID (only those belonging to the same application).
func replaceComponentTagsAndPages(component *models.Component, tagIDs, pageIDs []string) error {
	if tagIDs != nil {
		var tags []models.Tag
		for _, idStr := range tagIDs {
			id, err := uuid.Parse(strings.TrimSpace(idStr))
			if err != nil {
				continue
			}
			var t models.Tag
			if err := database.DB.Where("id = ? AND application_id = ?", id, component.ApplicationID).First(&t).Error; err != nil {
				continue
			}
			tags = append(tags, t)
		}
		if err := database.DB.Model(component).Association("Tags").Replace(tags); err != nil {
			return err
		}
	}
	if pageIDs != nil {
		var pages []models.Page
		for _, idStr := range pageIDs {
			id, err := uuid.Parse(strings.TrimSpace(idStr))
			if err != nil {
				continue
			}
			var p models.Page
			if err := database.DB.Where("id = ? AND application_id = ?", id, component.ApplicationID).First(&p).Error; err != nil {
				continue
			}
			pages = append(pages, p)
		}
		if err := database.DB.Model(component).Association("Pages").Replace(pages); err != nil {
			return err
		}
	}
	return nil
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
	query := database.DB.Preload("Tags").Preload("Pages")
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
	if err := database.DB.Preload("Application").Preload("Tags").Preload("Pages").First(&component, "id = ?", identifier).Error; err != nil {
		if err := database.DB.Preload("Application").Preload("Tags").Preload("Pages").First(&component, "code = ?", identifier).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
			return
		}
	}

	// Cache for 1 hour
	cache.Set(cacheKey, component, 3600*1000000000)
	c.JSON(http.StatusOK, component)
}

// createComponentBody is the request body for creating a component (includes tag_ids and page_ids).
type createComponentBody struct {
	Name          string         `json:"name" binding:"required"`
	Code          string         `json:"code" binding:"required"`
	ApplicationID uuid.UUID      `json:"application_id" binding:"required"`
	Description   string         `json:"description"`
	DefaultLocale string         `json:"default_locale" binding:"required"`
	Structure     models.JSONB   `json:"structure"`
	TagIDs        []string       `json:"tag_ids"`
	PageIDs       []string       `json:"page_ids"`
}

// CreateComponent creates a new component
func (h *ComponentHandler) CreateComponent(c *gin.Context) {
	var body createComponentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	component := models.Component{
		Name:          strings.TrimSpace(body.Name),
		Code:          strings.TrimSpace(body.Code),
		ApplicationID: body.ApplicationID,
		Description:   strings.TrimSpace(body.Description),
		DefaultLocale: strings.TrimSpace(body.DefaultLocale),
		Structure:     body.Structure,
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	if err := database.DB.Create(&component).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Component code already exists for this application"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	if err := replaceComponentTagsAndPages(&component, body.TagIDs, body.PageIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Preload("Tags").Preload("Pages").First(&component, component.ID)

	h.auditService.LogCreate(userID, username, "component", component.ID, component.Code, component, ipAddress, userAgent)
	c.JSON(http.StatusCreated, component)
}

// updateComponentBody is the request body for updating a component (includes optional tag_ids and page_ids).
type updateComponentBody struct {
	Name          *string       `json:"name"`
	Code          *string       `json:"code"`
	Description   *string       `json:"description"`
	DefaultLocale *string       `json:"default_locale"`
	Structure     *models.JSONB `json:"structure"`
	TagIDs        []string      `json:"tag_ids"`
	PageIDs       []string      `json:"page_ids"`
}

// UpdateComponent updates a component
func (h *ComponentHandler) UpdateComponent(c *gin.Context) {
	id := c.Param("id")
	var component models.Component

	if err := database.DB.Preload("Tags").Preload("Pages").First(&component, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	before := models.Component{
		Name:          component.Name,
		Code:          component.Code,
		Description:   component.Description,
		Structure:     component.Structure,
		DefaultLocale: component.DefaultLocale,
	}

	var body updateComponentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.Name != nil {
		component.Name = strings.TrimSpace(*body.Name)
	}
	if body.Code != nil {
		component.Code = strings.TrimSpace(*body.Code)
	}
	if body.Description != nil {
		component.Description = strings.TrimSpace(*body.Description)
	}
	if body.DefaultLocale != nil {
		component.DefaultLocale = strings.TrimSpace(*body.DefaultLocale)
	}
	if body.Structure != nil {
		component.Structure = *body.Structure
	}
	component.UpdatedBy = userID

	if err := database.DB.Save(&component).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := replaceComponentTagsAndPages(&component, body.TagIDs, body.PageIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Preload("Tags").Preload("Pages").First(&component, component.ID)

	after := models.Component{
		Name:          component.Name,
		Code:          component.Code,
		Description:   component.Description,
		Structure:     component.Structure,
		DefaultLocale: component.DefaultLocale,
	}
	h.auditService.LogUpdate(userID, username, "component", component.ID, component.Code, before, after, ipAddress, userAgent)
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

