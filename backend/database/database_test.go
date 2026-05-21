package database

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupDBMock wires a sqlmock-backed *gorm.DB into the package-level DB so
// tests can assert on the exact SQL that would have been executed.
func setupDBMock(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	old := DB
	DB = gdb
	t.Cleanup(func() {
		DB = old
		_ = sqlDB.Close()
		require.NoError(t, mock.ExpectationsWereMet())
	})

	return mock
}

func TestSetupObservabilityCallbacks_NilDB(t *testing.T) {
	old := DB
	DB = nil
	t.Cleanup(func() { DB = old })
	setupObservabilityCallbacks()
}

func TestCleanupOldVersions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := setupDBMock(t)
		mock.ExpectExec(`DELETE FROM translation_versions`).WillReturnResult(sqlmock.NewResult(0, 1))
		assert.NoError(t, CleanupOldVersions())
	})

	t.Run("exec error", func(t *testing.T) {
		mock := setupDBMock(t)
		mock.ExpectExec(`DELETE FROM translation_versions`).WillReturnError(assert.AnError)
		assert.Error(t, CleanupOldVersions())
	})
}

// The previous tests (TestMigrateCodeFields, TestDropTagPageNameColumns,
// and the various ensure*Indexes tests) were removed when the bespoke
// in-binary migration helpers were squashed into backend/migrations/00001_init.sql.
// Schema is now applied via `i18n-center-migrate up` — see migrations/README.md.
