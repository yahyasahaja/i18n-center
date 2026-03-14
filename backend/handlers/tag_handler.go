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

type TagHandler struct {
	auditService *services.AuditService
}

func NewTagHandler() *TagHandler {
	return &TagHandler{auditService: services.NewAuditService()}
}

func (h *TagHandler) getCurrentUser(c *gin.Context) (userID uuid.UUID, username string) {
	userIDVal, _ := c.Get("user_id")
	if idStr, ok := userIDVal.(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			userID = id
		}
	}
	usernameVal, _ := c.Get("username")
	if name, ok := usernameVal.(string); ok {
		username = name
	}
	return userID, username
}

func (h *TagHandler) getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	return c.ClientIP(), c.GetHeader("User-Agent")
}

// ListByApplication returns all tags for an application
func (h *TagHandler) ListByApplication(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	var tags []models.Tag
	if err := database.DB.Where("application_id = ?", appID).Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// Create creates a tag for an application
func (h *TagHandler) Create(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	var body struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	code := strings.TrimSpace(strings.ToLower(body.Code))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}
	var existing models.Tag
	if err := database.DB.Where("application_id = ? AND code = ?", appID, code).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tag code already exists for this application"})
		return
	}
	tag := models.Tag{ApplicationID: appID, Code: code}
	if err := database.DB.Create(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogCreate(userID, username, "tag", tag.ID, tag.Code, tag, ipAddress, userAgent)
	c.JSON(http.StatusCreated, tag)
}

// Get returns a single tag by ID
func (h *TagHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var tag models.Tag
	if err := database.DB.First(&tag, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}
	c.JSON(http.StatusOK, tag)
}

// Update updates a tag
func (h *TagHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var tag models.Tag
	if err := database.DB.First(&tag, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before := models.Tag{Code: tag.Code}
	if body.Code != "" {
		tag.Code = strings.TrimSpace(strings.ToLower(body.Code))
	}
	var duplicate models.Tag
	if err := database.DB.Where("application_id = ? AND code = ? AND id != ?", tag.ApplicationID, tag.Code, tag.ID).First(&duplicate).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tag code already exists for this application"})
		return
	}
	if err := database.DB.Save(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogUpdate(userID, username, "tag", tag.ID, tag.Code, before, tag, ipAddress, userAgent)
	c.JSON(http.StatusOK, tag)
}

// Delete deletes a tag (removes from components via many2many)
func (h *TagHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var tag models.Tag
	if err := database.DB.First(&tag, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}
	if err := database.DB.Delete(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogDelete(userID, username, "tag", tag.ID, tag.Code, tag, ipAddress, userAgent)
	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted"})
}

// GetComponents returns components that have this tag
func (h *TagHandler) GetComponents(c *gin.Context) {
	id := c.Param("id")
	var tag models.Tag
	if err := database.DB.First(&tag, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}
	var components []models.Component
	if err := database.DB.Joins("JOIN component_tags ON component_tags.component_id = components.id AND component_tags.tag_id = ?", tag.ID).Where("components.deleted_at IS NULL").Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, components)
}
