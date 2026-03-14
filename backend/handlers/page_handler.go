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

type PageHandler struct {
	auditService *services.AuditService
}

func NewPageHandler() *PageHandler {
	return &PageHandler{auditService: services.NewAuditService()}
}

func (h *PageHandler) getCurrentUser(c *gin.Context) (userID uuid.UUID, username string) {
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

func (h *PageHandler) getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	return c.ClientIP(), c.GetHeader("User-Agent")
}

// ListByApplication returns all pages for an application
func (h *PageHandler) ListByApplication(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	var pages []models.Page
	if err := database.DB.Where("application_id = ?", appID).Find(&pages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pages)
}

// Create creates a page for an application
func (h *PageHandler) Create(c *gin.Context) {
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
	var existing models.Page
	if err := database.DB.Where("application_id = ? AND code = ?", appID, code).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page code already exists for this application"})
		return
	}
	page := models.Page{ApplicationID: appID, Code: code}
	if err := database.DB.Create(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogCreate(userID, username, "page", page.ID, page.Code, page, ipAddress, userAgent)
	c.JSON(http.StatusCreated, page)
}

// Get returns a single page by ID
func (h *PageHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var page models.Page
	if err := database.DB.First(&page, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	c.JSON(http.StatusOK, page)
}

// Update updates a page
func (h *PageHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var page models.Page
	if err := database.DB.First(&page, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before := models.Page{Code: page.Code}
	if body.Code != "" {
		page.Code = strings.TrimSpace(strings.ToLower(body.Code))
	}
	var duplicate models.Page
	if err := database.DB.Where("application_id = ? AND code = ? AND id != ?", page.ApplicationID, page.Code, page.ID).First(&duplicate).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page code already exists for this application"})
		return
	}
	if err := database.DB.Save(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogUpdate(userID, username, "page", page.ID, page.Code, before, page, ipAddress, userAgent)
	c.JSON(http.StatusOK, page)
}

// Delete deletes a page
func (h *PageHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var page models.Page
	if err := database.DB.First(&page, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	if err := database.DB.Delete(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogDelete(userID, username, "page", page.ID, page.Code, page, ipAddress, userAgent)
	c.JSON(http.StatusOK, gin.H{"message": "Page deleted"})
}

// GetComponents returns components that have this page
func (h *PageHandler) GetComponents(c *gin.Context) {
	id := c.Param("id")
	var page models.Page
	if err := database.DB.First(&page, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	var components []models.Component
	if err := database.DB.Joins("JOIN component_pages ON component_pages.component_id = components.id AND component_pages.page_id = ?", page.ID).Where("components.deleted_at IS NULL").Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, components)
}
