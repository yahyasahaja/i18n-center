// Package application is the data access layer for the `applications` table —
// the top-level container that owns components, tags, pages, API keys, CMS
// items, and translation versions.
//
// Two lookup paths are supported: by UUID (internal) and by code (used by
// SDK calls and external integrations). The code is unique among non-deleted
// rows via the partial unique index idx_applications_code, so a code freed by
// a soft-delete can be reused.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/your-org/i18n-center/repository"
)

// Application matches the `applications` row layout. enabled_languages is a
// Postgres text[]; we use lib/pq's StringArray for round-tripping.
//
// HasOpenAIKey is NOT a column — it's a computed flag derived from whether
// OpenAIKey is non-empty. Stored as `db:"-"` so sqlx ignores it on scan; the
// handler/service sets it before returning the value to clients.
type Application struct {
	ID               uuid.UUID      `db:"id"                json:"id"`
	Name             string         `db:"name"              json:"name"`
	Code             string         `db:"code"              json:"code"`
	Description      string         `db:"description"       json:"description"`
	OpenAIKey        string         `db:"openai_key"        json:"-"`
	HasOpenAIKey     bool           `db:"-"                 json:"has_openai_key"`
	EnabledLanguages pq.StringArray `db:"enabled_languages" json:"enabled_languages"`
	CreatedBy        uuid.UUID      `db:"created_by"        json:"created_by"`
	UpdatedBy        uuid.UUID      `db:"updated_by"        json:"updated_by"`
	CreatedAt        time.Time      `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"        json:"updated_at"`
}

// PopulateComputed sets HasOpenAIKey from OpenAIKey. Call after fetching a row
// (or in a wrapping service) when the result is about to be serialised to a client.
func (a *Application) PopulateComputed() {
	a.HasOpenAIKey = a.OpenAIKey != ""
}

// Repository is the contract for application persistence.
type Repository interface {
	// GetByID fetches by UUID. ErrNotFound when missing or soft-deleted.
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Application, error)

	// GetByCode fetches by the unique code (e.g. "joytify", "lapakgaming-fe").
	// ErrNotFound when missing.
	GetByCode(ctx context.Context, q repository.Queryer, code string) (*Application, error)

	// List returns every non-deleted application, newest first.
	List(ctx context.Context, q repository.Queryer) ([]Application, error)

	// Create inserts a new application. ErrConflict on duplicate code.
	Create(ctx context.Context, q repository.Queryer, a *Application) error

	// Update overwrites mutable fields (name, code, description, openai_key,
	// enabled_languages, updated_by). ErrNotFound when missing.
	Update(ctx context.Context, q repository.Queryer, a *Application) error

	// SoftDelete marks the application deleted. Caller is responsible for
	// downstream cleanup (cascading or cascading-by-policy is the calling
	// usecase's job, not this layer's).
	SoftDelete(ctx context.Context, q repository.Queryer, id, userID uuid.UUID) error

	// UpdateEnabledLanguages replaces the enabled_languages array. Separate
	// method (rather than overloading Update) because the add-language /
	// remove-language flows touch only this column.
	UpdateEnabledLanguages(ctx context.Context, q repository.Queryer, id uuid.UUID, langs []string, userID uuid.UUID) error
}
