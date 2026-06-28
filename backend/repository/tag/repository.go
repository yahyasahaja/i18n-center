// Package tag is the data access layer for `tags` — labels that group
// components within an application. Identified by `code` (unique per
// application). Component↔Tag is many-to-many via the `component_tags` table.
package tag

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

// Tag is the in-memory representation of a row from `tags`.
type Tag struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	Code          string    `db:"code"           json:"code"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
}

// Repository is the contract for tag persistence. All methods take a Queryer
// so they work both inside and outside a transaction.
type Repository interface {
	// GetByID fetches a single tag. ErrNotFound on miss or soft-delete.
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Tag, error)

	// GetByAppCode fetches by (application_id, code) — the natural identifier.
	// Used to enforce uniqueness on Create and to look up by external reference.
	GetByAppCode(ctx context.Context, q repository.Queryer, appID uuid.UUID, code string) (*Tag, error)

	// ListByApp returns every non-deleted tag for an application.
	ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Tag, error)

	// Create inserts a new tag. ErrConflict on duplicate (application_id, code).
	Create(ctx context.Context, q repository.Queryer, t *Tag) error

	// Update only the code (the only mutable field). ErrNotFound if missing.
	Update(ctx context.Context, q repository.Queryer, t *Tag) error

	// SoftDelete marks the row deleted. Junction rows in component_tags survive
	// the soft-delete but are filtered out at read time via the deleted_at check.
	SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error

	// GetComponentIDs returns the IDs of every non-deleted component that has
	// this tag attached. Drives `GET /tags/:id/components`.
	GetComponentIDs(ctx context.Context, q repository.Queryer, tagID uuid.UUID) ([]uuid.UUID, error)

	// AttachComponents adds the given component IDs to this tag via INSERT
	// ON CONFLICT DO NOTHING — idempotent and race-safe. Non-existent or
	// soft-deleted component IDs are silently skipped. Returns the number of
	// NEW rows inserted (excludes already-attached IDs) so callers can
	// audit-log only meaningful additions.
	AttachComponents(ctx context.Context, q repository.Queryer, tagID uuid.UUID, componentIDs []uuid.UUID) (int64, error)

	// DetachComponent removes a single component from this tag. ErrNotFound
	// if the junction row didn't exist.
	DetachComponent(ctx context.Context, q repository.Queryer, tagID, componentID uuid.UUID) error
}
