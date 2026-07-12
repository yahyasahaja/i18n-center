package tag

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
		FROM tags
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	queryGetByAppCode = `
		SELECT id, application_id, code, created_at, updated_at
		FROM tags
		WHERE application_id = ?
		  AND code = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryListByApp = `
		SELECT id, application_id, code, created_at, updated_at
		FROM tags
		WHERE application_id = ?
		  AND deleted_at IS NULL
		ORDER BY code
	`

	queryInsert = `
		INSERT INTO tags (id, application_id, code, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE tags
		SET code = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE tags
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = ?
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
		WHERE ct.tag_id = ?
		  AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC
	`

	// queryAttachComponentsToTag inserts (tag_id, component_id) rows from a
	// SELECT-driven expansion so the same query handles any number of IDs
	// in one round trip. JOIN to `components` filters out IDs that don't
	// exist or are soft-deleted. INSERT IGNORE keeps the operation
	// idempotent at the composite primary key. The `IN (?)` placeholder is
	// expanded by sqlx.In at call time.
	queryAttachComponentsToTag = `
		INSERT IGNORE INTO component_tags (component_id, tag_id)
		SELECT c.id, ?
		FROM components c
		WHERE c.id IN (?)
		  AND c.deleted_at IS NULL
	`

	queryDetachComponentFromTag = `
		DELETE FROM component_tags
		WHERE tag_id = ?
		  AND component_id = ?
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

func (r *Impl) AttachComponents(ctx context.Context, q repository.Queryer, tagID uuid.UUID, componentIDs []uuid.UUID) (int64, error) {
	if len(componentIDs) == 0 {
		return 0, nil
	}
	// sqlx.In expands the trailing `IN (?)` into the right number of
	// placeholders and flattens the args in the correct order.
	query, args, err := sqlx.In(queryAttachComponentsToTag, tagID, componentIDs)
	if err != nil {
		return 0, err
	}
	result, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Impl) DetachComponent(ctx context.Context, q repository.Queryer, tagID, componentID uuid.UUID) error {
	result, err := q.ExecContext(ctx, queryDetachComponentFromTag, tagID, componentID)
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
