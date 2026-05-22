package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/middleware"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/cms"
	"github.com/your-org/i18n-center/repository/job"
	"github.com/your-org/i18n-center/repository/translation"
	"github.com/your-org/i18n-center/services"
)

// normalizeIdentifier matches the SDK's case-folding so an item created as
// "Flash_Banner" in the admin UI is still found when an FE calls
// getCmsContent('flash_banner'). The SDK lowercases before sending; the server
// must do the same on create and read.
func normalizeIdentifier(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type CmsItemHandler struct {
	auditService      services.AuditServicer
	templates         cms.TemplateRepository
	items             cms.ItemRepository
	localizations     cms.LocalizationRepository
	cmsTranslateJobs  job.CmsTranslateRepository
}

func NewCmsItemHandler() *CmsItemHandler {
	templates := cms.NewTemplateRepository()
	return &CmsItemHandler{
		auditService:     services.NewAuditService(),
		templates:        templates,
		items:            cms.NewItemRepository(templates),
		localizations:    cms.NewLocalizationRepository(),
		cmsTranslateJobs: job.NewCmsTranslateRepository(),
	}
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
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Application ID (UUID)"
// @Success      200  {array}   cms.Item
// @Failure      401  {object}  map[string]string
// @Router       /applications/{id}/cms/items [get]
func (h *CmsItemHandler) ListItems(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	ctx := c.Request.Context()
	items, err := h.items.ListByApp(ctx, database.SQLX, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Preload templates for each item so the list page can display template names
	// without an N+1 round-trip from the admin UI. Bounded by item count (~100s).
	for i := range items {
		t, err := h.templates.GetByID(ctx, database.SQLX, items[i].TemplateID)
		if err == nil {
			items[i].Template = t
		}
	}
	c.JSON(http.StatusOK, items)
}

// GetItem returns a single CMS item with its template + fields preloaded.
// @Summary      Get CMS item
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "CMS item ID (UUID)"
// @Success      200  {object}  cms.Item
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id} [get]
func (h *CmsItemHandler) GetItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}
	item, err := h.items.GetByIDWithTemplate(c.Request.Context(), database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string             true  "Application ID (UUID)"
// @Param        body  body      createCmsItemBody  true  "CMS item data"
// @Success      201  {object}  cms.Item
// @Failure      400  {object}  map[string]string
// @Router       /applications/{id}/cms/items [post]
func (h *CmsItemHandler) CreateItem(c *gin.Context) {
	appUUID, err := uuid.Parse(c.Param("id"))
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

	ctx := c.Request.Context()

	// Verify template belongs to the same application. Looking up via the repo
	// + checking ApplicationID is one round-trip instead of GORM's filter on
	// the WHERE clause.
	tmpl, err := h.templates.GetByID(ctx, database.SQLX, templateID)
	if err != nil || tmpl.ApplicationID != appUUID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template not found in this application"})
		return
	}

	userID, username := h.currentUser(c)

	item := cms.Item{
		ApplicationID: appUUID,
		TemplateID:    templateID,
		Identifier:    normalizeIdentifier(body.Identifier),
		Name:          strings.TrimSpace(body.Name),
		Description:   strings.TrimSpace(body.Description),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	if err := h.items.Create(ctx, database.SQLX, &item); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CMS item identifier already exists in this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload with template for the response.
	if reloaded, err := h.items.GetByIDWithTemplate(ctx, database.SQLX, item.ID); err == nil {
		item = *reloaded
	}
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
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string             true  "CMS item ID (UUID)"
// @Param        body  body      updateCmsItemBody  true  "CMS item update data"
// @Success      200  {object}  cms.Item
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id} [put]
func (h *CmsItemHandler) UpdateItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	ctx := c.Request.Context()
	item, err := h.items.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		tmpl, err := h.templates.GetByID(ctx, database.SQLX, tid)
		if err != nil || tmpl.ApplicationID != item.ApplicationID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Template not found in this application"})
			return
		}
		item.TemplateID = tid
	}
	item.UpdatedBy = userID

	if err := h.items.Update(ctx, database.SQLX, item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if reloaded, err := h.items.GetByIDWithTemplate(ctx, database.SQLX, item.ID); err == nil {
		item = reloaded
	}
	h.auditService.LogUpdate(userID, username, "cms_item", item.ID, item.Identifier, nil, item, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, item)
}

