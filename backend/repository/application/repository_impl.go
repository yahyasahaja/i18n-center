package application

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

// ─── Queries ─────────────────────────────────────────────────────────────────

const (
	selectColumns = `id, name, code, description, openai_key, enabled_languages,
	                 created_by, updated_by, created_at, updated_at`

	queryGetByID = `
		SELECT id, name, code, description, openai_key, enabled_languages,
		       created_by, updated_by, created_at, updated_at
		FROM applications
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	queryGetByCode = `
		SELECT id, name, code, description, openai_key, enabled_languages,
		       created_by, updated_by, created_at, updated_at
		FROM applications
		WHERE code = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryList = `
		SELECT id, name, code, description, openai_key, enabled_languages,
		       created_by, updated_by, created_at, updated_at
		FROM applications
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`

	queryInsert = `
		INSERT INTO applications (
			id, name, code, description, openai_key, enabled_languages,
			created_by, updated_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE applications
		SET name = ?,
		    code = ?,
		    description = ?,
		    openai_key = ?,
		    enabled_languages = ?,
		    updated_by = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE applications
		SET deleted_at = NOW(),
		    updated_by = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	queryUpdateEnabledLanguages = `
		UPDATE applications
		SET enabled_languages = ?,
		    updated_by = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`
)

// Silence the unused-warning for the doc-only selectColumns constant — it's
// kept so future query authors can copy the canonical column list.
var _ = selectColumns

// ─── Implementation ──────────────────────────────────────────────────────────

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Application, error) {
	var a Application
	if err := q.GetContext(ctx, &a, queryGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *Impl) GetByCode(ctx context.Context, q repository.Queryer, code string) (*Application, error) {
	var a Application
	if err := q.GetContext(ctx, &a, queryGetByCode, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *Impl) List(ctx context.Context, q repository.Queryer) ([]Application, error) {
	apps := []Application{}
	if err := q.SelectContext(ctx, &apps, queryList); err != nil {
		return nil, err
	}
	return apps, nil
}

func (r *Impl) Create(ctx context.Context, q repository.Queryer, a *Application) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	langs := repository.JSONStringArray(a.EnabledLanguages)
	if langs == nil {
		langs = repository.JSONStringArray{}
	}
	_, err := q.ExecContext(ctx, queryInsert,
		a.ID, a.Name, a.Code, a.Description, a.OpenAIKey, langs, a.CreatedBy, a.CreatedBy,
	)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	a.UpdatedBy = a.CreatedBy
	return nil
}

func (r *Impl) Update(ctx context.Context, q repository.Queryer, a *Application) error {
	langs := repository.JSONStringArray(a.EnabledLanguages)
	if langs == nil {
		langs = repository.JSONStringArray{}
	}
	result, err := q.ExecContext(ctx, queryUpdate,
		a.Name, a.Code, a.Description, a.OpenAIKey, langs, a.UpdatedBy, a.ID,
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
	a.UpdatedAt = time.Now()
	return nil
}

func (r *Impl) SoftDelete(ctx context.Context, q repository.Queryer, id, userID uuid.UUID) error {
	result, err := q.ExecContext(ctx, querySoftDelete, userID, id)
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

func (r *Impl) UpdateEnabledLanguages(ctx context.Context, q repository.Queryer, id uuid.UUID, langs []string, userID uuid.UUID) error {
	arr := repository.JSONStringArray(langs)
	if arr == nil {
		arr = repository.JSONStringArray{}
	}
	result, err := q.ExecContext(ctx, queryUpdateEnabledLanguages, arr, userID, id)
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
