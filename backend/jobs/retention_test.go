package jobs

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/your-org/i18n-center/database"
)

// withMockSQLX swaps in a sqlmock-backed *sqlx.DB so retention tests can
// assert on the SQL emitted by sweepPolicy / tickRetention.
func withMockSQLX(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	xdb := sqlx.NewDb(sqlDB, "postgres")
	old := database.SQLX
	database.SQLX = xdb
	t.Cleanup(func() {
		database.SQLX = old
		_ = sqlDB.Close()
	})
	return mock
}

func TestSweepPolicy_SoftDeleteShape(t *testing.T) {
	mock := withMockSQLX(t)
	p := retentionPolicy{
		table:     "components",
		filterCol: "deleted_at",
		ttl:       90 * 24 * time.Hour,
	}
	expected := regexp.QuoteMeta(
		`DELETE FROM components WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - ($1 || ' seconds')::INTERVAL`,
	)
	mock.ExpectExec(expected).
		WithArgs("7776000").
		WillReturnResult(sqlmock.NewResult(0, 3))

	deleted, err := sweepPolicy(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSweepPolicy_TerminalJobShape(t *testing.T) {
	mock := withMockSQLX(t)
	p := retentionPolicy{
		table:      "translate_jobs",
		filterCol:  "updated_at",
		extraWHERE: "status IN ('completed','failed')",
		ttl:        7 * 24 * time.Hour,
	}
	// Terminal-shape WHERE: extraWHERE first, then the TTL clause.
	expected := regexp.QuoteMeta(
		`DELETE FROM translate_jobs WHERE status IN ('completed','failed') AND updated_at < NOW() - ($1 || ' seconds')::INTERVAL`,
	)
	mock.ExpectExec(expected).
		WithArgs("604800").
		WillReturnResult(sqlmock.NewResult(0, 12))

	deleted, err := sweepPolicy(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, int64(12), deleted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSweepPolicy_ExecErrorBubbles(t *testing.T) {
	mock := withMockSQLX(t)
	p := retentionPolicy{table: "pages", filterCol: "deleted_at", ttl: time.Hour}
	mock.ExpectExec(`DELETE FROM pages`).WillReturnError(assert.AnError)

	_, err := sweepPolicy(context.Background(), p)
	assert.Error(t, err)
}

func TestRetentionPolicies_CoverExpectedTables(t *testing.T) {
	// Sanity check that policy edits don't accidentally drop a table we care
	// about. The retention list is the contract; missing entries are silent
	// data growth in production, so a unit test guards the set.
	wantTables := map[string]bool{
		"application_api_keys":        true,
		"application_locale_deploys":  true,
		"users":                       true,
		"tags":                        true,
		"pages":                       true,
		"components":                  true,
		"cms_templates":               true,
		"cms_items":                   true,
		"applications":                true,
		"translation_versions":        true,
		"cms_localizations":           true,
		"add_language_jobs":           true,
		"translate_jobs":              true,
		"cms_translate_jobs":          true,
	}
	got := map[string]bool{}
	for _, p := range retentionPolicies {
		got[p.table] = true
	}
	for table := range wantTables {
		assert.True(t, got[table], "retention policy missing for %s", table)
	}
	// audit_logs is intentionally NOT in the sweep.
	assert.False(t, got["audit_logs"], "audit_logs must NOT be in retention policies — it's the recovery trail")
}
