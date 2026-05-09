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

type CmsItemHandler struct {
	auditService services.AuditServicer
}

func NewCmsItemHandler() *CmsItemHandler {
	return &CmsItemHandler{auditService: services.NewAuditService()}
}

func (h *CmsItemHandler) currentUser(c *gin.Context) (uuid.UUID, string) {
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

// ─── CMS Items ───────────────────────────────────────────────────────────────

// ListItems returns all CMS items for an application.
// @Summary      List CMS items
// @Description  Get all CMS items for an application
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Application ID (UUID)"
// @Success      200  {array}   models.CmsItem
// @Failure      401  {object}  map[string]string
// @Router       /applications/{id}/cms/items [get]
func (h *CmsItemHandler) ListItems(c *gin.Context) {
	appID := c.Param("id")
	var items []models.CmsItem
	if err := database.DB.Preload("Template").Where("application_id = ?", appID).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

// GetItem returns a single CMS item with its template and fields.
// @Summary      Get CMS item
// @Description  Get a single CMS item with its template and fields
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "CMS item ID (UUID)"
// @Success      200  {object}  models.CmsItem
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id} [get]
func (h *CmsItemHandler) GetItem(c *gin.Context) {
	id := c.Param("id")
	var item models.CmsItem
	if err := database.DB.Preload("Template.Fields").First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

type createCmsItemBody struct {
	TemplateID  string `json:"template_id" binding:"required"`
	Identifier  string `json:"identifier" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// CreateItem creates a new CMS item in an application.
// @Summary      Create CMS item
// @Description  Create a new CMS item in an application
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string             true  "Application ID (UUID)"
// @Param        body  body      createCmsItemBody  true  "CMS item data"
// @Success      201  {object}  models.CmsItem
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /applications/{id}/cms/items [post]
func (h *CmsItemHandler) CreateItem(c *gin.Context) {
	appID := c.Param("id")
	appUUID, err := uuid.Parse(appID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	var body createCmsItemBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	templateID, err := uuid.Parse(body.TemplateID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template_id"})
		return
	}

	// Verify template belongs to the same application
	var tmpl models.CmsTemplate
	if err := database.DB.First(&tmpl, "id = ? AND application_id = ?", templateID, appUUID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template not found in this application"})
		return
	}

	userID, username := h.currentUser(c)

	item := models.CmsItem{
		ApplicationID: appUUID,
		TemplateID:    templateID,
		Identifier:    strings.TrimSpace(body.Identifier),
		Name:          strings.TrimSpace(body.Name),
		Description:   strings.TrimSpace(body.Description),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	if err := database.DB.Create(&item).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CMS item identifier already exists in this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	database.DB.Preload("Template").First(&item, item.ID)
	h.auditService.LogCreate(userID, username, "cms_item", item.ID, item.Identifier, item, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusCreated, item)
}

type updateCmsItemBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	TemplateID  *string `json:"template_id"`
}

// UpdateItem updates a CMS item's metadata.
// @Summary      Update CMS item
// @Description  Update a CMS item's metadata
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string             true  "CMS item ID (UUID)"
// @Param        body  body      updateCmsItemBody  true  "CMS item update data"
// @Success      200  {object}  models.CmsItem
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id} [put]
func (h *CmsItemHandler) UpdateItem(c *gin.Context) {
	id := c.Param("id")
	var item models.CmsItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
		return
	}

	var body updateCmsItemBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.currentUser(c)

	if body.Name != nil {
		item.Name = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		item.Description = strings.TrimSpace(*body.Description)
	}
	if body.TemplateID != nil {
		tid, err := uuid.Parse(*body.TemplateID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template_id"})
			return
		}
		var tmpl models.CmsTemplate
		if err := database.DB.First(&tmpl, "id = ? AND application_id = ?", tid, item.ApplicationID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Template not found in this application"})
			return
		}
		item.TemplateID = tid
	}
	item.UpdatedBy = userID

	if err := database.DB.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	database.DB.Preload("Template").First(&item, item.ID)
	h.auditService.LogUpdate(userID, username, "cms_item", item.ID, item.Identifier, nil, item, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, item)
}

// DeleteItem deletes a CMS item and all its localizations.
// @Summary      Delete CMS item
// @Description  Delete a CMS item and all its localizations
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "CMS item ID (UUID)"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id} [delete]
func (h *CmsItemHandler) DeleteItem(c *gin.Context) {
	id := c.Param("id")
	var item models.CmsItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
		return
	}

	userID, username := h.currentUser(c)

	database.DB.Where("cms_item_id = ?", item.ID).Delete(&models.CmsLocalization{})
	if err := database.DB.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.auditService.LogDelete(userID, username, "cms_item", item.ID, item.Identifier, item, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "CMS item deleted"})
}

// ─── CMS Localizations ───────────────────────────────────────────────────────

// ListLocalizations returns all localizations for a CMS item.
// @Summary      List localizations
// @Description  Get all localizations for a CMS item
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "CMS item ID (UUID)"
// @Success      200  {array}   models.CmsLocalization
// @Failure      401  {object}  map[string]string
// @Router       /cms/items/{id}/localizations [get]
func (h *CmsItemHandler) ListLocalizations(c *gin.Context) {
	itemID := c.Param("id")
	var localizations []models.CmsLocalization
	if err := database.DB.Where("cms_item_id = ?", itemID).Find(&localizations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, localizations)
}

// GetLocalization returns the latest localization for a CMS item + locale + stage.
// @Summary      Get localization
// @Description  Get the latest localization for a CMS item by locale and stage
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "CMS item ID (UUID)"
// @Param        locale  query     string  false  "Locale code (default: en)"
// @Param        stage   query     string  false  "Stage: draft | staging | production (default: draft)"
// @Success      200  {object}  models.CmsLocalization
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/detail [get]
func (h *CmsItemHandler) GetLocalization(c *gin.Context) {
	itemID := c.Param("id")
	locale := c.Query("locale")
	stage := models.DeploymentStage(c.Query("stage"))
	if locale == "" {
		locale = "en"
	}
	if stage == "" {
		stage = models.StageDraft
	}

	var loc models.CmsLocalization
	if err := database.DB.
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemID, locale, stage).
		Order("version DESC").
		First(&loc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Localization not found"})
		return
	}
	c.JSON(http.StatusOK, loc)
}

type saveCmsLocalizationBody struct {
	Locale string       `json:"locale" binding:"required"`
	Stage  string       `json:"stage" binding:"required"`
	Data   models.JSONB `json:"data" binding:"required"`
}

// SaveLocalization creates a new version of a localization (always appends).
// @Summary      Save localization
// @Description  Creates a new version of a CMS localization (non-destructive — always appends)
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                   true  "CMS item ID (UUID)"
// @Param        body  body      saveCmsLocalizationBody  true  "Localization data"
// @Success      201  {object}  models.CmsLocalization
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations [post]
func (h *CmsItemHandler) SaveLocalization(c *gin.Context) {
	itemID := c.Param("id")
	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	var item models.CmsItem
	if err := database.DB.Preload("Template.Fields").First(&item, "id = ?", itemUUID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
		return
	}

	var body saveCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stage := models.DeploymentStage(body.Stage)
	if stage != models.StageDraft && stage != models.StageStaging && stage != models.StageProduction {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stage"})
		return
	}

	userID, _ := h.currentUser(c)

	// Determine next version
	var latestVersion int
	database.DB.Model(&models.CmsLocalization{}).
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemUUID, body.Locale, stage).
		Select("COALESCE(MAX(version), 0)").
		Scan(&latestVersion)

	loc := models.CmsLocalization{
		CmsItemID: itemUUID,
		Locale:    body.Locale,
		Stage:     stage,
		Version:   latestVersion + 1,
		Data:      body.Data,
		IsActive:  true,
		CreatedBy: userID,
		UpdatedBy: userID,
	}

	if err := database.DB.Create(&loc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, loc)
}

type translateCmsLocalizationBody struct {
	SourceLocale string `json:"source_locale" binding:"required"`
	TargetLocale string `json:"target_locale" binding:"required"`
	Stage        string `json:"stage" binding:"required"`
}

// TranslateLocalization enqueues an async AI translation job for a CMS localization.
// @Summary      Translate localization
// @Description  Enqueues an async AI translation job. Poll /cms/translate-jobs/{job_id} for status
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                        true  "CMS item ID (UUID)"
// @Param        body  body      translateCmsLocalizationBody  true  "Translation request"
// @Success      202  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/translate [post]
func (h *CmsItemHandler) TranslateLocalization(c *gin.Context) {
	itemID := c.Param("id")
	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	var item models.CmsItem
	if err := database.DB.First(&item, "id = ?", itemUUID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
		return
	}

	var body translateCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stage := models.DeploymentStage(body.Stage)

	// Verify source localization exists
	var sourceLoc models.CmsLocalization
	if err := database.DB.
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemUUID, body.SourceLocale, stage).
		Order("version DESC").
		First(&sourceLoc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source localization not found"})
		return
	}

	userID, _ := h.currentUser(c)

	job := models.CmsTranslateJob{
		ApplicationID: item.ApplicationID,
		CmsItemID:     itemUUID,
		SourceLocale:  body.SourceLocale,
		TargetLocale:  body.TargetLocale,
		Stage:         stage,
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
		"message": fmt.Sprintf("CMS translation job enqueued. Poll /cms/translate-jobs/%s for status.", job.ID.String()),
	})
}

type backfillCmsLocalizationBody struct {
	SourceLocale  string   `json:"source_locale" binding:"required"`
	TargetLocales []string `json:"target_locales" binding:"required"`
	Stage         string   `json:"stage" binding:"required"`
}

// BackfillLocalizations enqueues one async AI translation job per target locale.
// @Summary      Backfill CMS localizations (async)
// @Description  Enqueue OpenAI translation jobs for multiple target locales. Poll each job_id for individual status.
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                       true  "CMS item ID (UUID)"
// @Param        body  body      backfillCmsLocalizationBody  true  "Backfill request"
// @Success      202  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/backfill [post]
func (h *CmsItemHandler) BackfillLocalizations(c *gin.Context) {
	itemID := c.Param("id")
	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	var item models.CmsItem
	if err := database.DB.First(&item, "id = ?", itemUUID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
		return
	}

	var body backfillCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(body.TargetLocales) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one target_locale is required"})
		return
	}

	stage := models.DeploymentStage(body.Stage)

	// Verify source localization exists
	var sourceLoc models.CmsLocalization
	if err := database.DB.
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemUUID, body.SourceLocale, stage).
		Order("version DESC").
		First(&sourceLoc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source localization not found"})
		return
	}

	userID, _ := h.currentUser(c)

	jobIDs := make([]string, 0, len(body.TargetLocales))
	for _, targetLocale := range body.TargetLocales {
		if targetLocale == body.SourceLocale {
			continue
		}
		job := models.CmsTranslateJob{
			ApplicationID: item.ApplicationID,
			CmsItemID:     itemUUID,
			SourceLocale:  body.SourceLocale,
			TargetLocale:  targetLocale,
			Stage:         stage,
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
		"message": fmt.Sprintf("%d CMS translation jobs enqueued. Poll /cms/translate-jobs/:job_id for each status.", len(jobIDs)),
	})
}

type deployCmsLocalizationBody struct {
	Locale    string `json:"locale" binding:"required"`
	FromStage string `json:"from_stage" binding:"required"`
	ToStage   string `json:"to_stage" binding:"required"`
}

// DeployLocalization promotes a CMS localization from one stage to the next.
// @Summary      Deploy localization
// @Description  Promotes a localization from one stage to the next (draft→staging or staging→production)
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                      true  "CMS item ID (UUID)"
// @Param        body  body      deployCmsLocalizationBody   true  "Deploy request"
// @Success      200  {object}  models.CmsLocalization
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/deploy [post]
func (h *CmsItemHandler) DeployLocalization(c *gin.Context) {
	itemID := c.Param("id")
	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	var body deployCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fromStage := models.DeploymentStage(body.FromStage)
	toStage := models.DeploymentStage(body.ToStage)

	// Fetch latest source localization
	var source models.CmsLocalization
	if err := database.DB.
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemUUID, body.Locale, fromStage).
		Order("version DESC").
		First(&source).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source localization not found"})
		return
	}

	userID, _ := h.currentUser(c)

	// Determine next version for target stage
	var latestVersion int
	database.DB.Model(&models.CmsLocalization{}).
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemUUID, body.Locale, toStage).
		Select("COALESCE(MAX(version), 0)").
		Scan(&latestVersion)

	deployed := models.CmsLocalization{
		CmsItemID: itemUUID,
		Locale:    body.Locale,
		Stage:     toStage,
		Version:   latestVersion + 1,
		Data:      source.Data,
		IsActive:  true,
		CreatedBy: userID,
		UpdatedBy: userID,
	}

	if err := database.DB.Create(&deployed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, deployed)
}

type revertCmsLocalizationBody struct {
	Locale  string `json:"locale" binding:"required"`
	Stage   string `json:"stage" binding:"required"`
	Version int    `json:"version" binding:"required"`
}

// RevertLocalization creates a new version from a previous version's data.
// @Summary      Revert localization
// @Description  Creates a new version from an older version's data (non-destructive)
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                      true  "CMS item ID (UUID)"
// @Param        body  body      revertCmsLocalizationBody   true  "Revert request"
// @Success      200  {object}  models.CmsLocalization
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/revert [post]
func (h *CmsItemHandler) RevertLocalization(c *gin.Context) {
	itemID := c.Param("id")
	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	var body revertCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var prev models.CmsLocalization
	if err := database.DB.
		Where("cms_item_id = ? AND locale = ? AND stage = ? AND version = ?", itemUUID, body.Locale, body.Stage, body.Version).
		First(&prev).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
		return
	}

	userID, _ := h.currentUser(c)

	var latestVersion int
	database.DB.Model(&models.CmsLocalization{}).
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemUUID, body.Locale, body.Stage).
		Select("COALESCE(MAX(version), 0)").
		Scan(&latestVersion)

	reverted := models.CmsLocalization{
		CmsItemID: itemUUID,
		Locale:    body.Locale,
		Stage:     models.DeploymentStage(body.Stage),
		Version:   latestVersion + 1,
		Data:      prev.Data,
		IsActive:  true,
		CreatedBy: userID,
		UpdatedBy: userID,
	}

	if err := database.DB.Create(&reverted).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reverted)
}

// ListVersions lists all versions of a CMS localization for a given locale + stage.
// @Summary      List localization versions
// @Description  List all versions of a CMS localization for a given locale and stage
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "CMS item ID (UUID)"
// @Param        locale  query     string  true   "Locale code"
// @Param        stage   query     string  false  "Stage: draft | staging | production (default: draft)"
// @Success      200  {array}   models.CmsLocalization
// @Failure      400  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/versions [get]
func (h *CmsItemHandler) ListVersions(c *gin.Context) {
	itemID := c.Param("id")
	locale := c.Query("locale")
	stage := c.Query("stage")
	if locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "locale query param required"})
		return
	}
	if stage == "" {
		stage = "draft"
	}

	var versions []models.CmsLocalization
	if err := database.DB.
		Where("cms_item_id = ? AND locale = ? AND stage = ?", itemID, locale, stage).
		Order("version DESC").
		Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, versions)
}

// GetCmsTranslateJobStatus returns the status of a CmsTranslateJob by ID.
// @Summary      Get translate job status
// @Description  Get the status of a CMS translate job by ID
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        job_id  path      string  true  "Job ID (UUID)"
// @Success      200  {object}  models.CmsTranslateJob
// @Failure      404  {object}  map[string]string
// @Router       /cms/translate-jobs/{job_id} [get]
func (h *CmsItemHandler) GetCmsTranslateJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")
	var job models.CmsTranslateJob
	if err := database.DB.First(&job, "id = ?", jobID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// GetCmsItemByIdentifier returns the latest production localization for a CMS item by identifier.
// Used by the public-facing translation API.
// @Summary      Get CMS item by identifier
// @Description  Returns localized CMS content by identifier. Accessible via API key. Intended for client applications (FE, mobile)
// @Tags         cms-public
// @Produce      json
// @Param        id          path      string  true   "Application ID (UUID)"
// @Param        identifier  path      string  true   "CMS item identifier (e.g. flash_banner)"
// @Param        locale      query     string  false  "Locale code (default: en)"
// @Param        stage       query     string  false  "Stage: draft | staging | production (default: production)"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]string
// @Router       /applications/{id}/cms/{identifier} [get]
func GetCmsItemByIdentifier(c *gin.Context) {
	appID := c.Param("id")
	identifier := c.Param("identifier")
	locale := c.Query("locale")
	stageStr := c.Query("stage")
	if locale == "" {
		locale = "en"
	}
	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageProduction
	}

	var item models.CmsItem
	if err := database.DB.
		Where("application_id = ? AND identifier = ?", appID, identifier).
		First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
		return
	}

	var loc models.CmsLocalization
	if err := database.DB.
		Where("cms_item_id = ? AND locale = ? AND stage = ?", item.ID, locale, stage).
		Order("version DESC").
		First(&loc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Localization not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"identifier": item.Identifier,
		"locale":     locale,
		"stage":      stage,
		"data":       loc.Data,
	})
}
