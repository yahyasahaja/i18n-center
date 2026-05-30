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
	pagerepo "github.com/your-org/i18n-center/repository/page"
	"github.com/your-org/i18n-center/services"
)

type PageHandler struct {
	auditService services.AuditServicer
	pages        pagerepo.Repository
	components   component.Repository
}

func NewPageHandler() *PageHandler {
	return &PageHandler{
		auditService: services.NewAuditService(),
		pages:        pagerepo.New(),
		components:   component.New(),
	}
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

// ListByApplication returns all pages for an application.
func (h *PageHandler) ListByApplication(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	pages, err := h.pages.ListByApp(c.Request.Context(), database.SQLX, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pages)
}

// Create creates a page for an application.
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

	p := pagerepo.Page{ApplicationID: appID, Code: code}
	if err := h.pages.Create(c.Request.Context(), database.SQLX, &p); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Page code already exists for this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogCreate(userID, username, "page", p.ID, p.Code, p, ipAddress, userAgent)
	c.JSON(http.StatusCreated, p)
}

// Get returns a single page by ID.
func (h *PageHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page ID"})
		return
	}
	p, err := h.pages.GetByID(c.Request.Context(), database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// Update updates a page's code.
func (h *PageHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page ID"})
		return
	}
	ctx := c.Request.Context()
	p, err := h.pages.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
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
	before := pagerepo.Page{Code: p.Code}
	if body.Code != "" {
		p.Code = strings.TrimSpace(strings.ToLower(body.Code))
	}

	if err := h.pages.Update(ctx, database.SQLX, p); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Page code already exists for this application"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogUpdate(userID, username, "page", p.ID, p.Code, before, *p, ipAddress, userAgent)
	c.JSON(http.StatusOK, p)
}

// Delete soft-deletes a page.
func (h *PageHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page ID"})
		return
	}
	ctx := c.Request.Context()
	p, err := h.pages.GetByID(ctx, database.SQLX, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.pages.SoftDelete(ctx, database.SQLX, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogDelete(userID, username, "page", p.ID, p.Code, p, ipAddress, userAgent)
	c.JSON(http.StatusOK, gin.H{"message": "Page deleted"})
}

// attachComponentsBody is the request body for the bulk attach endpoint.
type attachComponentsBody struct {
	ComponentIDs []string `json:"component_ids" binding:"required"`
}

// AttachComponents bulk-adds components to this page. Idempotent — re-attaching
// already-linked components is a no-op (driven by ON CONFLICT DO NOTHING on
// the component_pages composite primary key). Replaces the FE's previous N×2
// round-trip loop where each component required a GET + PUT.
//
// @Summary      Attach components to a page (bulk, idempotent)
// @Description  Adds the given component IDs to this page. Already-attached IDs are silently ignored. Component IDs that don't exist or are soft-deleted are filtered out at the DB level — no error is returned for them.
// @Tags         pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                 true  "Page ID (UUID)"
// @Param        body  body  attachComponentsBody   true  "Component IDs to attach"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Router       /pages/{id}/components [post]
func (h *PageHandler) AttachComponents(c *gin.Context) {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page ID"})
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
	p, err := h.pages.GetByID(ctx, database.SQLX, pageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attached, err := h.pages.AttachComponents(ctx, database.SQLX, pageID, componentIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit-log only when we actually attached something — re-running with an
	// already-fully-attached set should be silent (matches operator intent).
	if attached > 0 {
		userID, username := h.getCurrentUser(c)
		ipAddress, userAgent := h.getClientInfo(c)
		_ = h.auditService.LogAction(userID, username, "ATTACH_COMPONENTS", "page", pageID, p.Code,
			map[string]interface{}{
				"action":             "ATTACH_COMPONENTS",
				"requested_count":    len(componentIDs),
				"newly_attached":     attached,
				"requested_ids":      body.ComponentIDs,
			},
			ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, gin.H{
		"page_id":          pageID,
		"requested_count":  len(componentIDs),
		"newly_attached":   attached,
		"already_attached": int64(len(componentIDs)) - attached,
	})
}

// DetachComponent removes a single component from this page.
//
// @Summary      Detach a single component from a page
// @Tags         pages
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Page ID (UUID)"
// @Param        cid  path  string  true  "Component ID (UUID)"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /pages/{id}/components/{cid} [delete]
func (h *PageHandler) DetachComponent(c *gin.Context) {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page ID"})
		return
	}
	componentID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}
	ctx := c.Request.Context()
	p, err := h.pages.GetByID(ctx, database.SQLX, pageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.pages.DetachComponent(ctx, database.SQLX, pageID, componentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Component is not attached to this page"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	_ = h.auditService.LogAction(userID, username, "DETACH_COMPONENT", "page", pageID, p.Code,
		map[string]interface{}{
			"action":       "DETACH_COMPONENT",
			"component_id": componentID.String(),
		},
		ipAddress, userAgent)

	c.JSON(http.StatusOK, gin.H{"message": "Component detached from page"})
}

// parseUUIDs converts a slice of UUID strings into a slice of uuid.UUID,
// rejecting the whole batch on the first parse error (more honest than
// silently dropping bad IDs, which would mask a bug in the caller).
func parseUUIDs(strs []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, errors.New("invalid UUID in component_ids: " + s)
		}
		out = append(out, id)
	}
	return out, nil
}

// GetComponents returns components that have this page.
func (h *PageHandler) GetComponents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page ID"})
		return
	}
	ctx := c.Request.Context()
	if _, err := h.pages.GetByID(ctx, database.SQLX, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	componentIDs, err := h.pages.GetComponentIDs(ctx, database.SQLX, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]*component.Component, 0, len(componentIDs))
	for _, cid := range componentIDs {
		comp, err := h.components.GetByID(ctx, database.SQLX, cid)
		if err != nil {
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
