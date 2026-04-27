package database

import (
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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

func TestDropTagPageNameColumns(t *testing.T) {
	t.Run("no name columns", func(t *testing.T) {
		mock := setupDBMock(t)
		q := `table_schema = 'public' AND table_name = \$1 AND column_name = 'name'`
		mock.ExpectQuery(q).WithArgs("tags").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery(q).WithArgs("pages").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		assert.NoError(t, dropTagPageNameColumns())
	})

	t.Run("drop error", func(t *testing.T) {
		mock := setupDBMock(t)
		q := `table_schema = 'public' AND table_name = \$1 AND column_name = 'name'`
		mock.ExpectQuery(q).WithArgs("tags").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(`ALTER TABLE tags DROP COLUMN IF EXISTS name`).WillReturnError(errors.New("cannot drop"))
		assert.Error(t, dropTagPageNameColumns())
	})
}

func TestMigrateCodeFields(t *testing.T) {
	t.Run("full success with index fallback", func(t *testing.T) {
		mock := setupDBMock(t)

		mock.ExpectQuery(`table_name = 'applications' AND column_name = 'code'`).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectExec(`ALTER TABLE applications ADD COLUMN code text`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE applications`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`is_nullable = 'NO'.*table_name = 'applications' AND column_name = 'code'`).
			WillReturnRows(sqlmock.NewRows([]string{"is_nullable"}).AddRow(false))
		mock.ExpectExec(`ALTER TABLE applications ALTER COLUMN code SET NOT NULL`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectQuery(`table_name = 'components' AND column_name = 'code'`).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectExec(`ALTER TABLE components ADD COLUMN code text`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE components`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`is_nullable = 'NO'.*table_name = 'components' AND column_name = 'code'`).
			WillReturnRows(sqlmock.NewRows([]string{"is_nullable"}).AddRow(false))
		mock.ExpectExec(`ALTER TABLE components ALTER COLUMN code SET NOT NULL`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectQuery(`tablename = 'components'.*indexname = 'components_code_key'`).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(`DROP INDEX IF EXISTS components_code_key`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectQuery(`tablename = 'components'.*indexname = 'idx_component_app_code'`).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectExec(`CREATE UNIQUE INDEX idx_component_app_code ON components\(application_id, code\) WHERE deleted_at IS NULL`).
			WillReturnError(errors.New("unsupported partial index"))
		mock.ExpectExec(`CREATE UNIQUE INDEX idx_component_app_code ON components\(application_id, code\)`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, migrateCodeFields())
	})

	t.Run("first query error", func(t *testing.T) {
		mock := setupDBMock(t)
		mock.ExpectQuery(`table_name = 'applications' AND column_name = 'code'`).
			WillReturnError(assert.AnError)
		assert.Error(t, migrateCodeFields())
	})
}