// DeleteItem soft-deletes a CMS item. Localizations stay (filtered out at
// read time via deleted_at on the item) so the audit trail is preserved.
// @Summary      Delete CMS item
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "CMS item ID (UUID)"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id} [delete]
func (h *CmsItemHandler) DeleteItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	ctx := c.Request.Context()
	item, err := h.items.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.currentUser(c)
	if err := h.items.SoftDelete(ctx, database.SQLX, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.auditService.LogDelete(userID, username, "cms_item", item.ID, item.Identifier, item, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "CMS item deleted"})
}

// ─── CMS Localizations ───────────────────────────────────────────────────────

// ListLocalizations returns all localizations for a CMS item across all locale/stage combinations.
// @Summary      List localizations
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "CMS item ID (UUID)"
// @Success      200  {array}   cms.Localization
// @Failure      401  {object}  map[string]string
// @Router       /cms/items/{id}/localizations [get]
func (h *CmsItemHandler) ListLocalizations(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}
	locs, err := h.localizations.ListAll(c.Request.Context(), database.SQLX, itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, locs)
}

// GetLocalization returns the latest localization for (item, locale, stage).
// @Summary      Get localization
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "CMS item ID (UUID)"
// @Param        locale  query     string  false  "Locale code (default: en)"
// @Param        stage   query     string  false  "Stage: draft | staging | production (default: draft)"
// @Success      200  {object}  cms.Localization
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/detail [get]
func (h *CmsItemHandler) GetLocalization(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}
	locale := c.Query("locale")
	if locale == "" {
		locale = "en"
	}
	stage := translation.Stage(c.Query("stage"))
	if stage == "" {
		stage = translation.StageDraft
	}

	loc, err := h.localizations.GetLatest(c.Request.Context(), database.SQLX, itemID, locale, stage)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Localization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loc)
}

type saveCmsLocalizationBody struct {
	Locale string           `json:"locale" binding:"required"`
	Stage  string           `json:"stage" binding:"required"`
	Data   repository.JSONB `json:"data" binding:"required"`
}

