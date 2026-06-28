package job

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

const (
	queryAddLangGetByID = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryAddLangGetByIDForApp = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE id = $1 AND application_id = $2
		  AND deleted_at IS NULL
	`

	queryAddLangFindActiveByLocale = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE application_id = $1
		  AND locale = $2
		  AND status IN ('pending', 'running')
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	queryAddLangListActiveByApp = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE application_id = $1
		  AND status IN ('pending', 'running')
		  AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	queryAddLangInsert = `
		INSERT INTO add_language_jobs (
			id, application_id, locale, auto_translate, status,
			total_components, completed_components,
			error_message, error_detail, claimed_by, created_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'pending', 0, 0, '', '', '', $5, NOW(), NOW())
	`

	// Claim atomically. UPDATE...RETURNING with a FOR UPDATE SKIP LOCKED
	// sub-select guarantees exactly one worker claims a given job, even with
	// multiple replicas racing. Returns no rows when nothing's pending.
	queryAddLangClaim = `
		UPDATE add_language_jobs
		SET status = 'running', claimed_by = $1, updated_at = NOW()
		WHERE id = (
			SELECT id FROM add_language_jobs
			WHERE status = 'pending' AND deleted_at IS NULL
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, application_id, locale, auto_translate, status,
		          total_components, completed_components,
		          error_message, error_detail, claimed_by, created_by,
		          created_at, updated_at
	`

	queryAddLangResetStuck = `
		UPDATE add_language_jobs
		SET status = 'pending', claimed_by = '', updated_at = NOW()
		WHERE status = 'running'
		  AND updated_at < NOW() - ($1 || ' seconds')::INTERVAL
		  AND deleted_at IS NULL
	`

	queryAddLangUpdateTotals = `
		UPDATE add_language_jobs
		SET total_components = $2,
		    completed_components = $3,
		    updated_at = NOW()
		WHERE id = $1
	`

	queryAddLangIncrementCompleted = `
		UPDATE add_language_jobs
		SET completed_components = completed_components + 1,
		    updated_at = NOW()
		WHERE id = $1
	`

	queryAddLangMarkCompleted = `
		UPDATE add_language_jobs
		SET status = 'completed', updated_at = NOW()
		WHERE id = $1
	`

	queryAddLangMarkFailed = `
		UPDATE add_language_jobs
		SET status = 'failed',
		    error_message = $2,
		    error_detail = $3,
		    updated_at = NOW()
		WHERE id = $1
	`
)

type addLangImpl struct{}

// NewAddLanguageRepository returns the default AddLanguageJob repository.
func NewAddLanguageRepository() AddLanguageRepository { return &addLangImpl{} }

func (r *addLangImpl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*AddLanguageJob, error) {
	var j AddLanguageJob
	if err := q.GetContext(ctx, &j, queryAddLangGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *addLangImpl) GetByIDForApp(ctx context.Context, q repository.Queryer, jobID, appID uuid.UUID) (*AddLanguageJob, error) {
	var j AddLanguageJob
	if err := q.GetContext(ctx, &j, queryAddLangGetByIDForApp, jobID, appID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *addLangImpl) FindActiveByLocale(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale string) (*AddLanguageJob, error) {
	var j AddLanguageJob
	if err := q.GetContext(ctx, &j, queryAddLangFindActiveByLocale, appID, locale); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *addLangImpl) ListActiveByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]AddLanguageJob, error) {
	out := []AddLanguageJob{}
	if err := q.SelectContext(ctx, &out, queryAddLangListActiveByApp, appID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *addLangImpl) Insert(ctx context.Context, q repository.Queryer, j *AddLanguageJob) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	_, err := q.ExecContext(ctx, queryAddLangInsert,
		j.ID, j.ApplicationID, j.Locale, j.AutoTranslate, j.CreatedBy,
	)
	if err != nil {
		return err
	}
	j.Status = StatusPending
	now := time.Now()
	j.CreatedAt = now
	j.UpdatedAt = now
	return nil
}

func (r *addLangImpl) ClaimNext(ctx context.Context, q repository.Queryer, instanceID string) (*AddLanguageJob, error) {
	var j AddLanguageJob
	if err := q.GetContext(ctx, &j, queryAddLangClaim, instanceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // no pending work — distinct from ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *addLangImpl) ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error {
	seconds := int64(stuckAfter.Seconds())
	_, err := q.ExecContext(ctx, queryAddLangResetStuck, fmt.Sprintf("%d", seconds))
	return err
}

func (r *addLangImpl) UpdateTotals(ctx context.Context, q repository.Queryer, jobID uuid.UUID, total, completed int) error {
	_, err := q.ExecContext(ctx, queryAddLangUpdateTotals, jobID, total, completed)
	return err
}

func (r *addLangImpl) IncrementCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error {
	_, err := q.ExecContext(ctx, queryAddLangIncrementCompleted, jobID)
	return err
}

func (r *addLangImpl) MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error {
	_, err := q.ExecContext(ctx, queryAddLangMarkCompleted, jobID)
	return err
}

func (r *addLangImpl) MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error {
	_, err := q.ExecContext(ctx, queryAddLangMarkFailed, jobID, errMsg, errDetail)
	return err
}
