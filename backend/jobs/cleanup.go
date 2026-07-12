package jobs

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"

	"github.com/lapakgaming/i18n-center/database"
	"github.com/lapakgaming/i18n-center/observability"
	"github.com/lapakgaming/i18n-center/repository/translation"
)

// cleanupInterval is how often the version-retention sweep runs. The job is
// idempotent so missing a tick is harmless — the next tick covers any backlog.
const cleanupInterval = 5 * time.Minute

// keepVersionsPerCell is how many of the most recent translation_versions
// rows we retain per (component_id, locale, stage). Lower = cheaper retention
// sweep + smaller table; higher = more revert headroom. 50 strikes a balance:
// covers a few months of typical edit cadence per cell without bloating.
const keepVersionsPerCell = 50

// cleanupAdvisoryLockKey is the MySQL named-lock key that gates the retention
// sweep. Only ONE pod across the K8s replica set holds it at any moment;
// everyone else's tick becomes a cheap no-op GET_LOCK + RELEASE_LOCK. Without
// this every replica would run the same DELETE every 5 min — wasted work plus
// occasional self-deadlock under contention. Must be identical across every
// pod so they all contend for the same lock.
const cleanupAdvisoryLockKey = "i18n-center:cleanup"

// translationsRepo is the package-level repo handle the cleanup ticker uses
// to run its retention sweep. Same pattern as the worker.
var translationsRepo = translation.New()

// RunCleanupTicker runs the translation_versions retention sweep on a periodic
// ticker. Returns when ctx is cancelled (SIGTERM path). The first sweep runs
// after one interval, not at startup, so cold-boot doesn't compete with
// migration/index work.
//
// Stateless K8s safety: guarded by a MySQL session-scoped named lock
// (GET_LOCK) so exactly one pod across the replica set runs the sweep on any
// given tick. Lock holders that crash mid-sweep auto-release when their MySQL
// session closes — no manual recovery needed.
//
// Replaces the previous behavior of calling CleanupOldVersions from inside
// every saveVersion call, which scaled poorly: each save triggered a full
// ROW_NUMBER OVER (...) sort of translation_versions whose cost grew with the
// table, even though retention only needs to run periodically.
func RunCleanupTicker(ctx context.Context) {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tickCleanup(ctx)
		}
	}
}

func tickCleanup(ctx context.Context) {
	// GET_LOCK(name, 0) returns 1 immediately on success, 0 if held elsewhere,
	// NULL on error. Timeout=0 means no blocking — losers just skip this tick.
	var got sql.NullInt64
	if err := database.SQLX.GetContext(ctx, &got, "SELECT GET_LOCK(?, 0)", cleanupAdvisoryLockKey); err != nil {
		observability.Logger.Warn("cleanup advisory_lock acquire failed", zap.Error(err))
		return
	}
	if !got.Valid || got.Int64 != 1 {
		// Another pod is sweeping (or GET_LOCK returned NULL); nothing to do.
		return
	}
	defer func() {
		if _, err := database.SQLX.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", cleanupAdvisoryLockKey); err != nil {
			observability.Logger.Warn("cleanup advisory_unlock failed", zap.Error(err))
		}
	}()

	start := time.Now()
	deleted, err := translationsRepo.DeleteOldVersions(ctx, database.SQLX, keepVersionsPerCell)
	if err != nil {
		observability.Logger.Warn("CleanupOldVersions failed",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
		)
		return
	}
	observability.Logger.Info("CleanupOldVersions completed",
		zap.Int64("deleted", deleted),
		zap.Duration("duration", time.Since(start)),
	)
}
