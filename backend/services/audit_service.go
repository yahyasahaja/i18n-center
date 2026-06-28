package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/database"
	"github.com/lapakgaming/i18n-center/models"
	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/audit"
)

// AuditService is the legacy-shaped facade in front of the new audit
// repository. It keeps the same signatures the handlers already call
// (LogCreate/LogUpdate/LogDelete/...) so converting it to sqlx didn't ripple
// through 8+ handler files.
//
// All persistence goes through audit.Repository now — no GORM left.
type AuditService struct {
	repo audit.Repository
}

func NewAuditService() *AuditService {
	return &AuditService{repo: audit.New()}
}

// logToModel re-shapes the new audit.Log struct into the legacy models.AuditLog
// the handlers (and Swagger annotations) still expect. Once Commit I strips
// the models package, this collapses to the audit.Log type directly.
func logToModel(l audit.Log) models.AuditLog {
	return models.AuditLog{
		ID:           l.ID,
		UserID:       l.UserID,
		Username:     l.Username,
		Action:       l.Action,
		ResourceType: l.ResourceType,
		ResourceID:   l.ResourceID,
		ResourceCode: l.ResourceCode,
		Changes:      models.JSONB(l.Changes),
		IPAddress:    l.IPAddress,
		UserAgent:    l.UserAgent,
		CreatedAt:    l.CreatedAt,
	}
}

// LogAction inserts an arbitrary audit row. Best-effort — failures are
// returned but never block the calling write (handlers log and move on).
func (s *AuditService) LogAction(
	userID uuid.UUID,
	username string,
	action string,
	resourceType string,
	resourceID uuid.UUID,
	resourceCode string,
	changes map[string]interface{},
	ipAddress string,
	userAgent string,
) error {
	row := &audit.Log{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceCode: resourceCode,
		Changes:      repository.JSONB(changes),
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}
	return s.repo.Insert(context.Background(), database.SQLX, row)
}

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

func (s *AuditService) LogDelete(
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
		"action": "DELETE",
		"data":   data,
	}
	return s.LogAction(userID, username, "DELETE", resourceType, resourceID, resourceCode, changes, ipAddress, userAgent)
}

// GetAuditLogs returns rows matching the legacy filter shape (resourceType +
// resourceID + limit). Caller's `limit <= 0` falls through to the repo's 50
// default — historically this was 100 here, so we clamp explicitly.
func (s *AuditService) GetAuditLogs(
	resourceType string,
	resourceID uuid.UUID,
	limit int,
) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, _, err := s.repo.List(context.Background(), database.SQLX, audit.ListFilter{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]models.AuditLog, len(rows))
	for i, r := range rows {
		out[i] = logToModel(r)
	}
	return out, nil
}

func (s *AuditService) GetAuditLogsByUser(
	userID uuid.UUID,
	limit int,
) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, _, err := s.repo.List(context.Background(), database.SQLX, audit.ListFilter{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]models.AuditLog, len(rows))
	for i, r := range rows {
		out[i] = logToModel(r)
	}
	return out, nil
}

func (s *AuditService) GetChangesForResource(
	resourceType string,
	resourceID uuid.UUID,
) ([]models.AuditLog, error) {
	rows, err := s.repo.History(context.Background(), database.SQLX, resourceType, resourceID, 0)
	if err != nil {
		return nil, err
	}
	out := make([]models.AuditLog, len(rows))
	for i, r := range rows {
		out[i] = logToModel(r)
	}
	return out, nil
}

// CompareValues creates a diff map showing what changed between two structs.
// Kept here because both before/after handlers and a couple of tests import it.
func CompareValues(before, after interface{}) map[string]interface{} {
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)

	var beforeMap map[string]interface{}
	var afterMap map[string]interface{}

	_ = json.Unmarshal(beforeJSON, &beforeMap)
	_ = json.Unmarshal(afterJSON, &afterMap)

	diff := make(map[string]interface{})

	for key, afterValue := range afterMap {
		beforeValue, exists := beforeMap[key]
		if !exists || fmt.Sprintf("%v", beforeValue) != fmt.Sprintf("%v", afterValue) {
			diff[key] = map[string]interface{}{
				"before": beforeValue,
				"after":  afterValue,
			}
		}
	}

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
