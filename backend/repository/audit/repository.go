// Package audit is the data access layer for `audit_logs` — the immutable
// trail of CREATE/UPDATE/DELETE/DEPLOY/etc. actions performed by users.
//
// Audit rows are never updated or deleted (no soft-delete column either).
// The retention job (Commit J) keeps them forever — they ARE the recovery
// for accidental data loss.
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

// Log is one row from audit_logs.
type Log struct {
	ID           uuid.UUID        `db:"id"            json:"id"`
	UserID       uuid.UUID        `db:"user_id"       json:"user_id"`
	Username     string           `db:"username"      json:"username"`
	Action       string           `db:"action"        json:"action"`
	ResourceType string           `db:"resource_type" json:"resource_type"`
	ResourceID   uuid.UUID        `db:"resource_id"   json:"resource_id"`
	ResourceCode string           `db:"resource_code" json:"resource_code"`
	Changes      repository.JSONB `db:"changes"       json:"changes"`
	IPAddress    string           `db:"ip_address"    json:"ip_address"`
	UserAgent    string           `db:"user_agent"    json:"user_agent"`
	CreatedAt    time.Time        `db:"created_at"    json:"created_at"`
}

// ListFilter shapes the WHERE clause for audit lookups. All fields are optional.
type ListFilter struct {
	UserID       uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	Action       string
	Limit        int
	Offset       int
}

// Repository is the contract for audit log persistence. Insert-only;
// read paths support filtering and pagination.
type Repository interface {
	// Insert appends a new audit row. Failures are still logged via the
	// service layer but never block the originating write — audit is best-
	// effort observability, not a transactional invariant.
	Insert(ctx context.Context, q repository.Queryer, l *Log) error

	// List returns audit rows matching the filter, newest first, with the
	// returned total count for paginator UIs.
	List(ctx context.Context, q repository.Queryer, f ListFilter) (rows []Log, total int, err error)

	// History returns the full audit timeline for (resourceType, resourceID),
	// newest first. Bounded by `limit`; if 0 we return everything.
	History(ctx context.Context, q repository.Queryer, resourceType string, resourceID uuid.UUID, limit int) ([]Log, error)
}
