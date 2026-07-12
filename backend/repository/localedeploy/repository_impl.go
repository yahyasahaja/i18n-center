package localedeploy

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

const (
	queryGetByAppLocale = `
		SELECT id, application_id, locale, stage_completed, created_at, updated_at
		FROM application_locale_deploys
		WHERE application_id = ? AND locale = ?
	`

	queryListPendingByApp = `
		SELECT id, application_id, locale, stage_completed, created_at, updated_at
		FROM application_locale_deploys
		WHERE application_id = ? AND stage_completed != ?
		ORDER BY locale ASC
	`

	// Upsert can't use ON DUPLICATE KEY UPDATE directly: the
	// (application_id, locale) uniqueness is enforced by a partial unique
	// index (idx_app_locale ... WHERE deleted_at IS NULL — expressed on
	// MySQL as a generated-column composite index that's NULL when
	// soft-deleted), and MySQL's INSERT ... ON DUPLICATE KEY UPDATE can't
	// target the non-deleted subset either. We do the update-then-insert
	// dance instead — same effect, works against the partial index.
	queryUpdateByAppLocale = `
		UPDATE application_locale_deploys
		SET stage_completed = ?, updated_at = NOW()
		WHERE application_id = ? AND locale = ? AND deleted_at IS NULL
	`

	// SELECT-back after the UPDATE succeeds to hydrate id/created_at/updated_at.
	querySelectActiveByAppLocale = `
		SELECT id, created_at, updated_at
		FROM application_locale_deploys
		WHERE application_id = ? AND locale = ? AND deleted_at IS NULL
	`

	queryInsertNew = `
		INSERT INTO application_locale_deploys (
			id, application_id, locale, stage_completed, created_at, updated_at
		) VALUES (?, ?, ?, ?, NOW(), NOW())
	`

	// SELECT-back after the INSERT to hydrate created_at/updated_at (id is
	// generated client-side so it's already known to the caller).
	querySelectByID = `
		SELECT id, created_at, updated_at
		FROM application_locale_deploys
		WHERE id = ?
	`

	querySetStage = `
		UPDATE application_locale_deploys
		SET stage_completed = ?, updated_at = NOW()
		WHERE application_id = ? AND locale = ?
	`

	queryDelete = `
		DELETE FROM application_locale_deploys
		WHERE application_id = ? AND locale = ?
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

	// Update-then-insert. Try the UPDATE first; RowsAffected==0 tells us
	// there's no active row and we need to fall through to INSERT.
	result, err := q.ExecContext(ctx, queryUpdateByAppLocale, d.StageCompleted, d.ApplicationID, d.Locale)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		// Row updated — SELECT it back to hydrate id/created_at/updated_at.
		if err := q.GetContext(ctx, &row, querySelectActiveByAppLocale, d.ApplicationID, d.Locale); err != nil {
			return err
		}
		d.ID = row.ID
		d.CreatedAt = row.CreatedAt
		d.UpdatedAt = row.UpdatedAt
		return nil
	}

	// No active row — insert a fresh one. The partial unique index might race
	// us if two pods upsert concurrently for a brand-new (app, locale); rare
	// in practice (the AddLanguage handler gates it), and the second writer
	// will see a unique violation it can surface as repository.ErrConflict.
	if _, err := q.ExecContext(ctx, queryInsertNew, d.ID, d.ApplicationID, d.Locale, d.StageCompleted); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	// SELECT-back by id to hydrate the server-set timestamps.
	if err := q.GetContext(ctx, &row, querySelectByID, d.ID); err != nil {
		return err
	}
	d.CreatedAt = row.CreatedAt
	d.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *Impl) SetStage(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale, stage string) error {
	_, err := q.ExecContext(ctx, querySetStage, stage, appID, locale)
	return err
}

func (r *Impl) Delete(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale string) error {
	_, err := q.ExecContext(ctx, queryDelete, appID, locale)
	return err
}
