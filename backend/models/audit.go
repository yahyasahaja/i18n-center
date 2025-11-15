package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog represents audit trail for all database changes
type AuditLog struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Username      string         `gorm:"not null" json:"username"`
	Action        string         `gorm:"type:varchar(50);not null;index" json:"action"` // CREATE, UPDATE, DELETE
	ResourceType  string         `gorm:"type:varchar(50);not null;index" json:"resource_type"` // application, component, translation, user
	ResourceID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"resource_id"`
	ResourceCode  string         `gorm:"index" json:"resource_code"` // For applications/components, store code for easier lookup
	Changes       JSONB          `gorm:"type:jsonb" json:"changes"` // Before/after values
	IPAddress     string         `gorm:"type:varchar(45)" json:"ip_address"` // IPv6 compatible
	UserAgent     string         `gorm:"type:text" json:"user_agent"`
	CreatedAt     time.Time      `json:"created_at"`
}

// TableName specifies the table name for AuditLog
func (AuditLog) TableName() string {
	return "audit_logs"
}

// BeforeCreate hook
func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

