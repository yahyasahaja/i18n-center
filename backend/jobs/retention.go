package jobs

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/lapakgaming/i18n-center/database"
	"github.com/lapakgaming/i18n-center/observability"
)

// retentionInterval is how often the soft-delete sweep runs. Like the version
// retention loop it's idempotent — missing a tick is harmless.
const retentionInterval = 6 * time.Hour

// retentionAdvisoryLockKey gates the soft-delete sweep so exactly one pod in
// the replica set runs it per tick. Different key from
// `cleanupAdvisoryLockKey` so the two sweeps can run side-by-side without
// blocking each other.
const retentionAdvisoryLockKey int64 = 0x6931386e72746e6d // i18nrtnm (truncated tag)

// retentionPolicy is one row in the retention table — table name, the column
// we filter on (deleted_at OR updated_at depending on the table), and a TTL.
//
// Tables fall into two shapes:
//
//  1. Soft-delete tables: `WHERE deleted_at < NOW() - ttl`. Once the row is
//     past TTL the data is unrecoverable, so the per-table TTL needs to be
//     long enough for a human to notice + restore. We err generous.
//
//  2. Terminal-state job tables: `WHERE status IN ('completed','failed')
//     AND updated_at < NOW() - ttl`. These have no `deleted_at` column
//     because the job is the record of work, not a user-visible resource.
//     Once they've terminated the row is just history; a week is plenty.
//
// Per-table commentary lives next to each policy below.
type retentionPolicy struct {
	table       string
	filterCol   string
	extraWHERE  string // optional, e.g. status IN ('completed','failed') for jobs
	ttl         time.Duration
	description string
}

// retentionPolicies defines what gets swept. Edit this slice to add or tune
// per-table TTLs — the loop is otherwise table-agnostic.
//
//   - audit_logs: NOT swept. The trail of who-did-what is the recovery story
//     for everything else; deleting it makes accidental data loss harder to
//     reason about. If size becomes a real concern we revisit.
//
//   - applications: 365 days. Long retention because re-creating an app with
//     the same code reuses the slot — keeping deleted apps recoverable for a
//     year matches our "deleted by mistake" recovery window for top-level
//     resources.
//
//   - components / cms_items / tags / pages / users / application_api_keys
//     / application_locale_deploys: 90 days. Standard mid-tier resources.
//
//   - translation_versions / cms_localizations: 30 days. These are versioned
//     by design — keeping every soft-deleted version forever would dominate
//     the table. Active versions are protected by the
//     `cleanupOldVersions` ticker's keep-last-N policy, not by this sweep.
//
//   - add_language_jobs / translate_jobs / cms_translate_jobs: 7 days,
//     terminal-state only. We keep recent successes/failures for
//     observability (dashboard, debugging) but a week is plenty.
var retentionPolicies = []retentionPolicy{
	{table: "application_api_keys", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted API keys"},
	{table: "application_locale_deploys", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted locale deploys"},
	{table: "users", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted users"},
	{table: "tags", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted tags"},
	{table: "pages", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted pages"},
	{table: "components", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted components"},
	{table: "cms_templates", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted CMS templates"},
	{table: "cms_items", filterCol: "deleted_at", ttl: 90 * 24 * time.Hour, description: "soft-deleted CMS items"},

	{table: "applications", filterCol: "deleted_at", ttl: 365 * 24 * time.Hour, description: "soft-deleted applications"},

	{table: "translation_versions", filterCol: "deleted_at", ttl: 30 * 24 * time.Hour, description: "soft-deleted translation versions"},
	{table: "cms_localizations", filterCol: "deleted_at", ttl: 30 * 24 * time.Hour, description: "soft-deleted CMS localizations"},

	{
		table:       "add_language_jobs",
		filterCol:   "updated_at",
		extraWHERE:  "status IN ('completed','failed')",
		ttl:         7 * 24 * time.Hour,
		description: "terminal add-language jobs",
	},
	{
		table:       "translate_jobs",
		filterCol:   "updated_at",
		extraWHERE:  "status IN ('completed','failed')",
		ttl:         7 * 24 * time.Hour,
		description: "terminal translate jobs",
	},
	{
		table:       "cms_translate_jobs",
		filterCol:   "updated_at",
		extraWHERE:  "status IN ('completed','failed')",
		ttl:         7 * 24 * time.Hour,
		description: "terminal CMS translate jobs",
	},
}

// RunRetentionTicker runs the soft-delete + terminal-job retention sweep on
// a periodic ticker. Returns when ctx is cancelled (SIGTERM path).
//
// Stateless K8s safety: guarded by `pg_try_advisory_lock` so exactly one pod
// runs it per tick. Same shape as RunCleanupTicker — see that comment for
// the full rationale.
func RunRetentionTicker(ctx context.Context) {
	t := time.NewTicker(retentionInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tickRetention(ctx)
		}
	}
}

func tickRetention(ctx context.Context) {
	var got bool
	if err := database.SQLX.GetContext(ctx, &got, "SELECT pg_try_advisory_lock($1)", retentionAdvisoryLockKey); err != nil {
		observability.Logger.Warn("retention advisory_lock acquire failed", zap.Error(err))
		return
	}
	if !got {
		return
	}
	defer func() {
		if _, err := database.SQLX.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", retentionAdvisoryLockKey); err != nil {
			observability.Logger.Warn("retention advisory_unlock failed", zap.Error(err))
		}
	}()

	start := time.Now()
	totalDeleted := int64(0)
	for _, p := range retentionPolicies {
		select {
		case <-ctx.Done():
			return
		default:
		}
		deleted, err := sweepPolicy(ctx, p)
		if err != nil {
			observability.Logger.Warn("retention sweep failed",
				zap.String("table", p.table),
				zap.String("description", p.description),
				zap.Duration("ttl", p.ttl),
				zap.Error(err),
			)
			continue
		}
		if deleted > 0 {
			observability.Logger.Info("retention sweep complete",
				zap.String("table", p.table),
				zap.String("description", p.description),
				zap.Duration("ttl", p.ttl),
				zap.Int64("deleted", deleted),
			)
			totalDeleted += deleted
		}
	}
	observability.Logger.Info("retention tick complete",
		zap.Int64("total_deleted", totalDeleted),
		zap.Duration("duration", time.Since(start)),
	)
}

// sweepPolicy issues one DELETE per table. The TTL is interpolated as a
// Postgres interval string — never user-supplied, so the format-string
// concat here is safe.
//
// We pass the TTL as a parameter rather than embedding it in the WHERE
// directly so the plan is cacheable across ticks (each TTL becomes a
// constant from Postgres's view, the same way `cleanupOldVersions` does it
// for keepLastN).
func sweepPolicy(ctx context.Context, p retentionPolicy) (int64, error) {
	seconds := int64(p.ttl.Seconds())
	where := fmt.Sprintf("%s IS NOT NULL AND %s < NOW() - ($1 || ' seconds')::INTERVAL", p.filterCol, p.filterCol)
	// Terminal job tables have no deleted_at — use the column directly.
	if p.extraWHERE != "" {
		where = fmt.Sprintf("%s AND %s < NOW() - ($1 || ' seconds')::INTERVAL", p.extraWHERE, p.filterCol)
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", p.table, where)
	result, err := database.SQLX.ExecContext(ctx, query, fmt.Sprintf("%d", seconds))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
