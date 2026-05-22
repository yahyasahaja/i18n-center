package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/page"
	"github.com/your-org/i18n-center/repository/tag"
)

// ─── Queries ─────────────────────────────────────────────────────────────────

const (
	// selectColumnsList is the canonical projection. Kept as a string constant
	// so all the SELECT queries below share one column order — the sqlx scan
	// into Component is positional.
	selectColumnsList = `id, application_id, name, code, description,
		key_contexts, default_locale, created_by, updated_by, created_at, updated_at`

	queryGetByID = `
		SELECT id, application_id, name, code, description,
		       key_contexts, default_locale, created_by, updated_by,
		       created_at, updated_at
		FROM components
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryGetByCode = `
		SELECT id, application_id, name, code, description,
		       key_contexts, default_locale, created_by, updated_by,
		       created_at, updated_at
		FROM components
		WHERE code = $1
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryListBase = `
		SELECT id, application_id, name, code, description,
		       key_contexts, default_locale, created_by, updated_by,
		       created_at, updated_at
		FROM components
		WHERE deleted_at IS NULL
	`

	queryCountBase = `
		SELECT COUNT(*) FROM components WHERE deleted_at IS NULL
	`

	queryInsert = `
		INSERT INTO components (
			id, application_id, name, code, description,
			key_contexts, default_locale, created_by, updated_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE components
		SET name = $2,
		    code = $3,
		    description = $4,
		    key_contexts = $5,
		    default_locale = $6,
		    updated_by = $7,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE components
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	// Junction table maintenance — DELETE the full set, then bulk-INSERT the
	// new IDs. The DELETE uses the junction's primary key index; the INSERT is
	// a single round-trip via Postgres array-unnest.
	queryDetachAllTags  = `DELETE FROM component_tags  WHERE component_id = $1`
	queryDetachAllPages = `DELETE FROM component_pages WHERE component_id = $1`

	queryAttachTagsBulk = `
		INSERT INTO component_tags (component_id, tag_id)
		SELECT $1, t.id
		FROM tags t
		WHERE t.id = ANY($2::uuid[])
		  AND t.deleted_at IS NULL
		ON CONFLICT DO NOTHING
	`

	queryAttachPagesBulk = `
		INSERT INTO component_pages (component_id, page_id)
		SELECT $1, p.id
		FROM pages p
		WHERE p.id = ANY($2::uuid[])
		  AND p.deleted_at IS NULL
		ON CONFLICT DO NOTHING
	`

	queryLoadTags = `
		SELECT t.id, t.application_id, t.code, t.created_at, t.updated_at
		FROM tags t
		JOIN component_tags ct ON ct.tag_id = t.id
		WHERE ct.component_id = $1
		  AND t.deleted_at IS NULL
		ORDER BY t.code
	`

	queryLoadPages = `
		SELECT p.id, p.application_id, p.code, p.created_at, p.updated_at
		FROM pages p
		JOIN component_pages cp ON cp.page_id = p.id
		WHERE cp.component_id = $1
		  AND p.deleted_at IS NULL
		ORDER BY p.code
	`
)

// Silence unused-warning — selectColumnsList is documentation-only.
var _ = selectColumnsList

// ─── Implementation ──────────────────────────────────────────────────────────

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Component, error) {
	var c Component
	if err := q.GetContext(ctx, &c, queryGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *Impl) GetByIDWithRelations(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Component, error) {
	c, err := r.GetByID(ctx, q, id)
	if err != nil {
		return nil, err
	}
	tags, err := r.LoadTags(ctx, q, c.ID)
	if err != nil {
		return nil, err
	}
	pages, err := r.LoadPages(ctx, q, c.ID)
	if err != nil {
		return nil, err
	}
	c.Tags = tags
	c.Pages = pages
	return c, nil
}

func (r *Impl) GetByCode(ctx context.Context, q repository.Queryer, code string) (*Component, error) {
	var c Component
	if err := q.GetContext(ctx, &c, queryGetByCode, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *Impl) List(ctx context.Context, q repository.Queryer, f ListFilter) ([]Component, int, error) {
	// Build the WHERE additions dynamically. Static prefix comes from the const
	// so the query plan is stable across most call patterns; the conditional
	// suffix is appended below with numbered placeholders.
	sb := strings.Builder{}
	cb := strings.Builder{}
	sb.WriteString(queryListBase)
	cb.WriteString(queryCountBase)
	args := []any{}
	i := 1
	if f.ApplicationID != uuid.Nil {
		fmt.Fprintf(&sb, " AND application_id = $%d", i)
		fmt.Fprintf(&cb, " AND application_id = $%d", i)
		args = append(args, f.ApplicationID)
		i++
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		// pg_trgm makes ILIKE cheap; trigram GIN index on name + code.
		fmt.Fprintf(&sb, " AND (name ILIKE $%d OR code ILIKE $%d)", i, i+1)
		fmt.Fprintf(&cb, " AND (name ILIKE $%d OR code ILIKE $%d)", i, i+1)
		like := "%" + s + "%"
		args = append(args, like, like)
		i += 2
	}
	// Count first so a slow LIMIT/OFFSET doesn't poison the count read.
	countArgs := append([]any(nil), args...)
	var total int
	if err := q.GetContext(ctx, &total, cb.String(), countArgs...); err != nil {
		return nil, 0, err
	}

	// Ordering and pagination on the row read.
	fmt.Fprintf(&sb, " ORDER BY created_at DESC LIMIT $%d OFFSET $%d", i, i+1)
	args = append(args, f.Limit, f.Offset)

	rows := []Component{}
	if err := q.SelectContext(ctx, &rows, sb.String(), args...); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Impl) Create(ctx context.Context, q repository.Queryer, c *Component) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if _, err := q.ExecContext(ctx, queryInsert,
		c.ID, c.ApplicationID, c.Name, c.Code, c.Description,
		c.KeyContexts, c.DefaultLocale, c.CreatedBy,
	); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *Impl) Update(ctx context.Context, q repository.Queryer, c *Component) error {
	result, err := q.ExecContext(ctx, queryUpdate,
		c.ID, c.Name, c.Code, c.Description, c.KeyContexts, c.DefaultLocale, c.UpdatedBy,
	)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Impl) SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error {
	result, err := q.ExecContext(ctx, querySoftDelete, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// AttachTags replaces ALL existing component_tags rows with the provided set.
// Empty slice == clear all attachments. Best called inside a transaction so
// the delete and insert are atomic from a reader's perspective.
func (r *Impl) AttachTags(ctx context.Context, q repository.Queryer, componentID uuid.UUID, tagIDs []uuid.UUID) error {
	if _, err := q.ExecContext(ctx, queryDetachAllTags, componentID); err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	// pq.Array handles the []uuid.UUID → uuid[] conversion for Postgres.
	if _, err := q.ExecContext(ctx, queryAttachTagsBulk, componentID, pq.Array(uuidToStringSlice(tagIDs))); err != nil {
		return err
	}
	return nil
}

func (r *Impl) AttachPages(ctx context.Context, q repository.Queryer, componentID uuid.UUID, pageIDs []uuid.UUID) error {
	if _, err := q.ExecContext(ctx, queryDetachAllPages, componentID); err != nil {
		return err
	}
	if len(pageIDs) == 0 {
		return nil
	}
	if _, err := q.ExecContext(ctx, queryAttachPagesBulk, componentID, pq.Array(uuidToStringSlice(pageIDs))); err != nil {
		return err
	}
	return nil
}

func (r *Impl) LoadTags(ctx context.Context, q repository.Queryer, componentID uuid.UUID) ([]tag.Tag, error) {
	out := []tag.Tag{}
	if err := q.SelectContext(ctx, &out, queryLoadTags, componentID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Impl) LoadPages(ctx context.Context, q repository.Queryer, componentID uuid.UUID) ([]page.Page, error) {
	out := []page.Page{}
	if err := q.SelectContext(ctx, &out, queryLoadPages, componentID); err != nil {
		return nil, err
	}
	return out, nil
}

// uuidToStringSlice converts a []uuid.UUID to []string for pq.Array (which
// needs a slice of types it can serialise as a Postgres array element).
// Going through string is the cheapest broadly-supported path.
func uuidToStringSlice(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// Unused import suppressor — sqlx is imported indirectly via repository.Queryer
// but the file compiles cleanly without an explicit reference. Keeping for
// future use when this file grows additional sqlx-specific helpers.
var _ = sqlx.In
