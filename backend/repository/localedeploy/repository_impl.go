package localedeploy

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

const (
	queryGetByAppLocale = `
		SELECT id, application_id, locale, stage_completed, created_at, updated_at
		FROM application_locale_deploys
		WHERE application_id = $1 AND locale = $2
	`

	queryListPendingByApp = `
		SELECT id, application_id, locale, stage_completed, created_at, updated_at
		FROM application_locale_deploys
		WHERE application_id = $1 AND stage_completed != $2
		ORDER BY locale ASC
	`

	// Upsert can't use ON CONFLICT directly: the (application_id, locale)
	// uniqueness is enforced by a partial unique index
	// (idx_app_locale ... WHERE deleted_at IS NULL), and Postgres only
	// matches ON CONFLICT against UNIQUE constraints or non-partial unique
	// indexes. We do the update-then-insert dance instead — same effect,
	// works against the partial index.
	queryUpdateByAppLocale = `
		UPDATE application_locale_deploys
		SET stage_completed = $3, updated_at = NOW()
		WHERE application_id = $1 AND locale = $2 AND deleted_at IS NULL
		RETURNING id, created_at, updated_at
	`

	queryInsertNew = `
		INSERT INTO application_locale_deploys (
			id, application_id, locale, stage_completed, created_at, updated_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	querySetStage = `
		UPDATE application_locale_deploys
		SET stage_completed = $3, updated_at = NOW()
		WHERE application_id = $1 AND locale = $2
	`

	queryDelete = `
		DELETE FROM application_locale_deploys
		WHERE application_id = $1 AND locale = $2
	`
)

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetByAppLocale(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale string) (*Deploy, error) {
	var d Deploy
	if err := q.GetContext(ctx, &d, queryGetByAppLocale, appID, locale); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *Impl) ListPendingByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Deploy, error) {
	out := []Deploy{}
	if err := q.SelectContext(ctx, &out, queryListPendingByApp, appID, StageProduction); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Impl) Upsert(ctx context.Context, q repository.Queryer, d *Deploy) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	row := struct {
		ID        uuid.UUID `db:"id"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}
	// Update-then-insert. The update RETURNING swallows the lookup-then-update
	// case in one round trip; sql.ErrNoRows tells us we need to insert.
	err := q.GetContext(ctx, &row, queryUpdateByAppLocale, d.ApplicationID, d.Locale, d.StageCompleted)
	if err == nil {
		d.ID = row.ID
		d.CreatedAt = row.CreatedAt
		d.UpdatedAt = row.UpdatedAt
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// No active row — insert a fresh one. The partial unique index might race
	// us if two pods upsert concurrently for a brand-new (app, locale); rare
	// in practice (the AddLanguage handler gates it), and the second writer
	// will see a unique violation it can surface as repository.ErrConflict.
	if err := q.GetContext(ctx, &row, queryInsertNew, d.ID, d.ApplicationID, d.Locale, d.StageCompleted); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	d.ID = row.ID
	d.CreatedAt = row.CreatedAt
	d.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *Impl) SetStage(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale, stage string) error {
	_, err := q.ExecContext(ctx, querySetStage, appID, locale, stage)
	return err
}

func (r *Impl) Delete(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale string) error {
	_, err := q.ExecContext(ctx, queryDelete, appID, locale)
	return err
}
