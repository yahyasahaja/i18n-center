package application

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/your-org/i18n-center/repository"
)

// ─── Queries ─────────────────────────────────────────────────────────────────

const (
	selectColumns = `id, name, code, description, openai_key, enabled_languages,
	                 created_by, updated_by, created_at, updated_at`

	queryGetByID = `
		SELECT id, name, code, description, openai_key, enabled_languages,
		       created_by, updated_by, created_at, updated_at
		FROM applications
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryGetByCode = `
		SELECT id, name, code, description, openai_key, enabled_languages,
		       created_by, updated_by, created_at, updated_at
		FROM applications
		WHERE code = $1
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
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $7, NOW(), NOW())
	`

	queryUpdate = `
		UPDATE applications
		SET name = $2,
		    code = $3,
		    description = $4,
		    openai_key = $5,
		    enabled_languages = $6,
		    updated_by = $7,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	querySoftDelete = `
		UPDATE applications
		SET deleted_at = NOW(),
		    updated_by = $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryUpdateEnabledLanguages = `
		UPDATE applications
		SET enabled_languages = $2,
		    updated_by = $3,
		    updated_at = NOW()
		WHERE id = $1
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
	langs := pq.StringArray(a.EnabledLanguages)
	if langs == nil {
		langs = pq.StringArray{}
	}
	_, err := q.ExecContext(ctx, queryInsert,
		a.ID, a.Name, a.Code, a.Description, a.OpenAIKey, langs, a.CreatedBy,
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
	langs := pq.StringArray(a.EnabledLanguages)
	if langs == nil {
		langs = pq.StringArray{}
	}
	result, err := q.ExecContext(ctx, queryUpdate,
		a.ID, a.Name, a.Code, a.Description, a.OpenAIKey, langs, a.UpdatedBy,
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
	result, err := q.ExecContext(ctx, querySoftDelete, id, userID)
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
	arr := pq.StringArray(langs)
	if arr == nil {
		arr = pq.StringArray{}
	}
	result, err := q.ExecContext(ctx, queryUpdateEnabledLanguages, id, arr, userID)
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
