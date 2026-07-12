package component

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/page"
	"github.com/lapakgaming/i18n-center/repository/tag"
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
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	queryGetByCode = `
		SELECT id, application_id, name, code, description,
		       key_contexts, default_locale, created_by, updated_by,
		       created_at, updated_at
		FROM components
		WHERE code = ?
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
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE components
		SET name = ?,
		    code = ?,
		    description = ?,
		    key_contexts = ?,
		    default_locale = ?,
		    updated_by = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE components
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	// Junction table maintenance — DELETE the full set, then bulk-INSERT the
	// new IDs. The DELETE uses the junction's primary key index; the INSERT is
	// a single round-trip that filters out soft-deleted parents via a SELECT
	// against tags/pages and uses INSERT IGNORE for duplicate-PK skip.
	queryDetachAllTags  = `DELETE FROM component_tags  WHERE component_id = ?`
	queryDetachAllPages = `DELETE FROM component_pages WHERE component_id = ?`

	queryAttachTagsBulk = `
		INSERT IGNORE INTO component_tags (component_id, tag_id)
		SELECT ?, t.id
		FROM tags t
		WHERE t.id IN (?)
		  AND t.deleted_at IS NULL
	`

	queryAttachPagesBulk = `
		INSERT IGNORE INTO component_pages (component_id, page_id)
		SELECT ?, p.id
		FROM pages p
		WHERE p.id IN (?)
		  AND p.deleted_at IS NULL
	`

	queryLoadTags = `
		SELECT t.id, t.application_id, t.code, t.created_at, t.updated_at
		FROM tags t
		JOIN component_tags ct ON ct.tag_id = t.id
		WHERE ct.component_id = ?
		  AND t.deleted_at IS NULL
		ORDER BY t.code
	`

	queryLoadPages = `
		SELECT p.id, p.application_id, p.code, p.created_at, p.updated_at
		FROM pages p
		JOIN component_pages cp ON cp.page_id = p.id
		WHERE cp.component_id = ?
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

// queryGetByAppCode hits the (application_id, code) partial unique index head
// directly. Single-row by definition — no LIMIT needed but kept for clarity.
const queryGetByAppCode = `
	SELECT id, application_id, name, code, description,
	       key_contexts, default_locale, created_by, updated_by,
	       created_at, updated_at
	FROM components
	WHERE application_id = ? AND code = ?
	  AND deleted_at IS NULL
	LIMIT 1
`

func (r *Impl) GetByAppCode(ctx context.Context, q repository.Queryer, appID uuid.UUID, code string) (*Component, error) {
	var c Component
	if err := q.GetContext(ctx, &c, queryGetByAppCode, appID, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// ListByIDs returns the components whose IDs are in the provided set. Order
// of results is unspecified — callers wanting a specific order should sort
// the returned slice themselves. Empty input → empty result, no query.
func (r *Impl) ListByIDs(ctx context.Context, q repository.Queryer, ids []uuid.UUID) ([]Component, error) {
	if len(ids) == 0 {
		return []Component{}, nil
	}
	const base = `
		SELECT id, application_id, name, code, description,
		       key_contexts, default_locale, created_by, updated_by,
		       created_at, updated_at
		FROM components
		WHERE id IN (?) AND deleted_at IS NULL
	`
	query, args, err := sqlx.In(base, ids)
	if err != nil {
		return nil, err
	}
	rows := []Component{}
	if err := q.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Impl) List(ctx context.Context, q repository.Queryer, f ListFilter) ([]Component, int, error) {
	// Build the WHERE additions dynamically. Static prefix comes from the const
	// so the query plan is stable across most call patterns; the conditional
	// suffix is appended below with positional `?` placeholders.
	sb := strings.Builder{}
	cb := strings.Builder{}
	sb.WriteString(queryListBase)
	cb.WriteString(queryCountBase)
	args := []any{}
	if f.ApplicationID != uuid.Nil {
		sb.WriteString(" AND application_id = ?")
		cb.WriteString(" AND application_id = ?")
		args = append(args, f.ApplicationID)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		// Case-insensitivity comes from the utf8mb4_unicode_ci collation on the
		// name/code columns — LIKE is case-insensitive by default. At ~200 rows
		// per app the full-scan cost is fine; no trigram index needed.
		sb.WriteString(" AND (name LIKE ? OR code LIKE ?)")
		cb.WriteString(" AND (name LIKE ? OR code LIKE ?)")
		like := "%" + s + "%"
		args = append(args, like, like)
	}
	// Count first so a slow LIMIT/OFFSET doesn't poison the count read.
	countArgs := append([]any(nil), args...)
	var total int
	if err := q.GetContext(ctx, &total, cb.String(), countArgs...); err != nil {
		return nil, 0, err
	}

	// Ordering and pagination on the row read. Limit=0 means "no limit" — used
	// by callers that need the full set (e.g. the AddLanguage worker fan-out).
	// MySQL requires LIMIT alongside OFFSET, so the unbounded branch uses the
	// max-uint64 sentinel documented in the MySQL manual.
	sb.WriteString(" ORDER BY created_at DESC")
	if f.Limit > 0 {
		sb.WriteString(" LIMIT ? OFFSET ?")
		args = append(args, f.Limit, f.Offset)
	} else if f.Offset > 0 {
		sb.WriteString(" LIMIT 18446744073709551615 OFFSET ?")
		args = append(args, f.Offset)
	}

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
	// created_by and updated_by are seeded to the same actor on insert; the
	// old $8/$8 reuse becomes two positional args in MySQL.
	if _, err := q.ExecContext(ctx, queryInsert,
		c.ID, c.ApplicationID, c.Name, c.Code, c.Description,
		c.KeyContexts, c.DefaultLocale, c.CreatedBy, c.CreatedBy,
	); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *Impl) Update(ctx context.Context, q repository.Queryer, c *Component) error {
	// MySQL is positional — SET args come first, WHERE id last.
	result, err := q.ExecContext(ctx, queryUpdate,
		c.Name, c.Code, c.Description, c.KeyContexts, c.DefaultLocale, c.UpdatedBy, c.ID,
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
	// sqlx.In expands the `IN (?)` placeholder to `IN (?,?,?…)` and interleaves
	// the args in order — componentID first, then the tag ID slice.
	query, args, err := sqlx.In(queryAttachTagsBulk, componentID, tagIDs)
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, query, args...); err != nil {
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
	query, args, err := sqlx.In(queryAttachPagesBulk, componentID, pageIDs)
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, query, args...); err != nil {
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
