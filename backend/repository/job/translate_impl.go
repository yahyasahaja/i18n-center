package job

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

const (
	queryTranslateGetByID = `
		SELECT id, application_id, component_id, job_type, source_locale, target_locales,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM translate_jobs
		WHERE id = $1 AND deleted_at IS NULL
	`

	// FindActive matches the dedupe-index tuple: (component_id, source_locale,
	// first target_locale, job_type) among active rows. Used both as an
	// idempotency check before insert and as a fallback after a unique-key
	// race on insert.
	queryTranslateFindActive = `
		SELECT id, application_id, component_id, job_type, source_locale, target_locales,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM translate_jobs
		WHERE component_id = $1
		  AND source_locale = $2
		  AND target_locales[1] = $3
		  AND job_type = $4
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
		WHERE application_id = $1
		  AND status IN ('pending', 'running')
		  AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	queryTranslateInsert = `
		INSERT INTO translate_jobs (
			id, application_id, component_id, job_type, source_locale, target_locales,
			status, error_message, error_detail, claimed_by, created_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending', '', '', '', $7, NOW(), NOW())
	`

	queryTranslateClaim = `
		UPDATE translate_jobs
		SET status = 'running', claimed_by = $1, updated_at = NOW()
		WHERE id = (
			SELECT id FROM translate_jobs
			WHERE status = 'pending' AND deleted_at IS NULL
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, application_id, component_id, job_type, source_locale, target_locales,
		          status, error_message, error_detail, claimed_by,
		          created_by, created_at, updated_at
	`

	queryTranslateResetStuck = `
		UPDATE translate_jobs
		SET status = 'pending', claimed_by = '', updated_at = NOW()
		WHERE status = 'running'
		  AND updated_at < NOW() - ($1 || ' seconds')::INTERVAL
		  AND deleted_at IS NULL
	`

	queryTranslateMarkCompleted = `
		UPDATE translate_jobs SET status = 'completed', updated_at = NOW() WHERE id = $1
	`

	queryTranslateMarkFailed = `
		UPDATE translate_jobs
		SET status = 'failed', error_message = $2, error_detail = $3, updated_at = NOW()
		WHERE id = $1
	`
)

type translateImpl struct{}

// NewTranslateRepository returns the default TranslateJob repository.
func NewTranslateRepository() TranslateRepository { return &translateImpl{} }

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

func (r *translateImpl) ClaimNext(ctx context.Context, q repository.Queryer, instanceID string) (*TranslateJob, error) {
	var j TranslateJob
	if err := q.GetContext(ctx, &j, queryTranslateClaim, instanceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &j, nil
}

func (r *translateImpl) ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error {
	seconds := int64(stuckAfter.Seconds())
	_, err := q.ExecContext(ctx, queryTranslateResetStuck, fmt.Sprintf("%d", seconds))
	return err
}

func (r *translateImpl) MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error {
	_, err := q.ExecContext(ctx, queryTranslateMarkCompleted, jobID)
	return err
}

func (r *translateImpl) MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error {
	_, err := q.ExecContext(ctx, queryTranslateMarkFailed, jobID, errMsg, errDetail)
	return err
}
