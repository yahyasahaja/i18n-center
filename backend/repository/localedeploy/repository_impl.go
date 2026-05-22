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

	// Upsert leans on the (application_id, locale) unique index. On conflict
	// we bump stage_completed to whatever the caller asked for — that's the
	// progression path (draft → staging → production) and also the "reset to
	// draft after a re-translate" path used by the AddLanguage worker.
	queryUpsert = `
		INSERT INTO application_locale_deploys (
			id, application_id, locale, stage_completed, created_at, updated_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (application_id, locale) DO UPDATE
		SET stage_completed = EXCLUDED.stage_completed,
		    updated_at = NOW()
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
	if err := q.GetContext(ctx, &row, queryUpsert, d.ID, d.ApplicationID, d.Locale, d.StageCompleted); err != nil {
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
