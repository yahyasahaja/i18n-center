package handlers

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/database"
	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/cms"
	"github.com/lapakgaming/i18n-center/services"
)

type CmsTemplateHandler struct {
	auditService services.AuditServicer
	templates    cms.TemplateRepository
}

func NewCmsTemplateHandler() *CmsTemplateHandler {
	return &CmsTemplateHandler{
		auditService: services.NewAuditService(),
		templates:    cms.NewTemplateRepository(),
	}
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
// @Summary      List templates
// @Description  Get all CMS templates for an application
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Application ID (UUID)"
// @Success      200  {array}   cms.Template
// @Failure      401  {object}  map[string]string
// @Router       /applications/{id}/cms/templates [get]
func (h *CmsTemplateHandler) ListTemplates(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	ctx := c.Request.Context()
	templates, err := h.templates.ListByApp(ctx, database.SQLX, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Populate Fields per template for the response. Page size is bounded
	// (templates per app << 100 in practice); N+1 is fine.
	for i := range templates {
		fields, err := h.templates.LoadFields(ctx, database.SQLX, templates[i].ID)
		if err == nil {
			sortFields(fields)
			templates[i].Fields = fields
		}
	}
	c.JSON(http.StatusOK, templates)
}

// GetTemplate returns a single CMS template by ID, including its fields.
// @Summary      Get template
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Template ID (UUID)"
// @Success      200  {object}  cms.Template
// @Failure      404  {object}  map[string]string
// @Router       /cms/templates/{id} [get]
func (h *CmsTemplateHandler) GetTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}
	t, err := h.templates.GetByIDWithFields(c.Request.Context(), database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sortFields(t.Fields)
	c.JSON(http.StatusOK, t)
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

// CreateTemplate creates a new CMS template with its fields, atomically.
// @Summary      Create template
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                 true  "Application ID (UUID)"
// @Param        body  body      createCmsTemplateBody  true  "Template data"
// @Success      201  {object}  cms.Template
// @Failure      400  {object}  map[string]string
// @Router       /applications/{id}/cms/templates [post]
func (h *CmsTemplateHandler) CreateTemplate(c *gin.Context) {
	appUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	var body createCmsTemplateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate value_types up front so we fail before opening a transaction.
	for _, f := range body.Fields {
		if !cms.IsValidValueType(f.ValueType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value_type: " + f.ValueType})
			return
		}
	}

	userID, username := h.currentUser(c)

	tmpl := cms.Template{
		ApplicationID: appUUID,
		Name:          strings.TrimSpace(body.Name),
		Code:          strings.TrimSpace(body.Code),
		Description:   strings.TrimSpace(body.Description),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	ctx := c.Request.Context()
	if err := repository.WithTx(ctx, database.SQLX, func(tx repository.Queryer) error {
		if err := h.templates.Create(ctx, tx, &tmpl); err != nil {
			return err
		}
		fields := mapInputToFields(body.Fields, tmpl.ID)
		return h.templates.ReplaceFields(ctx, tx, tmpl.ID, fields)
	}); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Template code already exists for this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload with fields populated for the response.
	if reloaded, err := h.templates.GetByIDWithFields(ctx, database.SQLX, tmpl.ID); err == nil {
		sortFields(reloaded.Fields)
		tmpl = *reloaded
	}

	h.auditService.LogCreate(userID, username, "cms_template", tmpl.ID, tmpl.Code, tmpl, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusCreated, tmpl)
}

type updateCmsTemplateBody struct {
	Name        *string                 `json:"name"`
	Description *string                 `json:"description"`
	Fields      []cmsTemplateFieldInput `json:"fields"`
}

// UpdateTemplate replaces a CMS template's metadata and (optionally) its full
// field list. Fields, when provided, REPLACE the current set — clients that
// want to add a single field must send the full updated list.
//
// @Summary      Update template
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                 true  "Template ID (UUID)"
// @Param        body  body      updateCmsTemplateBody  true  "Template update data"
// @Success      200  {object}  cms.Template
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/templates/{id} [put]
func (h *CmsTemplateHandler) UpdateTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	ctx := c.Request.Context()
	tmpl, err := h.templates.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	// Validate value_types up front (before opening the tx).
	for _, f := range body.Fields {
		if !cms.IsValidValueType(f.ValueType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value_type: " + f.ValueType})
			return
		}
	}

	if err := repository.WithTx(ctx, database.SQLX, func(tx repository.Queryer) error {
		if err := h.templates.Update(ctx, tx, tmpl); err != nil {
			return err
		}
		if body.Fields != nil {
			fields := mapInputToFields(body.Fields, tmpl.ID)
			return h.templates.ReplaceFields(ctx, tx, tmpl.ID, fields)
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if reloaded, err := h.templates.GetByIDWithFields(ctx, database.SQLX, tmpl.ID); err == nil {
		sortFields(reloaded.Fields)
		tmpl = reloaded
	}

	h.auditService.LogUpdate(userID, username, "cms_template", tmpl.ID, tmpl.Code, nil, tmpl, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, tmpl)
}

// DeleteTemplate refuses to delete if items still reference it.
// @Summary      Delete template
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Template ID (UUID)"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/templates/{id} [delete]
func (h *CmsTemplateHandler) DeleteTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	ctx := c.Request.Context()
	tmpl, err := h.templates.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	count, err := h.templates.CountItemsForTemplate(ctx, database.SQLX, tmpl.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete template: it is used by existing CMS items"})
		return
	}

	if err := h.templates.SoftDelete(ctx, database.SQLX, tmpl.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.currentUser(c)
	h.auditService.LogDelete(userID, username, "cms_template", tmpl.ID, tmpl.Code, tmpl, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "Template deleted"})
}

// mapInputToFields converts the user-facing field-input slice into the
// repository's TemplateField shape, trimming string values along the way.
// TemplateID is set by ReplaceFields.
func mapInputToFields(input []cmsTemplateFieldInput, templateID uuid.UUID) []cms.TemplateField {
	out := make([]cms.TemplateField, len(input))
	for i, f := range input {
		out[i] = cms.TemplateField{
			TemplateID: templateID,
			Key:        strings.TrimSpace(f.Key),
			Label:      strings.TrimSpace(f.Label),
			ValueType:  f.ValueType,
			Required:   f.Required,
			SortOrder:  f.SortOrder,
		}
	}
	return out
}

// sortFields sorts the provided slice in-place by (SortOrder, Key) so the
// response order is stable for a given template configuration.
func sortFields(fields []cms.TemplateField) {
	sort.Slice(fields, func(a, b int) bool {
		if fields[a].SortOrder != fields[b].SortOrder {
			return fields[a].SortOrder < fields[b].SortOrder
		}
		return fields[a].Key < fields[b].Key
	})
}
