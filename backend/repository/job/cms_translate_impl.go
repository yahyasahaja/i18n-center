package job

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/translation"
)

const (
	queryCmsTranslateGetByID = `
		SELECT id, application_id, cms_item_id, source_locale, target_locale, stage,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM cms_translate_jobs
		WHERE id = $1 AND deleted_at IS NULL
	`

	queryCmsTranslateFindActive = `
		SELECT id, application_id, cms_item_id, source_locale, target_locale, stage,
		       status, error_message, error_detail, claimed_by,
		       created_by, created_at, updated_at
		FROM cms_translate_jobs
		WHERE cms_item_id = $1
		  AND source_locale = $2
		  AND target_locale = $3
		  AND stage = $4
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
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending', '', '', '', $7, NOW(), NOW())
	`

	queryCmsTranslateClaim = `
		UPDATE cms_translate_jobs
		SET status = 'running', claimed_by = $1, updated_at = NOW()
		WHERE id = (
			SELECT id FROM cms_translate_jobs
			WHERE status = 'pending' AND deleted_at IS NULL
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, application_id, cms_item_id, source_locale, target_locale, stage,
		          status, error_message, error_detail, claimed_by,
		          created_by, created_at, updated_at
	`

	queryCmsTranslateResetStuck = `
		UPDATE cms_translate_jobs
		SET status = 'pending', claimed_by = '', updated_at = NOW()
		WHERE status = 'running'
		  AND updated_at < NOW() - ($1 || ' seconds')::INTERVAL
		  AND deleted_at IS NULL
	`

	queryCmsTranslateMarkCompleted = `
		UPDATE cms_translate_jobs SET status = 'completed', updated_at = NOW() WHERE id = $1
	`

	queryCmsTranslateMarkFailed = `
		UPDATE cms_translate_jobs
		SET status = 'failed', error_message = $2, error_detail = $3, updated_at = NOW()
		WHERE id = $1
	`
)

type cmsTranslateImpl struct{}

// NewCmsTranslateRepository returns the default CmsTranslateJob repository.
func NewCmsTranslateRepository() CmsTranslateRepository { return &cmsTranslateImpl{} }

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

func (r *cmsTranslateImpl) ClaimNext(ctx context.Context, q repository.Queryer, instanceID string) (*CmsTranslateJob, error) {
	var j CmsTranslateJob
	if err := q.GetContext(ctx, &j, queryCmsTranslateClaim, instanceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &j, nil
}

func (r *cmsTranslateImpl) ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error {
	seconds := int64(stuckAfter.Seconds())
	_, err := q.ExecContext(ctx, queryCmsTranslateResetStuck, fmt.Sprintf("%d", seconds))
	return err
}

func (r *cmsTranslateImpl) MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error {
	_, err := q.ExecContext(ctx, queryCmsTranslateMarkCompleted, jobID)
	return err
}

func (r *cmsTranslateImpl) MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error {
	_, err := q.ExecContext(ctx, queryCmsTranslateMarkFailed, jobID, errMsg, errDetail)
	return err
}
