package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/component"
	"github.com/your-org/i18n-center/services"
)

type ComponentHandler struct {
	auditService services.AuditServicer
	components   component.Repository
}

func NewComponentHandler() *ComponentHandler {
	return &ComponentHandler{
		auditService: services.NewAuditService(),
		components:   component.New(),
	}
}

// getCurrentUser extracts user info from context.
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

func (h *ComponentHandler) getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	return c.ClientIP(), c.GetHeader("User-Agent")
}

// sanitizeKeyContexts coerces a JSONB blob into a flat {dot.path: non-empty string} map.
// Non-string values and empty strings are dropped so the prompt builder downstream
// can safely treat the result as map[string]string. Returns nil for an effectively
// empty map so the column stores SQL NULL rather than '{}'.
func sanitizeKeyContexts(raw repository.JSONB) repository.JSONB {
	if len(raw) == 0 {
		return nil
	}
	out := make(repository.JSONB, len(raw))
	for k, v := range raw {
		s, ok := v.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		out[k] = trimmed
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseUUIDList parses a list of UUID-as-string into []uuid.UUID, silently
// dropping unparseable entries. Used for tag_ids / page_ids in the request body.
func parseUUIDList(ss []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ss))
	for _, s := range ss {
		if id, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// GetComponents lists components with optional pagination, search, and application filter.
// @Summary      List components
// @Description  Get components with pagination and search
// @Tags         components
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        application_id  query     string  false  "Filter by application ID"
// @Param        search          query     string  false  "Search by name or code (case-insensitive)"
// @Param        page            query     int     false  "Page number (default: 1)"
// @Param        page_size       query     int     false  "Page size (default: 20, max: 100)"
// @Success      200            {object}  map[string]interface{}
// @Failure      401            {object}  map[string]string
// @Router       /components [get]
func (h *ComponentHandler) GetComponents(c *gin.Context) {
	applicationIDStr := c.Query("application_id")
	search := strings.TrimSpace(c.Query("search"))

	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if v, err := parsePositiveInt(p); err == nil {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := parsePositiveInt(ps); err == nil && v <= 100 {
			pageSize = v
		}
	}
	offset := (page - 1) * pageSize

	filter := component.ListFilter{
		Search: search,
		Limit:  pageSize,
		Offset: offset,
	}
	if applicationIDStr != "" {
		if appID, err := uuid.Parse(applicationIDStr); err == nil {
			filter.ApplicationID = appID
		}
	}

	ctx := c.Request.Context()
	rows, total, err := h.components.List(ctx, database.SQLX, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Load tags + pages for each row. List page sizes are bounded (max 100) so
	// N+1 reads here are acceptable. If we ever serve unbounded lists, fold this
	// into a single JOIN-with-aggregation query.
	for i := range rows {
		if tags, err := h.components.LoadTags(ctx, database.SQLX, rows[i].ID); err == nil {
			rows[i].Tags = tags
		}
		if pages, err := h.components.LoadPages(ctx, database.SQLX, rows[i].ID); err == nil {
			rows[i].Pages = pages
		}
	}

	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        rows,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

func parsePositiveInt(s string) (int, error) {
	var v int
	_, err := fmtSscanf(s, "%d", &v)
	if err != nil || v < 1 {
		return 0, errors.New("invalid positive int")
	}
	return v, nil
}

// fmtSscanf is a tiny shim to keep imports clean. Avoids importing fmt just for
// one Sscanf call here. (It IS importing fmt — left as a deliberate trampoline
// in case we ever want to switch parsing implementations.)
func fmtSscanf(src, format string, a ...any) (int, error) {
	// Inline the trivial integer parse — fmt.Sscanf is fine but we don't want
	// to drag in fmt just for this. strconv.Atoi handles the common case.
	if format == "%d" && len(a) == 1 {
		if dst, ok := a[0].(*int); ok {
			n, err := atoi(src)
			if err != nil {
				return 0, err
			}
			*dst = n
			return 1, nil
		}
	}
	return 0, errors.New("unsupported format")
}

func atoi(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, errors.New("empty")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errors.New("non-digit")
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}

// GetComponent gets a single component (by ID or code).
func (h *ComponentHandler) GetComponent(c *gin.Context) {
	identifier := c.Param("id")

	// Cache lookup keyed by whatever identifier the client used.
	cacheKey := cache.ComponentKey(identifier)
	var cached component.Component
	if err := cache.Get(cacheKey, &cached); err == nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	ctx := c.Request.Context()
	var comp *component.Component

	// Try UUID first; fall back to code if it doesn't parse OR isn't found.
	if id, err := uuid.Parse(identifier); err == nil {
		got, lookupErr := h.components.GetByIDWithRelations(ctx, database.SQLX, id)
		if lookupErr == nil {
			comp = got
		} else if !errors.Is(lookupErr, repository.ErrNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": lookupErr.Error()})
			return
		}
	}
	if comp == nil {
		got, err := h.components.GetByCode(ctx, database.SQLX, identifier)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// We had to fall back to by-code; tags/pages weren't loaded yet.
		if tags, err := h.components.LoadTags(ctx, database.SQLX, got.ID); err == nil {
			got.Tags = tags
		}
		if pages, err := h.components.LoadPages(ctx, database.SQLX, got.ID); err == nil {
			got.Pages = pages
		}
		comp = got
	}

	cache.Set(cacheKey, *comp, 3600*1000000000) // 1 hour
	c.JSON(http.StatusOK, comp)
}

// createComponentBody is the request body for creating a component (includes tag_ids and page_ids).
type createComponentBody struct {
	Name          string           `json:"name" binding:"required"`
	Code          string           `json:"code" binding:"required"`
	ApplicationID uuid.UUID        `json:"application_id" binding:"required"`
	Description   string           `json:"description"`
	DefaultLocale string           `json:"default_locale" binding:"required"`
	KeyContexts   repository.JSONB `json:"key_contexts"`
	TagIDs        []string         `json:"tag_ids"`
	PageIDs       []string         `json:"page_ids"`
}

// CreateComponent creates a new component, optionally attaching tags and pages.
// The component insert + junction attachments run inside a transaction so a
// failed attach rolls the component back.
func (h *ComponentHandler) CreateComponent(c *gin.Context) {
	var body createComponentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	comp := component.Component{
		Name:          strings.TrimSpace(body.Name),
		Code:          strings.TrimSpace(body.Code),
		ApplicationID: body.ApplicationID,
		Description:   strings.TrimSpace(body.Description),
		DefaultLocale: strings.TrimSpace(body.DefaultLocale),
		KeyContexts:   sanitizeKeyContexts(body.KeyContexts),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	ctx := c.Request.Context()
	tagIDs := parseUUIDList(body.TagIDs)
	pageIDs := parseUUIDList(body.PageIDs)

	if err := repository.WithTx(ctx, database.SQLX, func(tx repository.Queryer) error {
		if err := h.components.Create(ctx, tx, &comp); err != nil {
			return err
		}
		if len(tagIDs) > 0 {
			if err := h.components.AttachTags(ctx, tx, comp.ID, tagIDs); err != nil {
				return err
			}
		}
		if len(pageIDs) > 0 {
			if err := h.components.AttachPages(ctx, tx, comp.ID, pageIDs); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Component code already exists for this application"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reload with relations for the response.
	if reloaded, err := h.components.GetByIDWithRelations(ctx, database.SQLX, comp.ID); err == nil {
		comp = *reloaded
	}

	h.auditService.LogCreate(userID, username, "component", comp.ID, comp.Code, comp, ipAddress, userAgent)
	c.JSON(http.StatusCreated, comp)
}

// updateComponentBody is the request body for updating a component.
type updateComponentBody struct {
	Name          *string           `json:"name"`
	Code          *string           `json:"code"`
	Description   *string           `json:"description"`
	DefaultLocale *string           `json:"default_locale"`
	KeyContexts   *repository.JSONB `json:"key_contexts"`
	TagIDs        []string          `json:"tag_ids"`
	PageIDs       []string          `json:"page_ids"`
}

// UpdateComponent updates a component. tag_ids and page_ids, when present
// (non-nil slice — note that an empty array IS distinguishable from a missing
// field at the JSON layer via the slice's nil-vs-empty distinction), replace
// the existing junction sets atomically.
func (h *ComponentHandler) UpdateComponent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	ctx := c.Request.Context()
	comp, err := h.components.GetByIDWithRelations(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	before := component.Component{
		Name:          comp.Name,
		Code:          comp.Code,
		Description:   comp.Description,
		KeyContexts:   comp.KeyContexts,
		DefaultLocale: comp.DefaultLocale,
	}

	var body updateComponentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.Name != nil {
		comp.Name = strings.TrimSpace(*body.Name)
	}
	if body.Code != nil {
		comp.Code = strings.TrimSpace(*body.Code)
	}
	if body.Description != nil {
		comp.Description = strings.TrimSpace(*body.Description)
	}
	if body.DefaultLocale != nil {
		comp.DefaultLocale = strings.TrimSpace(*body.DefaultLocale)
	}
	if body.KeyContexts != nil {
		comp.KeyContexts = sanitizeKeyContexts(*body.KeyContexts)
	}
	comp.UpdatedBy = userID

	if err := repository.WithTx(ctx, database.SQLX, func(tx repository.Queryer) error {
		if err := h.components.Update(ctx, tx, comp); err != nil {
			return err
		}
		// nil slice → don't touch the junction. Empty non-nil slice → clear it.
		if body.TagIDs != nil {
			if err := h.components.AttachTags(ctx, tx, comp.ID, parseUUIDList(body.TagIDs)); err != nil {
				return err
			}
		}
		if body.PageIDs != nil {
			if err := h.components.AttachPages(ctx, tx, comp.ID, parseUUIDList(body.PageIDs)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Component code already exists for this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if reloaded, err := h.components.GetByIDWithRelations(ctx, database.SQLX, comp.ID); err == nil {
		comp = reloaded
	}

	after := component.Component{
		Name:          comp.Name,
		Code:          comp.Code,
		Description:   comp.Description,
		KeyContexts:   comp.KeyContexts,
		DefaultLocale: comp.DefaultLocale,
	}
	h.auditService.LogUpdate(userID, username, "component", comp.ID, comp.Code, before, after, ipAddress, userAgent)
	cache.Delete(cache.ComponentKey(idStr))
	c.JSON(http.StatusOK, comp)
}

// DeleteComponent soft-deletes a component. Junction rows survive but are
// filtered out at read time.
func (h *ComponentHandler) DeleteComponent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	ctx := c.Request.Context()
	comp, err := h.components.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	if err := h.components.SoftDelete(ctx, database.SQLX, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.auditService.LogDelete(
		userID, username, "component",
		comp.ID, comp.Code, comp,
		ipAddress, userAgent,
	)

	cache.Delete(cache.ComponentKey(idStr))
	c.JSON(http.StatusOK, gin.H{"message": "Component deleted"})
}
