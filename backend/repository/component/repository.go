// Package component is the data access layer for `components` — UI units that
// own translation key sets. A component is scoped to one application (unique
// per application via the partial index idx_component_app_code on
// `application_id, code`).
//
// Components have many-to-many relationships with tags and pages, materialised
// in `component_tags` and `component_pages` junction tables. Bulk attach/detach
// goes through the AttachTags/AttachPages methods here.
package component

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/page"
	"github.com/your-org/i18n-center/repository/tag"
)

// Component is the in-memory representation of a row from `components`.
//
// Tags / Pages are populated by the WithTagsAndPages variant; the bare load
// methods leave them nil so callers that don't need the relations avoid two
// extra round-trips per row.
type Component struct {
	ID            uuid.UUID         `db:"id"             json:"id"`
	ApplicationID uuid.UUID         `db:"application_id" json:"application_id"`
	Name          string            `db:"name"           json:"name"`
	Code          string            `db:"code"           json:"code"`
	Description   string            `db:"description"    json:"description"`
	// KeyContexts: optional flat {dot.path: hint} for AI translation context.
	// Nullable in the DB; nil here when no contexts are authored.
	KeyContexts   repository.JSONB  `db:"key_contexts"   json:"key_contexts"`
	DefaultLocale string            `db:"default_locale" json:"default_locale"`
	CreatedBy     uuid.UUID         `db:"created_by"     json:"created_by"`
	UpdatedBy     uuid.UUID         `db:"updated_by"     json:"updated_by"`
	CreatedAt     time.Time         `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time         `db:"updated_at"     json:"updated_at"`

	// Populated only by GetByIDWithRelations / ListByAppWithRelations.
	Tags  []tag.Tag   `db:"-" json:"tags,omitempty"`
	Pages []page.Page `db:"-" json:"pages,omitempty"`
}

// ListFilter shapes the WHERE clause for paginated listing.
type ListFilter struct {
	// ApplicationID, when non-Nil, scopes the list to one application.
	ApplicationID uuid.UUID
	// Search, when non-empty, matches against `name` or `code` via ILIKE.
	// pg_trgm GIN indexes (idx_components_name_trgm / idx_components_code_trgm)
	// make this scan cheap even at scale.
	Search string
	// Limit / Offset — caller-provided pagination. The handler enforces a max
	// limit; this layer trusts whatever it's given.
	Limit  int
	Offset int
}

type Repository interface {
	// GetByID returns the bare component (no tags/pages). ErrNotFound on miss.
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Component, error)

	// GetByIDWithRelations is GetByID + one round-trip each to populate Tags and Pages.
	GetByIDWithRelations(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Component, error)

	// GetByCode looks up a component by its code alone. NOT application-scoped:
	// codes are unique per application (idx_component_app_code), so the same
	// code can exist across multiple apps and this method returns whichever
	// Postgres picks. Prefer GetByAppCode when you know the application ID;
	// reserve GetByCode for the legacy "ambiguous global lookup" path used by
	// the public component handler.
	GetByCode(ctx context.Context, q repository.Queryer, code string) (*Component, error)

	// GetByAppCode looks up a component by (applicationID, code). This is the
	// correct lookup for any flow that already knows which application it's
	// operating in — bootstrap, in-app component management, etc. Returns
	// ErrNotFound when no row matches; never returns a component owned by a
	// different application.
	GetByAppCode(ctx context.Context, q repository.Queryer, appID uuid.UUID, code string) (*Component, error)

	// ListByIDs returns the components whose IDs are in the provided set
	// (no tags/pages preloaded). Used by the by-tag / by-page handlers that
	// resolve a code→data map for a list of component IDs.
	ListByIDs(ctx context.Context, q repository.Queryer, ids []uuid.UUID) ([]Component, error)

	// List returns one page of components matching the filter, plus the total
	// count for pagination. The two queries run sequentially against the
	// same Queryer; callers can wrap in WithTx if they need a snapshot view.
	List(ctx context.Context, q repository.Queryer, f ListFilter) (rows []Component, total int, err error)

	// Create inserts a new component. ErrConflict on duplicate (app, code).
	// Does NOT attach tags or pages — call AttachTags/AttachPages separately
	// inside the same transaction.
	Create(ctx context.Context, q repository.Queryer, c *Component) error

	// Update overwrites mutable fields (name, code, description, key_contexts,
	// default_locale, updated_by). ErrNotFound on miss.
	Update(ctx context.Context, q repository.Queryer, c *Component) error

	// SoftDelete marks the component deleted. Junction rows in component_tags
	// / component_pages survive but are filtered out at read time.
	SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error

	// AttachTags / AttachPages REPLACE the existing junction rows with the
	// provided ID set, atomically. Passing an empty slice clears all attachments.
	// Best run inside a transaction so the delete + insert is observable as
	// a single state change.
	AttachTags(ctx context.Context, q repository.Queryer, componentID uuid.UUID, tagIDs []uuid.UUID) error
	AttachPages(ctx context.Context, q repository.Queryer, componentID uuid.UUID, pageIDs []uuid.UUID) error

	// LoadTags / LoadPages populate the join rows for a single component.
	// Mostly used inline by GetByIDWithRelations; exposed for callers that
	// already have a Component in hand.
	LoadTags(ctx context.Context, q repository.Queryer, componentID uuid.UUID) ([]tag.Tag, error)
	LoadPages(ctx context.Context, q repository.Queryer, componentID uuid.UUID) ([]page.Page, error)
}
