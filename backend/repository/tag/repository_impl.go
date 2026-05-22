package tag

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
		FROM tags
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryGetByAppCode = `
		SELECT id, application_id, code, created_at, updated_at
		FROM tags
		WHERE application_id = $1
		  AND code = $2
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryListByApp = `
		SELECT id, application_id, code, created_at, updated_at
		FROM tags
		WHERE application_id = $1
		  AND deleted_at IS NULL
		ORDER BY code
	`

	queryInsert = `
		INSERT INTO tags (id, application_id, code, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE tags
		SET code = $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE tags
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	// Returns the IDs of non-deleted components that carry this tag. Joins
	// component_tags rather than touching the components table directly so
	// callers can decide what columns they want via a follow-up SELECT or
	// a Component repository lookup.
	queryGetComponentIDsByTag = `
		SELECT c.id
		FROM components c
		JOIN component_tags ct ON ct.component_id = c.id
		WHERE ct.tag_id = $1
		  AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC
	`
)

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Tag, error) {
	var t Tag
	if err := q.GetContext(ctx, &t, queryGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *Impl) GetByAppCode(ctx context.Context, q repository.Queryer, appID uuid.UUID, code string) (*Tag, error) {
	var t Tag
	if err := q.GetContext(ctx, &t, queryGetByAppCode, appID, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *Impl) ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Tag, error) {
	tags := []Tag{}
	if err := q.SelectContext(ctx, &tags, queryListByApp, appID); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *Impl) Create(ctx context.Context, q repository.Queryer, t *Tag) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if _, err := q.ExecContext(ctx, queryInsert, t.ID, t.ApplicationID, t.Code); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *Impl) Update(ctx context.Context, q repository.Queryer, t *Tag) error {
	result, err := q.ExecContext(ctx, queryUpdate, t.ID, t.Code)
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

func (r *Impl) GetComponentIDs(ctx context.Context, q repository.Queryer, tagID uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if err := q.SelectContext(ctx, &ids, queryGetComponentIDsByTag, tagID); err != nil {
		return nil, err
	}
	return ids, nil
}
