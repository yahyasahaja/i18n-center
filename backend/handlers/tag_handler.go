package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/component"
	tagrepo "github.com/your-org/i18n-center/repository/tag"
	"github.com/your-org/i18n-center/services"
)

type TagHandler struct {
	auditService services.AuditServicer
	tags         tagrepo.Repository
	components   component.Repository
}

func NewTagHandler() *TagHandler {
	return &TagHandler{
		auditService: services.NewAuditService(),
		tags:         tagrepo.New(),
		components:   component.New(),
	}
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

// ListByApplication returns all tags for an application.
func (h *TagHandler) ListByApplication(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	tags, err := h.tags.ListByApp(c.Request.Context(), database.SQLX, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// Create creates a tag for an application. Tag codes are normalised to
// lowercase + trimmed; duplicates within the same application return 400.
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

	t := tagrepo.Tag{ApplicationID: appID, Code: code}
	ctx := c.Request.Context()
	if err := h.tags.Create(ctx, database.SQLX, &t); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tag code already exists for this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogCreate(userID, username, "tag", t.ID, t.Code, t, ipAddress, userAgent)
	c.JSON(http.StatusCreated, t)
}

// Get returns a single tag by ID.
func (h *TagHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}
	t, err := h.tags.GetByID(c.Request.Context(), database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// Update updates a tag's code. Other fields are immutable.
func (h *TagHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}
	ctx := c.Request.Context()
	t, err := h.tags.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before := tagrepo.Tag{Code: t.Code}
	if body.Code != "" {
		t.Code = strings.TrimSpace(strings.ToLower(body.Code))
	}

	if err := h.tags.Update(ctx, database.SQLX, t); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tag code already exists for this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogUpdate(userID, username, "tag", t.ID, t.Code, before, *t, ipAddress, userAgent)
	c.JSON(http.StatusOK, t)
}

// Delete soft-deletes a tag. Junction rows in component_tags survive but
// are filtered out at read time via the deleted_at check.
func (h *TagHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}
	ctx := c.Request.Context()
	t, err := h.tags.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.tags.SoftDelete(ctx, database.SQLX, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogDelete(userID, username, "tag", t.ID, t.Code, t, ipAddress, userAgent)
	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted"})
}

// AttachComponents bulk-adds components to this tag. Idempotent — already-attached
// IDs are silently no-op (driven by ON CONFLICT DO NOTHING on the
// component_tags composite primary key).
//
// @Summary      Attach components to a tag (bulk, idempotent)
// @Description  Adds the given component IDs to this tag. Already-attached IDs are silently ignored. Component IDs that don't exist or are soft-deleted are filtered at the DB level — no error is returned for them.
// @Tags         tags
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                 true  "Tag ID (UUID)"
// @Param        body  body  attachComponentsBody   true  "Component IDs to attach"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Router       /tags/{id}/components [post]
func (h *TagHandler) AttachComponents(c *gin.Context) {
	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}
	var body attachComponentsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	componentIDs, err := parseUUIDs(body.ComponentIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	t, err := h.tags.GetByID(ctx, database.SQLX, tagID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attached, err := h.tags.AttachComponents(ctx, database.SQLX, tagID, componentIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if attached > 0 {
		userID, username := h.getCurrentUser(c)
		ipAddress, userAgent := h.getClientInfo(c)
		_ = h.auditService.LogAction(userID, username, "ATTACH_COMPONENTS", "tag", tagID, t.Code,
			map[string]interface{}{
				"action":             "ATTACH_COMPONENTS",
				"requested_count":    len(componentIDs),
				"newly_attached":     attached,
				"requested_ids":      body.ComponentIDs,
			},
			ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, gin.H{
		"tag_id":           tagID,
		"requested_count":  len(componentIDs),
		"newly_attached":   attached,
		"already_attached": int64(len(componentIDs)) - attached,
	})
}

// DetachComponent removes a single component from this tag.
//
// @Summary      Detach a single component from a tag
// @Tags         tags
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Tag ID (UUID)"
// @Param        cid  path  string  true  "Component ID (UUID)"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /tags/{id}/components/{cid} [delete]
func (h *TagHandler) DetachComponent(c *gin.Context) {
	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}
	componentID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}
	ctx := c.Request.Context()
	t, err := h.tags.GetByID(ctx, database.SQLX, tagID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.tags.DetachComponent(ctx, database.SQLX, tagID, componentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Component is not attached to this tag"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	_ = h.auditService.LogAction(userID, username, "DETACH_COMPONENT", "tag", tagID, t.Code,
		map[string]interface{}{
			"action":       "DETACH_COMPONENT",
			"component_id": componentID.String(),
		},
		ipAddress, userAgent)

	c.JSON(http.StatusOK, gin.H{"message": "Component detached from tag"})
}

// GetComponents returns components that have this tag.
func (h *TagHandler) GetComponents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}
	ctx := c.Request.Context()
	// Verify the tag exists (and isn't soft-deleted) so we can return a clear
	// 404 rather than an empty array for a non-existent tag.
	if _, err := h.tags.GetByID(ctx, database.SQLX, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	componentIDs, err := h.tags.GetComponentIDs(ctx, database.SQLX, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Fan out to load each component. At current scale (one tag ≪ one app's
	// components, almost always < 100) the N+1 is fine. If a tag ends up with
	// thousands of components, switch to a single IN-clause SELECT.
	out := make([]*component.Component, 0, len(componentIDs))
	for _, cid := range componentIDs {
		comp, err := h.components.GetByID(ctx, database.SQLX, cid)
		if err != nil {
			// Skip rather than fail the whole list — a junction row may
			// reference a component that was just soft-deleted.
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out = append(out, comp)
	}
	c.JSON(http.StatusOK, out)
}
