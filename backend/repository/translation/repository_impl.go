package translation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/lapakgaming/i18n-center/repository"
)

// maxSaveAttempts caps the SaveVersion retry loop. A high collision count
// here means the same (component, locale, stage) is being hammered by many
// writers — application-level pathology, not normal load. Bail rather than
// loop forever.
const maxSaveAttempts = 5

const (
	queryGetLatest = `
		SELECT id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM translation_versions
		WHERE component_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND is_active = TRUE
		  AND deleted_at IS NULL
		ORDER BY version DESC
		LIMIT 1
	`

	// DISTINCT ON returns one row per partition (the first per ORDER BY).
	// Ordering by (component_id, version DESC) makes "first per component_id"
	// == "highest version for that component". Backed by idx_tv_lookup.
	queryGetLatestByComponentIDs = `
		SELECT DISTINCT ON (component_id)
		       id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM translation_versions
		WHERE component_id = ANY($1::uuid[])
		  AND locale = $2
		  AND stage = $3
		  AND is_active = TRUE
		  AND deleted_at IS NULL
		ORDER BY component_id, version DESC
	`

	queryGetByVersion = `
		SELECT id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM translation_versions
		WHERE component_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND version = $4
		  AND deleted_at IS NULL
	`

	queryListVersions = `
		SELECT id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM translation_versions
		WHERE component_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND deleted_at IS NULL
		ORDER BY version DESC
	`

	queryNextVersion = `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM translation_versions
		WHERE component_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND deleted_at IS NULL
	`

	queryInsertVersion = `
		INSERT INTO translation_versions (
			id, component_id, locale, stage, version,
			data, source_locale, source_data, is_active,
			created_by, updated_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, TRUE, $9, $9, NOW(), NOW()
		)
	`

	queryDeleteByID = `
		DELETE FROM translation_versions WHERE id = $1
	`

	// Retention sweep. Within each (component_id, locale, stage) partition,
	// keep only the keepLastN most recent rows by version. Hard delete the rest.
	queryDeleteOldVersions = `
		DELETE FROM translation_versions
		WHERE id IN (
			SELECT id FROM (
				SELECT id,
				       ROW_NUMBER() OVER (
				           PARTITION BY component_id, locale, stage
				           ORDER BY version DESC
				       ) AS rn
				FROM translation_versions
			) sub
			WHERE rn > $1
		)
	`

	queryDeleteByComponentLocale = `
		DELETE FROM translation_versions
		WHERE component_id = $1 AND locale = $2
	`

	// ListLatestLocales: one row per locale, highest-versioned, for a single
	// (component, stage). DISTINCT ON does the per-locale selection without
	// a self-join; the ORDER BY clause picks the winner.
	queryListLatestLocales = `
		SELECT DISTINCT ON (locale)
		       id, component_id, locale, stage, version, data,
		       source_locale, source_data, is_active, created_by, updated_by,
		       created_at, updated_at
		FROM translation_versions
		WHERE component_id = $1 AND stage = $2 AND is_active = TRUE
		ORDER BY locale, version DESC
	`
)

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetLatest(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string, stage Stage) (*Version, error) {
	var v Version
	if err := q.GetContext(ctx, &v, queryGetLatest, componentID, locale, stage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *Impl) GetLatestByComponentIDs(ctx context.Context, q repository.Queryer, componentIDs []uuid.UUID, locale string, stage Stage) ([]Version, error) {
	if len(componentIDs) == 0 {
		return []Version{}, nil
	}
	// pq.Array needs a slice of natively-supported types; uuid → string is the
	// cheapest broadly-supported path. The CAST inside the SQL coerces back.
	ids := make([]string, len(componentIDs))
	for i, id := range componentIDs {
		ids[i] = id.String()
	}
	out := []Version{}
	if err := q.SelectContext(ctx, &out, queryGetLatestByComponentIDs, pq.Array(ids), locale, stage); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Impl) GetByVersion(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string, stage Stage, version int) (*Version, error) {
	var v Version
	if err := q.GetContext(ctx, &v, queryGetByVersion, componentID, locale, stage, version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *Impl) ListVersions(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string, stage Stage) ([]Version, error) {
	out := []Version{}
	if err := q.SelectContext(ctx, &out, queryListVersions, componentID, locale, stage); err != nil {
		return nil, err
	}
	return out, nil
}

// SaveVersion implements the read-MAX-then-insert pattern with retry on race.
//
// Two concurrent writers can both compute the same next-version. The partial
// unique index idx_tv_unique_version turns the collision into a duplicate-key
// error. We catch that via repository.IsUniqueViolation and re-read MAX(version)
// up to maxSaveAttempts times. A high collision count here is application-
// level pathology (one component being hammered by parallel saves), not
// normal load.
//
// The retry runs inside the caller's Queryer — if they're in a transaction,
// the colliding INSERT aborts the tx, so retrying requires the caller to
// retry the whole transaction. For autocommit Queryers (the common path) the
// retry is safe.
func (r *Impl) SaveVersion(ctx context.Context, q repository.Queryer, v *Version) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.IsActive == false && v.Version == 0 {
		// Caller didn't set IsActive — default to TRUE for normal saves.
		v.IsActive = true
	}

	var lastErr error
	for attempt := 1; attempt <= maxSaveAttempts; attempt++ {
		var next int
		if err := q.GetContext(ctx, &next, queryNextVersion, v.ComponentID, v.Locale, v.Stage); err != nil {
			return fmt.Errorf("compute next version: %w", err)
		}
		v.Version = next
		_, err := q.ExecContext(ctx, queryInsertVersion,
			v.ID, v.ComponentID, v.Locale, v.Stage, v.Version,
			v.Data, v.SourceLocale, v.SourceData, v.CreatedBy,
		)
		if err == nil {
			return nil
		}
		lastErr = err
		if !repository.IsUniqueViolation(err) {
			return err
		}
		// New ID for the retry — Postgres won't reject the same UUID on retry
		// since we never committed, but generating a fresh one is cheap insurance.
		v.ID = uuid.New()
	}
	return fmt.Errorf("translation.SaveVersion: exhausted %d retries on unique-version conflict: %w", maxSaveAttempts, lastErr)
}

func (r *Impl) DeleteByID(ctx context.Context, q repository.Queryer, id uuid.UUID) error {
	result, err := q.ExecContext(ctx, queryDeleteByID, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Impl) DeleteOldVersions(ctx context.Context, q repository.Queryer, keepLastN int) (int64, error) {
	result, err := q.ExecContext(ctx, queryDeleteOldVersions, keepLastN)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Impl) DeleteByComponentLocale(ctx context.Context, q repository.Queryer, componentID uuid.UUID, locale string) error {
	_, err := q.ExecContext(ctx, queryDeleteByComponentLocale, componentID, locale)
	return err
}

func (r *Impl) ListLatestLocales(ctx context.Context, q repository.Queryer, componentID uuid.UUID, stage Stage) ([]Version, error) {
	rows := []Version{}
	if err := q.SelectContext(ctx, &rows, queryListLatestLocales, componentID, stage); err != nil {
		return nil, err
	}
	return rows, nil
}

// keep sqlx referenced in case future helpers grow here.
var _ = sqlx.In
