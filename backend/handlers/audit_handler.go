package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/services"
)

type AuditHandler struct {
	auditService *services.AuditService
}

func NewAuditHandler() *AuditHandler {
	return &AuditHandler{
		auditService: services.NewAuditService(),
	}
}

// GetAuditLogs retrieves audit logs
// @Summary      Get audit logs
// @Description  Get audit logs with optional filters
// @Tags         audit
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        resource_type  query     string  false  "Filter by resource type (application, component, translation, user)"
// @Param        resource_id    query     string  false  "Filter by resource ID"
// @Param        user_id        query     string  false  "Filter by user ID"
// @Param        limit          query     int     false  "Limit results (default: 100, max: 1000)"
// @Success      200            {array}   models.AuditLog
// @Failure      400            {object}  map[string]string
// @Failure      401            {object}  map[string]string
// @Router       /audit/logs [get]
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	resourceType := c.Query("resource_type")
	resourceIDStr := c.Query("resource_id")
	userIDStr := c.Query("user_id")
	limitStr := c.Query("limit")

	var resourceID uuid.UUID
	if resourceIDStr != "" {
		var err error
		resourceID, err = uuid.Parse(resourceIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid resource_id format"})
			return
		}
	}

	var userID uuid.UUID
	if userIDStr != "" {
		var err error
		userID, err = uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id format"})
			return
		}
	}

	limit := 100 // default
	if limitStr != "" {
		var err error
		limit, err = parseInt(limitStr)
		if err != nil || limit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
			return
		}
		if limit > 1000 {
			limit = 1000 // max
		}
	}

	var logs []interface{}

	if userID != uuid.Nil {
		// Get logs by user
		auditLogs, err := h.auditService.GetAuditLogsByUser(userID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, log := range auditLogs {
			logs = append(logs, log)
		}
	} else if resourceType != "" || resourceID != uuid.Nil {
		// Get logs by resource
		auditLogs, err := h.auditService.GetAuditLogs(resourceType, resourceID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, log := range auditLogs {
			logs = append(logs, log)
		}
	} else {
		// Get all logs
		auditLogs, err := h.auditService.GetAuditLogs("", uuid.Nil, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, log := range auditLogs {
			logs = append(logs, log)
		}
	}

	c.JSON(http.StatusOK, logs)
}

// GetResourceHistory gets change history for a specific resource
// @Summary      Get resource history
// @Description  Get all changes for a specific resource
// @Tags         audit
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        resource_type  path      string  true   "Resource type (application, component, translation, user)"
// @Param        resource_id    path      string  true   "Resource ID"
// @Success      200            {array}   models.AuditLog
// @Failure      400            {object}  map[string]string
// @Failure      401            {object}  map[string]string
// @Router       /audit/history/{resource_type}/{resource_id} [get]
func (h *AuditHandler) GetResourceHistory(c *gin.Context) {
	resourceType := c.Param("resource_type")
	resourceIDStr := c.Param("resource_id")

	resourceID, err := uuid.Parse(resourceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid resource_id format"})
		return
	}

	logs, err := h.auditService.GetChangesForResource(resourceType, resourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// Helper function to parse integer
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

