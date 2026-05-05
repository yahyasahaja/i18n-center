package handlers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

type CmsTemplateHandler struct {
	auditService services.AuditServicer
}

func NewCmsTemplateHandler() *CmsTemplateHandler {
	return &CmsTemplateHandler{auditService: services.NewAuditService()}
}

func (h *CmsTemplateHandler) currentUser(c *gin.Context) (uuid.UUID, string) {
	var userID uuid.UUID
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			userID, _ = uuid.Parse(s)
		}
	}
	username, _ := c.Get("username")
	name, _ := username.(string)
	return userID, name
}

// ListTemplates lists all CMS templates for an application.
func (h *CmsTemplateHandler) ListTemplates(c *gin.Context) {
	appID := c.Param("id")
	var templates []models.CmsTemplate
	if err := database.DB.Preload("Fields").Where("application_id = ?", appID).Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range templates {
		sort.Slice(templates[i].Fields, func(a, b int) bool {
			return templates[i].Fields[a].SortOrder < templates[i].Fields[b].SortOrder
		})
	}
	c.JSON(http.StatusOK, templates)
}

// GetTemplate returns a single CMS template by ID.
func (h *CmsTemplateHandler) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	var tmpl models.CmsTemplate
	if err := database.DB.Preload("Fields").First(&tmpl, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	sort.Slice(tmpl.Fields, func(a, b int) bool {
		return tmpl.Fields[a].SortOrder < tmpl.Fields[b].SortOrder
	})
	c.JSON(http.StatusOK, tmpl)
}

type cmsTemplateFieldInput struct {
	Key       string `json:"key" binding:"required"`
	Label     string `json:"label" binding:"required"`
	ValueType string `json:"value_type" binding:"required"`
	Required  bool   `json:"required"`
	SortOrder int    `json:"sort_order"`
}

type createCmsTemplateBody struct {
	Name        string                  `json:"name" binding:"required"`
	Code        string                  `json:"code" binding:"required"`
	Description string                  `json:"description"`
	Fields      []cmsTemplateFieldInput `json:"fields"`
}

// CreateTemplate creates a new CMS template with its fields.
func (h *CmsTemplateHandler) CreateTemplate(c *gin.Context) {
	appID := c.Param("id")
	appUUID, err := uuid.Parse(appID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	var body createCmsTemplateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.currentUser(c)

	tmpl := models.CmsTemplate{
		ApplicationID: appUUID,
		Name:          strings.TrimSpace(body.Name),
		Code:          strings.TrimSpace(body.Code),
		Description:   strings.TrimSpace(body.Description),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	tx := database.DB.Begin()
	if err := tx.Create(&tmpl).Error; err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Template code already exists for this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, f := range body.Fields {
		if !isValidCmsValueType(f.ValueType) {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value_type: " + f.ValueType})
			return
		}
		field := models.CmsTemplateField{
			TemplateID: tmpl.ID,
			Key:        strings.TrimSpace(f.Key),
			Label:      strings.TrimSpace(f.Label),
			ValueType:  f.ValueType,
			Required:   f.Required,
			SortOrder:  f.SortOrder,
		}
		if err := tx.Create(&field).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	database.DB.Preload("Fields").First(&tmpl, tmpl.ID)
	sort.Slice(tmpl.Fields, func(a, b int) bool {
		return tmpl.Fields[a].SortOrder < tmpl.Fields[b].SortOrder
	})

	h.auditService.LogCreate(userID, username, "cms_template", tmpl.ID, tmpl.Code, tmpl, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusCreated, tmpl)
}

type updateCmsTemplateBody struct {
	Name        *string                  `json:"name"`
	Description *string                  `json:"description"`
	Fields      []cmsTemplateFieldInput  `json:"fields"`
}

// UpdateTemplate replaces a CMS template's metadata and fields.
func (h *CmsTemplateHandler) UpdateTemplate(c *gin.Context) {
	id := c.Param("id")
	var tmpl models.CmsTemplate
	if err := database.DB.First(&tmpl, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	var body updateCmsTemplateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.currentUser(c)

	if body.Name != nil {
		tmpl.Name = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		tmpl.Description = strings.TrimSpace(*body.Description)
	}
	tmpl.UpdatedBy = userID

	tx := database.DB.Begin()
	if err := tx.Save(&tmpl).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if body.Fields != nil {
		// Replace all fields
		if err := tx.Where("template_id = ?", tmpl.ID).Delete(&models.CmsTemplateField{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, f := range body.Fields {
			if !isValidCmsValueType(f.ValueType) {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value_type: " + f.ValueType})
				return
			}
			field := models.CmsTemplateField{
				TemplateID: tmpl.ID,
				Key:        strings.TrimSpace(f.Key),
				Label:      strings.TrimSpace(f.Label),
				ValueType:  f.ValueType,
				Required:   f.Required,
				SortOrder:  f.SortOrder,
			}
			if err := tx.Create(&field).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	database.DB.Preload("Fields").First(&tmpl, tmpl.ID)
	sort.Slice(tmpl.Fields, func(a, b int) bool {
		return tmpl.Fields[a].SortOrder < tmpl.Fields[b].SortOrder
	})

	h.auditService.LogUpdate(userID, username, "cms_template", tmpl.ID, tmpl.Code, nil, tmpl, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, tmpl)
}

// DeleteTemplate deletes a CMS template and its fields.
func (h *CmsTemplateHandler) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	var tmpl models.CmsTemplate
	if err := database.DB.First(&tmpl, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	userID, username := h.currentUser(c)

	// Check no CmsItems reference this template
	var count int64
	database.DB.Model(&models.CmsItem{}).Where("template_id = ?", tmpl.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete template: it is used by existing CMS items"})
		return
	}

	if err := database.DB.Delete(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.auditService.LogDelete(userID, username, "cms_template", tmpl.ID, tmpl.Code, tmpl, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "Template deleted"})
}

func isValidCmsValueType(vt string) bool {
	switch vt {
	case models.CmsValueTypeText, models.CmsValueTypeTextarea, models.CmsValueTypeRichText, models.CmsValueTypeJSON:
		return true
	}
	return false
}
