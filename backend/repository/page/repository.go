// Package page is the data access layer for `pages` — groupings that bundle
// related components into a single read endpoint (`GET /by-page/:code`).
// Identified by `code` (unique per application). Component↔Page is
// many-to-many via the `component_pages` table.
//
// The schema is structurally identical to `tags`; we keep them as separate
// packages because their domain meanings diverge (a Tag is a label, a Page is
// a content grouping for FE consumers) and so they can evolve independently.
package page

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

// Page is the in-memory representation of a row from `pages`.
type Page struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	Code          string    `db:"code"           json:"code"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
}

type Repository interface {
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Page, error)
	// GetByAppCode is the canonical "lookup by external identifier" path.
	// Powers `GET /applications/:id/translations/by-page/:pageCode`.
	GetByAppCode(ctx context.Context, q repository.Queryer, appID uuid.UUID, code string) (*Page, error)
	ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Page, error)
	Create(ctx context.Context, q repository.Queryer, p *Page) error
	Update(ctx context.Context, q repository.Queryer, p *Page) error
	SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error
	// GetComponentIDs returns the IDs of every non-deleted component grouped
	// under this page. Drives translation aggregation in `by-page` reads.
	GetComponentIDs(ctx context.Context, q repository.Queryer, pageID uuid.UUID) ([]uuid.UUID, error)
}
