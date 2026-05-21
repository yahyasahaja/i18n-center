package jobs

import (
	"context"
	"time"

	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/observability"
	"go.uber.org/zap"
)

// cleanupInterval is how often the version-retention sweep runs. The job is
// idempotent so missing a tick is harmless — the next tick covers any backlog.
const cleanupInterval = 5 * time.Minute

// RunCleanupTicker runs the translation_versions retention sweep on a periodic
// ticker. Returns when ctx is cancelled (SIGTERM path). The first sweep runs
// after one interval, not at startup, so cold-boot doesn't compete with
// migration/index work.
//
// This replaces the previous behavior of calling CleanupOldVersions from inside
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
			start := time.Now()
			if err := database.CleanupOldVersions(); err != nil {
				observability.Logger.Warn("CleanupOldVersions failed",
					zap.Error(err),
					zap.Duration("duration", time.Since(start)),
				)
				continue
			}
			observability.Logger.Info("CleanupOldVersions completed",
				zap.Duration("duration", time.Since(start)),
			)
		}
	}
}
