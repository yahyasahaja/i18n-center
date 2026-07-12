package job

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/translation"
)

const (
	queryCmsTranslateGetByID = `
		SELECT id, application_id, cms_item_id, source_locale, target_locale, stage,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM cms_translate_jobs
		WHERE id = ? AND deleted_at IS NULL
	`

	queryCmsTranslateFindActive = `
		SELECT id, application_id, cms_item_id, source_locale, target_locale, stage,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM cms_translate_jobs
		WHERE cms_item_id = ?
		  AND source_locale = ?
		  AND target_locale = ?
		  AND stage = ?
		  AND status IN ('pending', 'running')
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	queryCmsTranslateInsert = `
		INSERT INTO cms_translate_jobs (
			id, application_id, cms_item_id, source_locale, target_locale, stage,
			status, error_message, error_detail, claimed_by, created_by,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'pending', '', '', '', ?, NOW(), NOW())
	`

	// 3-statement claim pattern — see translate_impl.go for the rationale.
	queryCmsTranslateClaimSelect = `
		SELECT id FROM cms_translate_jobs
		WHERE status = 'pending' AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	queryCmsTranslateClaimUpdate = `
		UPDATE cms_translate_jobs
		SET status = 'running', claimed_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	queryCmsTranslateClaimSelectRow = `
		SELECT id, application_id, cms_item_id, source_locale, target_locale, stage,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM cms_translate_jobs
		WHERE id = ?
	`

	queryCmsTranslateResetStuck = `
		UPDATE cms_translate_jobs
		SET status = 'pending', claimed_by = '', updated_at = NOW()
		WHERE status = 'running'
		  AND updated_at < NOW() - INTERVAL ? SECOND
		  AND deleted_at IS NULL
	`

	queryCmsTranslateMarkCompleted = `
		UPDATE cms_translate_jobs SET status = 'completed', updated_at = NOW() WHERE id = ?
	`

	queryCmsTranslateMarkFailed = `
		UPDATE cms_translate_jobs
		SET status = 'failed', error_message = ?, error_detail = ?, updated_at = NOW()
		WHERE id = ?
	`
)

// cmsTranslateImpl carries the underlying *sqlx.DB so ClaimNext can open its
// own transaction — the MySQL claim pattern needs FOR UPDATE SKIP LOCKED
// inside a tx. Every other method still routes through the passed Queryer
// so it composes with an outer WithTx.
type cmsTranslateImpl struct {
	db *sqlx.DB
}

// NewCmsTranslateRepository returns the default CmsTranslateJob repository.
func NewCmsTranslateRepository(db *sqlx.DB) CmsTranslateRepository {
	return &cmsTranslateImpl{db: db}
}

func (r *cmsTranslateImpl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*CmsTranslateJob, error) {
	var j CmsTranslateJob
	if err := q.GetContext(ctx, &j, queryCmsTranslateGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *cmsTranslateImpl) FindActive(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, sourceLocale, targetLocale string, stage translation.Stage) (*CmsTranslateJob, error) {
	var j CmsTranslateJob
	if err := q.GetContext(ctx, &j, queryCmsTranslateFindActive, cmsItemID, sourceLocale, targetLocale, stage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *cmsTranslateImpl) Insert(ctx context.Context, q repository.Queryer, j *CmsTranslateJob) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	_, err := q.ExecContext(ctx, queryCmsTranslateInsert,
		j.ID, j.ApplicationID, j.CmsItemID, j.SourceLocale, j.TargetLocale, j.Stage, j.CreatedBy,
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

// ClaimNext atomically picks the oldest pending CMS translate job. See
// translate_impl.go's ClaimNext for the full rationale — same 3-statement
// pattern, same "nil, nil on empty queue" semantics.
func (r *cmsTranslateImpl) ClaimNext(ctx context.Context, _ repository.Queryer, instanceID string) (*CmsTranslateJob, error) {
	var job CmsTranslateJob
	err := repository.WithTx(ctx, r.db, func(tx repository.Queryer) error {
		var id uuid.UUID
		if err := tx.GetContext(ctx, &id, queryCmsTranslateClaimSelect); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, queryCmsTranslateClaimUpdate, instanceID, id); err != nil {
			return err
		}
		return tx.GetContext(ctx, &job, queryCmsTranslateClaimSelectRow, id)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *cmsTranslateImpl) ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error {
	seconds := int64(stuckAfter.Seconds())
	_, err := q.ExecContext(ctx, queryCmsTranslateResetStuck, seconds)
	return err
}

func (r *cmsTranslateImpl) MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error {
	_, err := q.ExecContext(ctx, queryCmsTranslateMarkCompleted, jobID)
	return err
}

func (r *cmsTranslateImpl) MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error {
	_, err := q.ExecContext(ctx, queryCmsTranslateMarkFailed, errMsg, errDetail, jobID)
	return err
}
