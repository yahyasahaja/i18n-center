package page

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lapakgaming/i18n-center/repository"
)

const (
	queryGetByID = `
		SELECT id, application_id, code, created_at, updated_at
		FROM pages
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	queryGetByAppCode = `
		SELECT id, application_id, code, created_at, updated_at
		FROM pages
		WHERE application_id = ?
		  AND code = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryListByApp = `
		SELECT id, application_id, code, created_at, updated_at
		FROM pages
		WHERE application_id = ?
		  AND deleted_at IS NULL
		ORDER BY code
	`

	queryInsert = `
		INSERT INTO pages (id, application_id, code, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE pages
		SET code = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE pages
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	queryGetComponentIDsByPage = `
		SELECT c.id
		FROM components c
		JOIN component_pages cp ON cp.component_id = c.id
		WHERE cp.page_id = ?
		  AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC
	`

	// queryAttachComponentsBulk inserts (page_id, component_id) rows using a
	// SELECT-driven expansion so the same query handles any number of IDs
	// in one round trip. The JOIN to `components` filters out IDs that don't
	// exist or are soft-deleted — protecting the junction from dangling rows.
	// INSERT IGNORE makes the operation idempotent at the composite primary
	// key (component_id, page_id). The `IN (?)` placeholder is expanded by
	// sqlx.In at call time.
	queryAttachComponentsToPage = `
		INSERT IGNORE INTO component_pages (component_id, page_id)
		SELECT c.id, ?
		FROM components c
		WHERE c.id IN (?)
		  AND c.deleted_at IS NULL
	`

	queryDetachComponentFromPage = `
		DELETE FROM component_pages
		WHERE page_id = ?
		  AND component_id = ?
	`
)

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Page, error) {
	var p Page
	if err := q.GetContext(ctx, &p, queryGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Impl) GetByAppCode(ctx context.Context, q repository.Queryer, appID uuid.UUID, code string) (*Page, error) {
	var p Page
	if err := q.GetContext(ctx, &p, queryGetByAppCode, appID, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Impl) ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Page, error) {
	pages := []Page{}
	if err := q.SelectContext(ctx, &pages, queryListByApp, appID); err != nil {
		return nil, err
	}
	return pages, nil
}

func (r *Impl) Create(ctx context.Context, q repository.Queryer, p *Page) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if _, err := q.ExecContext(ctx, queryInsert, p.ID, p.ApplicationID, p.Code); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *Impl) Update(ctx context.Context, q repository.Queryer, p *Page) error {
	result, err := q.ExecContext(ctx, queryUpdate, p.ID, p.Code)
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

func (r *Impl) GetComponentIDs(ctx context.Context, q repository.Queryer, pageID uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if err := q.SelectContext(ctx, &ids, queryGetComponentIDsByPage, pageID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *Impl) AttachComponents(ctx context.Context, q repository.Queryer, pageID uuid.UUID, componentIDs []uuid.UUID) (int64, error) {
	if len(componentIDs) == 0 {
		return 0, nil
	}
	// sqlx.In expands the trailing `IN (?)` into the right number of
	// placeholders and flattens the args in the correct order.
	query, args, err := sqlx.In(queryAttachComponentsToPage, pageID, componentIDs)
	if err != nil {
		return 0, err
	}
	result, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Impl) DetachComponent(ctx context.Context, q repository.Queryer, pageID, componentID uuid.UUID) error {
	result, err := q.ExecContext(ctx, queryDetachComponentFromPage, pageID, componentID)
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