// SaveLocalization creates a new version of a localization (always appends).
// @Summary      Save localization
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                   true  "CMS item ID (UUID)"
// @Param        body  body      saveCmsLocalizationBody  true  "Localization data"
// @Success      201  {object}  cms.Localization
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations [post]
func (h *CmsItemHandler) SaveLocalization(c *gin.Context) {
	itemUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	ctx := c.Request.Context()
	if _, err := h.items.GetByID(ctx, database.SQLX, itemUUID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var body saveCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stage := translation.Stage(body.Stage)
	if stage != translation.StageDraft && stage != translation.StageStaging && stage != translation.StageProduction {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stage"})
		return
	}

	userID, _ := h.currentUser(c)
	loc := &cms.Localization{
		CmsItemID: itemUUID,
		Locale:    body.Locale,
		Stage:     stage,
		Data:      body.Data,
		IsActive:  true,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := h.localizations.SaveLocalizationVersion(ctx, database.SQLX, loc); err != nil {
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

// TranslateLocalization enqueues an async AI translation job. CmsTranslateJob
// itself still lives on GORM until Commit H — this handler reads the item via
// the new repo but writes the job row through the legacy ORM.
// @Summary      Translate localization
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
	itemUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	ctx := c.Request.Context()
	item, err := h.items.GetByID(ctx, database.SQLX, itemUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var body translateCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stage := translation.Stage(body.Stage)

	// Verify source localization exists via the new repo.
	if _, err := h.localizations.GetLatest(ctx, database.SQLX, itemUUID, body.SourceLocale, stage); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Source localization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, _ := h.currentUser(c)

	// Idempotency: dedupe double-clicks. The partial unique index
	// idx_cms_translate_jobs_dedupe enforces this at the DB level too — this
	// lookup just gives a clean 202 response when a user double-clicks.
	if existing := h.findActiveCmsTranslateJob(ctx, itemUUID, body.SourceLocale, body.TargetLocale, stage); existing != nil {
		c.JSON(http.StatusAccepted, gin.H{
			"job_id":  existing.ID.String(),
			"status":  existing.Status,
			"deduped": true,
			"message": "Existing CMS translation job is " + existing.Status + ".",
		})
		return
	}

	newJob := &job.CmsTranslateJob{
		ApplicationID: item.ApplicationID,
		CmsItemID:     itemUUID,
		SourceLocale:  body.SourceLocale,
		TargetLocale:  body.TargetLocale,
		Stage:         stage,
		CreatedBy:     userID,
	}
	if err := h.cmsTranslateJobs.Insert(ctx, database.SQLX, newJob); err != nil {
		if existing := h.findActiveCmsTranslateJob(ctx, itemUUID, body.SourceLocale, body.TargetLocale, stage); existing != nil {
			c.JSON(http.StatusAccepted, gin.H{
				"job_id":  existing.ID.String(),
				"status":  existing.Status,
				"deduped": true,
				"message": "Existing CMS translation job is " + existing.Status + ".",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue translation job"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  newJob.ID.String(),
		"status":  job.StatusPending,
		"message": fmt.Sprintf("CMS translation job enqueued. Poll /cms/translate-jobs/%s for status.", newJob.ID.String()),
	})
}

// findActiveCmsTranslateJob returns the first pending/running CmsTranslateJob
// matching the de-duplication tuple, or nil if none exists. Errors are
// swallowed — the caller treats nil as "no in-flight job" and proceeds.
func (h *CmsItemHandler) findActiveCmsTranslateJob(ctx context.Context, cmsItemID uuid.UUID, sourceLocale, targetLocale string, stage translation.Stage) *job.CmsTranslateJob {
	existing, err := h.cmsTranslateJobs.FindActive(ctx, database.SQLX, cmsItemID, sourceLocale, targetLocale, stage)
	if err != nil {
		return nil
	}
	return existing
}

type backfillCmsLocalizationBody struct {
	SourceLocale  string   `json:"source_locale" binding:"required"`
	TargetLocales []string `json:"target_locales" binding:"required"`
	Stage         string   `json:"stage" binding:"required"`
}

// BackfillLocalizations enqueues one async AI translation job per target locale.
// @Summary      Backfill CMS localizations (async)
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                       true  "CMS item ID (UUID)"
// @Param        body  body      backfillCmsLocalizationBody  true  "Backfill request"
// @Success      202  {object}  map[string]interface{}
// @Router       /cms/items/{id}/localizations/backfill [post]
func (h *CmsItemHandler) BackfillLocalizations(c *gin.Context) {
	itemUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	ctx := c.Request.Context()
	item, err := h.items.GetByID(ctx, database.SQLX, itemUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	stage := translation.Stage(body.Stage)

	// Verify source localization exists.
	if _, err := h.localizations.GetLatest(ctx, database.SQLX, itemUUID, body.SourceLocale, stage); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Source localization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, _ := h.currentUser(c)
	jobIDs := make([]string, 0, len(body.TargetLocales))
	dedupedCount := 0
	for _, targetLocale := range body.TargetLocales {
		if targetLocale == body.SourceLocale {
			continue
		}
		if existing := h.findActiveCmsTranslateJob(ctx, itemUUID, body.SourceLocale, targetLocale, stage); existing != nil {
			jobIDs = append(jobIDs, existing.ID.String())
			dedupedCount++
			continue
		}
		newJob := &job.CmsTranslateJob{
			ApplicationID: item.ApplicationID,
			CmsItemID:     itemUUID,
			SourceLocale:  body.SourceLocale,
			TargetLocale:  targetLocale,
			Stage:         stage,
			CreatedBy:     userID,
		}
		if err := h.cmsTranslateJobs.Insert(ctx, database.SQLX, newJob); err != nil {
			if existing := h.findActiveCmsTranslateJob(ctx, itemUUID, body.SourceLocale, targetLocale, stage); existing != nil {
				jobIDs = append(jobIDs, existing.ID.String())
				dedupedCount++
				continue
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue job for locale " + targetLocale})
			return
		}
		jobIDs = append(jobIDs, newJob.ID.String())
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_ids":       jobIDs,
		"count":         len(jobIDs),
		"deduped_count": dedupedCount,
		"message":       fmt.Sprintf("%d CMS translation jobs (%d already in-flight).", len(jobIDs)-dedupedCount, dedupedCount),
	})
}

type deployCmsLocalizationBody struct {
	Locale    string `json:"locale" binding:"required"`
	FromStage string `json:"from_stage" binding:"required"`
	ToStage   string `json:"to_stage" binding:"required"`
}

// DeployLocalization promotes a CMS localization from one stage to the next.
// Reads source from fromStage, writes a new row at toStage with the same data.
// @Summary      Deploy localization
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                      true  "CMS item ID (UUID)"
// @Param        body  body      deployCmsLocalizationBody   true  "Deploy request"
// @Success      200  {object}  cms.Localization
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/deploy [post]
func (h *CmsItemHandler) DeployLocalization(c *gin.Context) {
	itemUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	var body deployCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fromStage := translation.Stage(body.FromStage)
	toStage := translation.Stage(body.ToStage)

	ctx := c.Request.Context()
	source, err := h.localizations.GetLatest(ctx, database.SQLX, itemUUID, body.Locale, fromStage)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Source localization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, _ := h.currentUser(c)
	deployed := &cms.Localization{
		CmsItemID: itemUUID,
		Locale:    body.Locale,
		Stage:     toStage,
		Data:      source.Data,
		IsActive:  true,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := h.localizations.SaveLocalizationVersion(ctx, database.SQLX, deployed); err != nil {
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
// Non-destructive — the old version stays in the history.
// @Summary      Revert localization
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                      true  "CMS item ID (UUID)"
// @Param        body  body      revertCmsLocalizationBody   true  "Revert request"
// @Success      200  {object}  cms.Localization
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/revert [post]
func (h *CmsItemHandler) RevertLocalization(c *gin.Context) {
	itemUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}

	var body revertCmsLocalizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	stage := translation.Stage(body.Stage)
	prev, err := h.localizations.GetByVersion(ctx, database.SQLX, itemUUID, body.Locale, stage, body.Version)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, _ := h.currentUser(c)
	reverted := &cms.Localization{
		CmsItemID: itemUUID,
		Locale:    body.Locale,
		Stage:     stage,
		Data:      prev.Data,
		IsActive:  true,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := h.localizations.SaveLocalizationVersion(ctx, database.SQLX, reverted); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reverted)
}

// ListVersions lists all versions of a CMS localization for (locale, stage), newest first.
// @Summary      List localization versions
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "CMS item ID (UUID)"
// @Param        locale  query     string  true   "Locale code"
// @Param        stage   query     string  false  "Stage: draft | staging | production (default: draft)"
// @Success      200  {array}   cms.Localization
// @Failure      400  {object}  map[string]string
// @Router       /cms/items/{id}/localizations/versions [get]
func (h *CmsItemHandler) ListVersions(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS item ID"})
		return
	}
	locale := c.Query("locale")
	if locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "locale query param required"})
		return
	}
	stage := translation.Stage(c.Query("stage"))
	if stage == "" {
		stage = translation.StageDraft
	}

	versions, err := h.localizations.ListVersions(c.Request.Context(), database.SQLX, itemID, locale, stage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, versions)
}

// GetCmsTranslateJobStatus returns the status of a CmsTranslateJob by ID.
// @Summary      Get translate job status
// @Tags         cms
// @Produce      json
// @Security     BearerAuth
// @Param        job_id  path      string  true  "Job ID (UUID)"
// @Success      200  {object}  job.CmsTranslateJob
// @Failure      404  {object}  map[string]string
// @Router       /cms/translate-jobs/{job_id} [get]
func (h *CmsItemHandler) GetCmsTranslateJobStatus(c *gin.Context) {
	jobIDStr := c.Param("job_id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}
	j, err := h.cmsTranslateJobs.GetByID(c.Request.Context(), database.SQLX, jobID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, j)
}

// GetCmsItemByIdentifier returns the latest production localization for a CMS item.
// Public-facing read; accepts either JWT or API key (scoped to the URL's application).
// @Summary      Get CMS item by identifier
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
	appIDStr := c.Param("id")
	identifier := normalizeIdentifier(c.Param("identifier"))
	locale := c.Query("locale")
	stageStr := c.Query("stage")
	if locale == "" {
		locale = "en"
	}
	stage := translation.Stage(stageStr)
	if stage == "" {
		stage = translation.StageProduction
	}

	applicationID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	// API-key application scoping (cross-tenant data leak prevention).
	if apiKeyAppID := middleware.GetAPIKeyApplicationID(c); apiKeyAppID != uuid.Nil && apiKeyAppID != applicationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "API key does not have access to this application"})
		return
	}

	ctx := c.Request.Context()
	items := cms.NewItemRepository(cms.NewTemplateRepository())
	locs := cms.NewLocalizationRepository()

	item, err := items.GetByAppIdentifier(ctx, database.SQLX, applicationID, identifier)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "CMS item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	loc, err := locs.GetLatest(ctx, database.SQLX, item.ID, locale, stage)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Localization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cloudflare-cacheable for production; private no-store for draft/staging
	// so operator edits show immediately.
	if stage == translation.StageProduction {
		c.Header("Cache-Control", "public, max-age=60, s-maxage=300, stale-while-revalidate=600")
		c.Header("Vary", "X-API-Key, Authorization, Accept-Encoding")
	} else {
		c.Header("Cache-Control", "private, no-store")
	}
	c.JSON(http.StatusOK, gin.H{
		"identifier": item.Identifier,
		"locale":     locale,
		"stage":      stage,
		"data":       loc.Data,
	})
}
