// Package job is the data access layer for async work queues. Three tables,
// three repos in one package:
//   - add_language_jobs   — add-locale fan-out across an application's components
//   - translate_jobs      — per-component, single or multi-target translate
//   - cms_translate_jobs  — per-CMS-item translate
//
// All three share the FOR UPDATE SKIP LOCKED claim pattern: workers claim
// the oldest pending job atomically, mark it `running` with their pod's
// claimed_by, and process it out-of-band. Stuck-running jobs older than
// 15 minutes get reset to pending so a crashed pod doesn't permanently
// orphan its in-flight work.
package job

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/translation"
)

// Status constants for all three job types.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Type constants for TranslateJob.JobType.
const (
	TranslateTypeAutoTranslate = "auto_translate"
	TranslateTypeBackfill      = "backfill"
)

// ─── AddLanguageJob ──────────────────────────────────────────────────────────

// AddLanguageJob describes a "add new locale to application + AI translate all
// components" task. One row per add-language request; the worker fans out
// internally with a goroutine pool.
type AddLanguageJob struct {
	ID                  uuid.UUID `db:"id"                   json:"id"`
	ApplicationID       uuid.UUID `db:"application_id"       json:"application_id"`
	Locale              string    `db:"locale"               json:"locale"`
	AutoTranslate       bool      `db:"auto_translate"       json:"auto_translate"`
	Status              string    `db:"status"               json:"status"`
	TotalComponents     int       `db:"total_components"     json:"total_components"`
	CompletedComponents int       `db:"completed_components" json:"completed_components"`
	ErrorMessage        string    `db:"error_message"        json:"error_message,omitempty"`
	ErrorDetail         string    `db:"error_detail"         json:"error_detail,omitempty"`
	ClaimedBy           string    `db:"claimed_by"           json:"claimed_by,omitempty"`
	CreatedBy           uuid.UUID `db:"created_by"           json:"created_by"`
	CreatedAt           time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"           json:"updated_at"`
}

type AddLanguageRepository interface {
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*AddLanguageJob, error)
	GetByIDForApp(ctx context.Context, q repository.Queryer, jobID, appID uuid.UUID) (*AddLanguageJob, error)
	FindActiveByLocale(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale string) (*AddLanguageJob, error)
	ListActiveByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]AddLanguageJob, error)
	Insert(ctx context.Context, q repository.Queryer, j *AddLanguageJob) error
	// ClaimNext atomically marks one pending job `running` (with the provided
	// instanceID) and returns it. ResetStuck must be invoked first to recover
	// jobs orphaned by a crashed worker.
	ClaimNext(ctx context.Context, q repository.Queryer, instanceID string) (*AddLanguageJob, error)
	// ResetStuck moves jobs stuck in `running` for >stuckAfter back to `pending`.
	ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error
	UpdateTotals(ctx context.Context, q repository.Queryer, jobID uuid.UUID, total, completed int) error
	IncrementCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error
	MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error
	MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error
}

// ─── TranslateJob ────────────────────────────────────────────────────────────

// TranslateJob describes a per-component translation task.
// TargetLocales is a Postgres text[]; single-target jobs have one element,
// multi-target backfills have N. Storage uses lib/pq's StringArray.
type TranslateJob struct {
	ID            uuid.UUID      `db:"id"             json:"id"`
	ApplicationID uuid.UUID      `db:"application_id" json:"application_id"`
	ComponentID   uuid.UUID      `db:"component_id"   json:"component_id"`
	JobType       string         `db:"job_type"       json:"job_type"`
	SourceLocale  string         `db:"source_locale"  json:"source_locale"`
	TargetLocales pq.StringArray `db:"target_locales" json:"target_locales"`
	Status        string         `db:"status"         json:"status"`
	ErrorMessage  string         `db:"error_message"  json:"error_message,omitempty"`
	ErrorDetail   string         `db:"error_detail"   json:"error_detail,omitempty"`
	ClaimedBy     string         `db:"claimed_by"     json:"claimed_by,omitempty"`
	CreatedBy     uuid.UUID      `db:"created_by"     json:"created_by"`
	CreatedAt     time.Time      `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"     json:"updated_at"`
}

type TranslateRepository interface {
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*TranslateJob, error)
	FindActive(ctx context.Context, q repository.Queryer, componentID uuid.UUID, sourceLocale, targetLocale, jobType string) (*TranslateJob, error)
	ListActiveByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]TranslateJob, error)
	Insert(ctx context.Context, q repository.Queryer, j *TranslateJob) error
	ClaimNext(ctx context.Context, q repository.Queryer, instanceID string) (*TranslateJob, error)
	ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error
	MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error
	MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error
}

// ─── CmsTranslateJob ─────────────────────────────────────────────────────────

// CmsTranslateJob is the per-CMS-item analogue of TranslateJob. Single
// target_locale (not an array — CMS templates are smaller and re-running
// per-locale is cheap), so the dedupe index is simpler.
type CmsTranslateJob struct {
	ID            uuid.UUID         `db:"id"             json:"id"`
	ApplicationID uuid.UUID         `db:"application_id" json:"application_id"`
	CmsItemID     uuid.UUID         `db:"cms_item_id"    json:"cms_item_id"`
	SourceLocale  string            `db:"source_locale"  json:"source_locale"`
	TargetLocale  string            `db:"target_locale"  json:"target_locale"`
	Stage         translation.Stage `db:"stage"          json:"stage"`
	Status        string            `db:"status"         json:"status"`
	ErrorMessage  string            `db:"error_message"  json:"error_message,omitempty"`
	ErrorDetail   string            `db:"error_detail"   json:"error_detail,omitempty"`
	ClaimedBy     string            `db:"claimed_by"     json:"claimed_by,omitempty"`
	CreatedBy     uuid.UUID         `db:"created_by"     json:"created_by"`
	CreatedAt     time.Time         `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time         `db:"updated_at"     json:"updated_at"`
}

type CmsTranslateRepository interface {
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*CmsTranslateJob, error)
	FindActive(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, sourceLocale, targetLocale string, stage translation.Stage) (*CmsTranslateJob, error)
	Insert(ctx context.Context, q repository.Queryer, j *CmsTranslateJob) error
	ClaimNext(ctx context.Context, q repository.Queryer, instanceID string) (*CmsTranslateJob, error)
	ResetStuck(ctx context.Context, q repository.Queryer, stuckAfter time.Duration) error
	MarkCompleted(ctx context.Context, q repository.Queryer, jobID uuid.UUID) error
	MarkFailed(ctx context.Context, q repository.Queryer, jobID uuid.UUID, errMsg, errDetail string) error
}
