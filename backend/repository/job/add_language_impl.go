package job

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lapakgaming/i18n-center/repository"
)

const (
	queryAddLangGetByID = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	queryAddLangGetByIDForApp = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE id = ? AND application_id = ?
		  AND deleted_at IS NULL
	`

	queryAddLangFindActiveByLocale = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE application_id = ?
		  AND locale = ?
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
		WHERE application_id = ?
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
		) VALUES (?, ?, ?, ?, 'pending', 0, 0, '', '', '', ?, NOW(), NOW())
	`

	// Claim atomically. MySQL 8 has no RETURNING, so the claim is 3 statements
	// inside a transaction:
	//   1. Pick the oldest pending row under FOR UPDATE SKIP LOCKED — other
	//      replicas racing on the same tick will skip our candidate.
	//   2. UPDATE to flip status to running with our claimed_by.
	//   3. SELECT the fresh row so the worker gets consistent data.
	// Returns no rows when nothing's pending — surfaced as (nil, nil).
	queryAddLangClaimSelect = `
		SELECT id FROM add_language_jobs
		WHERE status = 'pending' AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	queryAddLangClaimUpdate = `
		UPDATE add_language_jobs
		SET status = 'running', claimed_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	queryAddLangClaimSelectRow = `
		SELECT id, application_id, locale, auto_translate, status,
		       total_components, completed_components,
		       error_message, error_detail, claimed_by, created_by,
		       created_at, updated_at
		FROM add_language_jobs
		WHERE id = ?
	`

	queryAddLangResetStuck = `
		UPDATE add_language_jobs
		SET status = 'pending', claimed_by = '', updated_at = NOW()
		WHERE status = 'running'
		  AND updated_at < NOW() - INTERVAL ? SECOND
		  AND deleted_at IS NULL
	`

	queryAddLangUpdateTotals = `
		UPDATE add_language_jobs
		SET total_components = ?,
		    completed_components = ?,
		    updated_at = NOW()
		WHERE id = ?
	`

	queryAddLangIncrementCompleted = `
		UPDATE add_language_jobs
		SET completed_components = completed_components + 1,
		    updated_at = NOW()
		WHERE id = ?
	`

	queryAddLangMarkCompleted = `
		UPDATE add_language_jobs
		SET status = 'completed', updated_at = NOW()
		WHERE id = ?
	`

	queryAddLangMarkFailed = `
		UPDATE add_language_jobs
		SET status = 'failed',
		    error_message = ?,
		    error_detail = ?,
		    updated_at = NOW()
		WHERE id = ?
	`
)

// addLangImpl carries the underlying *sqlx.DB for the ClaimNext transaction.
// Every other method uses the passed Queryer so it composes with an outer
// WithTx.
type addLangImpl struct {
	db *sqlx.DB
}

// NewAddLanguageRepository returns the default AddLanguageJob repository.
func NewAddLanguageRepository(db *sqlx.DB) AddLanguageRepository {
	return &addLangImpl{db: db}
}

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

// ClaimNext atomically picks the oldest pending add-language job. See
// translate_impl.go's ClaimNext for the full rationale — same 3-statement
// pattern, same "no pending work → (nil, nil)" semantics (distinct from
// ErrNotFound so worker callers can loop without treating an empty queue as
// an error).
func (r *addLangImpl) ClaimNext(ctx context.Context, _ repository.Queryer, instanceID string) (*AddLanguageJob, error) {
	var job AddLanguageJob
	err := repository.WithTx(ctx, r.db, func(tx repository.Queryer) error {
		var id uuid.UUID
		if err := tx.GetContext(ctx, &id, queryAddLangClaimSelect); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, queryAddLangClaimUpdate, instanceID, id); err != nil {
			return err
		}
		return tx.GetContext(ctx, &job, queryAddLangClaimSelectRow, id)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *addLangImpl) ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error {
	seconds := int64(stuckAfter.Seconds())
	_, err := q.ExecContext(ctx, queryAddLangResetStuck, seconds)
	return err
}

func (r *addLangImpl) UpdateTotals(ctx context.Context, q repository.Queryer, jobID uuid.UUID, total, completed int) error {
	_, err := q.ExecContext(ctx, queryAddLangUpdateTotals, total, completed, jobID)
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
	_, err := q.ExecContext(ctx, queryAddLangMarkFailed, errMsg, errDetail, jobID)
	return err
}
