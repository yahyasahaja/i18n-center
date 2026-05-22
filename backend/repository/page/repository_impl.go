package page

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

const (
	queryGetByID = `
		SELECT id, application_id, code, created_at, updated_at
		FROM pages
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryGetByAppCode = `
		SELECT id, application_id, code, created_at, updated_at
		FROM pages
		WHERE application_id = $1
		  AND code = $2
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryListByApp = `
		SELECT id, application_id, code, created_at, updated_at
		FROM pages
		WHERE application_id = $1
		  AND deleted_at IS NULL
		ORDER BY code
	`

	queryInsert = `
		INSERT INTO pages (id, application_id, code, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE pages
		SET code = $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE pages
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryGetComponentIDsByPage = `
		SELECT c.id
		FROM components c
		JOIN component_pages cp ON cp.component_id = c.id
		WHERE cp.page_id = $1
		  AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC
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
