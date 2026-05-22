// Package translation is the data access layer for `translation_versions` —
// versioned, per-locale, per-stage translation data for each component.
//
// Versioning conventions:
//   - Every "save" is an INSERT, never an UPDATE. A new version row appears
//     with version = MAX(version)+1 for the (component, locale, stage) tuple.
//   - The MAX(version)+1 read-then-insert is racy under concurrent writes; we
//     rely on the partial unique index idx_tv_unique_version to catch
//     collisions and SaveVersion retries up to maxSaveAttempts times.
//   - `is_active` always TRUE here. The flag exists for future workflow extensions
//     (e.g. soft-rejecting a version without deleting it).
//   - SourceLocale + SourceData record the locale and snapshot used at AI
//     translate time so subsequent re-translations can diff against the
//     snapshot and only re-send changed leaves. Empty for manual edits.
package translation

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

// Stage is one of draft, staging, production.
type Stage string

const (
	StageDraft      Stage = "draft"
	StageStaging    Stage = "staging"
	StageProduction Stage = "production"
)

// Version is the in-memory representation of a row from `translation_versions`.
type Version struct {
	ID           uuid.UUID        `db:"id"            json:"id"`
	ComponentID  uuid.UUID        `db:"component_id"  json:"component_id"`
	Locale       string           `db:"locale"        json:"locale"`
	Stage        Stage            `db:"stage"         json:"stage"`
	Version      int              `db:"version"       json:"version"`
	Data         repository.JSONB `db:"data"          json:"data"`
	SourceLocale string           `db:"source_locale" json:"source_locale,omitempty"`
	SourceData   repository.JSONB `db:"source_data"   json:"source_data,omitempty"`
	IsActive     bool             `db:"is_active"     json:"is_active"`
	CreatedBy    uuid.UUID        `db:"created_by"    json:"created_by"`
	UpdatedBy    uuid.UUID        `db:"updated_by"    json:"updated_by"`
	CreatedAt    time.Time        `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at"    json:"updated_at"`
}

// Repository is the contract for translation_version persistence.
type Repository interface {
	// GetLatest returns the highest-versioned active row for (componentID, locale, stage).
	// ErrNotFound when no row matches. Hot path — backed by idx_tv_lookup.
	GetLatest(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string, stage Stage) (*Version, error)

	// GetLatestByComponentIDs is the bulk equivalent of GetLatest. Returns
	// at most one row per (componentID, locale, stage) — the highest version.
	// Uses Postgres DISTINCT ON for the per-component selection.
	GetLatestByComponentIDs(ctx context.Context, q repository.Queryer, componentIDs []uuid.UUID, locale string, stage Stage) ([]Version, error)

	// GetByVersion returns a specific version row. ErrNotFound when missing.
	// Used by the revert flow which needs the historical Data.
	GetByVersion(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string, stage Stage, version int) (*Version, error)

	// ListVersions returns every version row for (componentID, locale, stage),
	// newest first. Drives the "version history" UI.
	ListVersions(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string, stage Stage) ([]Version, error)

	// SaveVersion inserts a new row with version = MAX(version)+1, retrying
	// up to maxSaveAttempts on the unique-constraint race. SourceLocale +
	// SourceData are stored as provided (empty/nil for manual edits, populated
	// for AI translation snapshots).
	SaveVersion(ctx context.Context, q repository.Queryer, v *Version) error

	// DeleteByID hard-deletes a single version row. Used by the rollback path
	// in the AddLanguage worker when a multi-component translate fails partway.
	DeleteByID(ctx context.Context, q repository.Queryer, id uuid.UUID) error

	// DeleteOldVersions hard-deletes rows older than the retention bound for
	// each (componentID, locale, stage). Run periodically by the retention job.
	// Returns the number of rows deleted.
	DeleteOldVersions(ctx context.Context, q repository.Queryer, keepLastN int) (int64, error)

	// DeleteByComponentLocale hard-deletes every version for (componentID, locale)
	// across all stages. Used by the DeleteLanguage cascade.
	DeleteByComponentLocale(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string) error

	// ListLatestLocales returns the highest-versioned active row for each locale
	// at (componentID, stage). Drives the all-locale export endpoint —
	// "give me the current translation in every language I have".
	ListLatestLocales(ctx context.Context, q repository.Queryer, componentID uuid.UUID, stage Stage) ([]Version, error)
}
