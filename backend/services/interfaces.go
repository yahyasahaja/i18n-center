package services

import (
	"github.com/google/uuid"
	"github.com/lapakgaming/i18n-center/models"
)

// AuditServicer defines the audit logging interface used by handlers.
// Concrete implementation: *AuditService.
type AuditServicer interface {
	// LogCreate logs a resource creation event.
	LogCreate(userID uuid.UUID, username, resourceType string, resourceID uuid.UUID, resourceCode string, data interface{}, ipAddress, userAgent string) error
	// LogUpdate logs a resource update event with before/after state.
	LogUpdate(userID uuid.UUID, username, resourceType string, resourceID uuid.UUID, resourceCode string, before, after interface{}, ipAddress, userAgent string) error
	// LogDelete logs a resource deletion event.
	LogDelete(userID uuid.UUID, username, resourceType string, resourceID uuid.UUID, resourceCode string, data interface{}, ipAddress, userAgent string) error
	// LogAction logs an arbitrary audit action (used by translation deploy).
	LogAction(userID uuid.UUID, username, action, resourceType string, resourceID uuid.UUID, resourceCode string, changes map[string]interface{}, ipAddress, userAgent string) error
	// GetAuditLogs retrieves audit logs with optional filters.
	GetAuditLogs(resourceType string, resourceID uuid.UUID, limit int) ([]models.AuditLog, error)
	// GetAuditLogsByUser retrieves audit logs for a specific user.
	GetAuditLogsByUser(userID uuid.UUID, limit int) ([]models.AuditLog, error)
	// GetChangesForResource retrieves all audit logs for a resource.
	GetChangesForResource(resourceType string, resourceID uuid.UUID) ([]models.AuditLog, error)
}
