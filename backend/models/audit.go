// Package models holds the legacy struct definitions that were GORM-tagged.
// As of Commit I they are no longer the persistence layer — the repository
// packages (`repository/...`) own the database I/O. These types stick around
// because:
//
//   - Swagger annotations across the handlers still reference `models.X`.
//   - The legacy AuditServicer interface returns `[]models.AuditLog` for
//     backwards compatibility with the mock implementation. New code should
//     prefer the repository types directly (`audit.Log`, `application.Application`).
//
// GORM imports and hooks have been removed; struct tags now only carry
// `json` and `db` annotations.
package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog represents an audit-trail row. The repository layer (`repository/audit`)
// owns the canonical type; this struct is preserved for callers that still
// consume the legacy shape.
type AuditLog struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	UserID       uuid.UUID `db:"user_id"       json:"user_id"`
	Username     string    `db:"username"      json:"username"`
	Action       string    `db:"action"        json:"action"`
	ResourceType string    `db:"resource_type" json:"resource_type"`
	ResourceID   uuid.UUID `db:"resource_id"   json:"resource_id"`
	ResourceCode string    `db:"resource_code" json:"resource_code"`
	Changes      JSONB     `db:"changes"       json:"changes"`
	IPAddress    string    `db:"ip_address"    json:"ip_address"`
	UserAgent    string    `db:"user_agent"    json:"user_agent"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}
