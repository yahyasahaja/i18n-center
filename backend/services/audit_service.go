package services

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
)

type AuditService struct{}

func NewAuditService() *AuditService {
	return &AuditService{}
}

// LogAction logs an audit action
func (s *AuditService) LogAction(
	userID uuid.UUID,
	username string,
	action string, // CREATE, UPDATE, DELETE
	resourceType string, // application, component, translation, user
	resourceID uuid.UUID,
	resourceCode string, // Optional: for applications/components
	changes map[string]interface{}, // Before/after values
	ipAddress string,
	userAgent string,
) error {
	auditLog := models.AuditLog{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceCode: resourceCode,
		Changes:      models.JSONB(changes),
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	return database.DB.Create(&auditLog).Error
}

// LogCreate logs a CREATE action
func (s *AuditService) LogCreate(
	userID uuid.UUID,
	username string,
	resourceType string,
	resourceID uuid.UUID,
	resourceCode string,
	data interface{},
	ipAddress string,
	userAgent string,
) error {
	changes := map[string]interface{}{
		"action": "CREATE",
		"data":   data,
	}
	return s.LogAction(userID, username, "CREATE", resourceType, resourceID, resourceCode, changes, ipAddress, userAgent)
}

// LogUpdate logs an UPDATE action with before/after values
func (s *AuditService) LogUpdate(
	userID uuid.UUID,
	username string,
	resourceType string,
	resourceID uuid.UUID,
	resourceCode string,
	before interface{},
	after interface{},
	ipAddress string,
	userAgent string,
) error {
	changes := map[string]interface{}{
		"action": "UPDATE",
		"before": before,
		"after":  after,
	}
	return s.LogAction(userID, username, "UPDATE", resourceType, resourceID, resourceCode, changes, ipAddress, userAgent)
}

// LogDelete logs a DELETE action
func (s *AuditService) LogDelete(
	userID uuid.UUID,
	username string,
	resourceType string,
	resourceID uuid.UUID,
	resourceCode string,
	data interface{}, // Data before deletion
	ipAddress string,
	userAgent string,
) error {
	changes := map[string]interface{}{
		"action": "DELETE",
		"data":   data,
	}
	return s.LogAction(userID, username, "DELETE", resourceType, resourceID, resourceCode, changes, ipAddress, userAgent)
}

// GetAuditLogs retrieves audit logs with filters
func (s *AuditService) GetAuditLogs(
	resourceType string,
	resourceID uuid.UUID,
	limit int,
) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	query := database.DB.Order("created_at DESC")

	if resourceType != "" {
		query = query.Where("resource_type = ?", resourceType)
	}

	if resourceID != uuid.Nil {
		query = query.Where("resource_id = ?", resourceID)
	}

	if limit > 0 {
		query = query.Limit(limit)
	} else {
		query = query.Limit(100) // Default limit
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// GetAuditLogsByUser retrieves audit logs for a specific user
func (s *AuditService) GetAuditLogsByUser(
	userID uuid.UUID,
	limit int,
) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	query := database.DB.Where("user_id = ?", userID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	} else {
		query = query.Limit(100)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// GetChangesForResource gets all changes for a specific resource
func (s *AuditService) GetChangesForResource(
	resourceType string,
	resourceID uuid.UUID,
) ([]models.AuditLog, error) {
	return s.GetAuditLogs(resourceType, resourceID, 0)
}

// CompareValues creates a diff map showing what changed
func CompareValues(before, after interface{}) map[string]interface{} {
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)

	var beforeMap map[string]interface{}
	var afterMap map[string]interface{}

	json.Unmarshal(beforeJSON, &beforeMap)
	json.Unmarshal(afterJSON, &afterMap)

	diff := make(map[string]interface{})

	// Find changed fields
	for key, afterValue := range afterMap {
		beforeValue, exists := beforeMap[key]
		if !exists || fmt.Sprintf("%v", beforeValue) != fmt.Sprintf("%v", afterValue) {
			diff[key] = map[string]interface{}{
				"before": beforeValue,
				"after":  afterValue,
			}
		}
	}

	// Find deleted fields
	for key, beforeValue := range beforeMap {
		if _, exists := afterMap[key]; !exists {
			diff[key] = map[string]interface{}{
				"before": beforeValue,
				"after":  nil,
			}
		}
	}

	return diff
}

