package translation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

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
		WHERE component_id = ?
		  AND locale = ?
		  AND stage = ?
		  AND is_active = TRUE
		  AND deleted_at IS NULL
		ORDER BY version DESC
		LIMIT 1
	`

	// Per-component-highest-version selection. MySQL has no DISTINCT ON, so we
	// use ROW_NUMBER() OVER (PARTITION BY component_id ORDER BY version DESC)
	// in an inner subquery and filter rn = 1 in an outer wrapper — MySQL doesn't
	// allow window-function results in WHERE directly. Outer explicit column
	// list drops the extra rn (and any translation_versions columns Version
	// doesn't bind, e.g. deleted_at) so sqlx StructScan stays strict-safe.
	// The IN (?) is expanded by sqlx.In at call time. Backed by idx_tv_lookup.
	queryGetLatestByComponentIDs = `
		SELECT id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM (
			SELECT tv.*,
			       ROW_NUMBER() OVER (PARTITION BY component_id ORDER BY version DESC) AS rn
			FROM translation_versions tv
			WHERE component_id IN (?)
			  AND locale = ?
			  AND stage = ?
			  AND is_active = TRUE
			  AND deleted_at IS NULL
		) t
		WHERE rn = 1
	`

	queryGetByVersion = `
		SELECT id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM translation_versions
		WHERE component_id = ?
		  AND locale = ?
		  AND stage = ?
		  AND version = ?
		  AND deleted_at IS NULL
	`

	queryListVersions = `
		SELECT id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM translation_versions
		WHERE component_id = ?
		  AND locale = ?
		  AND stage = ?
		  AND deleted_at IS NULL
		ORDER BY version DESC
	`

	queryNextVersion = `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM translation_versions
		WHERE component_id = ?
		  AND locale = ?
		  AND stage = ?
		  AND deleted_at IS NULL
	`

	queryInsertVersion = `
		INSERT INTO translation_versions (
			id, component_id, locale, stage, version,
			data, source_locale, source_data, is_active,
			created_by, updated_by, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?, NOW(), NOW()
		)
	`

	queryDeleteByID = `
		DELETE FROM translation_versions WHERE id = ?
	`

	// Retention sweep. Within each (component_id, locale, stage) partition,
	// keep only the keepLastN most recent rows by version. Hard delete the rest.
	// The extra `SELECT id FROM (...) sub` wrapper is required because MySQL
	// disallows referencing the target table of a DELETE inside its own
	// subquery unless the subquery is aliased through another derived table.
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
			WHERE rn > ?
		)
	`

	queryDeleteByComponentLocale = `
		DELETE FROM translation_versions
		WHERE component_id = ? AND locale = ?
	`

	// ListLatestLocales: one row per locale, highest-versioned, for a single
	// (component, stage). Same ROW_NUMBER + outer wrapper pattern as
	// queryGetLatestByComponentIDs but partitioned by locale.
	queryListLatestLocales = `
		SELECT id, component_id, locale, stage, version,
		       data, source_locale, source_data, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM (
			SELECT tv.*,
			       ROW_NUMBER() OVER (PARTITION BY locale ORDER BY version DESC) AS rn
			FROM translation_versions tv
			WHERE component_id = ?
			  AND stage = ?
			  AND is_active = TRUE
		) t
		WHERE rn = 1
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
	// sqlx.In needs a driver-compatible element type; uuid → string keeps the
	// path broadly supported by the mysql driver and matches how UUID PKs are
	// stored (CHAR(36)) in the ported schema.
	ids := make([]string, len(componentIDs))
	for i, id := range componentIDs {
		ids[i] = id.String()
	}
	query, args, err := sqlx.In(queryGetLatestByComponentIDs, ids, locale, stage)
	if err != nil {
		return nil, err
	}
	out := []Version{}
	if err := q.SelectContext(ctx, &out, query, args...); err != nil {
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
// error (MySQL 1062). We catch that via repository.IsUniqueViolation and
// re-read MAX(version) up to maxSaveAttempts times. A high collision count
// here is application-level pathology (one component being hammered by
// parallel saves), not normal load.
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
			v.Data, v.SourceLocale, v.SourceData, v.CreatedBy, v.CreatedBy,
		)
		if err == nil {
			return nil
		}
		lastErr = err
		if !repository.IsUniqueViolation(err) {
			return err
		}
		// New ID for the retry — the previous attempt's INSERT never committed,
		// so the same UUID would technically be safe, but generating a fresh
		// one is cheap insurance.
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
