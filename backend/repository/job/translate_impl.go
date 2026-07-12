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
	queryTranslateGetByID = `
		SELECT id, application_id, component_id, job_type, source_locale, target_locales,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM translate_jobs
		WHERE id = ? AND deleted_at IS NULL
	`

	// FindActive matches the dedupe-index tuple: (component_id, source_locale,
	// first target_locale, job_type) among active rows. Used both as an
	// idempotency check before insert and as a fallback after a unique-key
	// race on insert. `first_target_locale` is a generated column populated
	// from `target_locales->>'$[0]'` in the MySQL schema.
	queryTranslateFindActive = `
		SELECT id, application_id, component_id, job_type, source_locale, target_locales,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM translate_jobs
		WHERE component_id = ?
		  AND source_locale = ?
		  AND first_target_locale = ?
		  AND job_type = ?
		  AND status IN ('pending', 'running')
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	queryTranslateListActiveByApp = `
		SELECT id, application_id, component_id, job_type, source_locale, target_locales,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM translate_jobs
		WHERE application_id = ?
		  AND status IN ('pending', 'running')
		  AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	queryTranslateInsert = `
		INSERT INTO translate_jobs (
			id, application_id, component_id, job_type, source_locale, target_locales,
			status, error_message, error_detail, claimed_by, created_by,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'pending', '', '', '', ?, NOW(), NOW())
	`

	// Claim pattern for MySQL 8 — 3 statements inside a transaction because
	// MySQL has no RETURNING clause.
	//   1. Pick the oldest pending row under a row lock (SKIP LOCKED so other
	//      replicas don't block on our candidate).
	//   2. Flip its status to running with our claimed_by.
	//   3. Read it back so the worker gets the fresh row.
	queryTranslateClaimSelect = `
		SELECT id FROM translate_jobs
		WHERE status = 'pending' AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	queryTranslateClaimUpdate = `
		UPDATE translate_jobs
		SET status = 'running', claimed_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	queryTranslateClaimSelectRow = `
		SELECT id, application_id, component_id, job_type, source_locale, target_locales,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM translate_jobs
		WHERE id = ?
	`

	queryTranslateResetStuck = `
		UPDATE translate_jobs
		SET status = 'pending', claimed_by = '', updated_at = NOW()
		WHERE status = 'running'
		  AND updated_at < NOW() - INTERVAL ? SECOND
		  AND deleted_at IS NULL
	`

	queryTranslateMarkCompleted = `
		UPDATE translate_jobs SET status = 'completed', updated_at = NOW() WHERE id = ?
	`

	queryTranslateMarkFailed = `
		UPDATE translate_jobs
		SET status = 'failed', error_message = ?, error_detail = ?, updated_at = NOW()
		WHERE id = ?
	`
)

// translateImpl carries the underlying *sqlx.DB so ClaimNext can open its own
// transaction for the multi-statement MySQL claim pattern. All other methods
// still accept a Queryer arg so they compose with an outer WithTx.
type translateImpl struct {
	db *sqlx.DB
}

// NewTranslateRepository returns the default TranslateJob repository. The db
// handle is used only by ClaimNext (which needs a fresh tx per call regardless
// of the caller's Queryer); every other method routes through the passed
// Queryer so it participates in an outer transaction when the caller opens one.
func NewTranslateRepository(db *sqlx.DB) TranslateRepository {
	return &translateImpl{db: db}
}

func (r *translateImpl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*TranslateJob, error) {
	var j TranslateJob
	if err := q.GetContext(ctx, &j, queryTranslateGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *translateImpl) FindActive(ctx context.Context, q repository.Queryer, componentID uuid.UUID, sourceLocale, targetLocale, jobType string) (*TranslateJob, error) {
	var j TranslateJob
	if err := q.GetContext(ctx, &j, queryTranslateFindActive, componentID, sourceLocale, targetLocale, jobType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *translateImpl) ListActiveByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]TranslateJob, error) {
	out := []TranslateJob{}
	if err := q.SelectContext(ctx, &out, queryTranslateListActiveByApp, appID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *translateImpl) Insert(ctx context.Context, q repository.Queryer, j *TranslateJob) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	_, err := q.ExecContext(ctx, queryTranslateInsert,
		j.ID, j.ApplicationID, j.ComponentID, j.JobType, j.SourceLocale, j.TargetLocales, j.CreatedBy,
	)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	j.Status = StatusPending
	now := time.Now()
	j.CreatedAt = now
	j.UpdatedAt = now
	return nil
}

// ClaimNext atomically picks the oldest pending translate job, marks it
// running under instanceID, and returns the fresh row. Semantics preserved
// from the Postgres one-shot UPDATE...RETURNING: no pending work → (nil, nil).
//
// Implementation ignores the passed Queryer and opens a dedicated transaction
// against r.db. FOR UPDATE SKIP LOCKED must run inside a transaction, and the
// caller (backend/jobs/worker.go) always calls ClaimNext outside any outer tx.
func (r *translateImpl) ClaimNext(ctx context.Context, _ repository.Queryer, instanceID string) (*TranslateJob, error) {
	var job TranslateJob
	err := repository.WithTx(ctx, r.db, func(tx repository.Queryer) error {
		// 1. Pick a candidate under a row lock.
		var id uuid.UUID
		if err := tx.GetContext(ctx, &id, queryTranslateClaimSelect); err != nil {
			return err
		}

		// 2. Flip its status.
		if _, err := tx.ExecContext(ctx, queryTranslateClaimUpdate, instanceID, id); err != nil {
			return err
		}

		// 3. Read the fresh row back.
		return tx.GetContext(ctx, &job, queryTranslateClaimSelectRow, id)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *translateImpl) ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error {
	seconds := int64(stuckAfter.Seconds())
	_, err := q.ExecContext(ctx, queryTranslateResetStuck, seconds)
	return err
}

func (r *translateImpl) MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error {
	_, err := q.ExecContext(ctx, queryTranslateMarkCompleted, jobID)
	return err
}

func (r *translateImpl) MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error {
	_, err := q.ExecContext(ctx, queryTranslateMarkFailed, errMsg, errDetail, jobID)
	return err
}
