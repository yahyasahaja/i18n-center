package auth

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lapakgaming/i18n-center/database"
)

// setupAPIKeyDB wires a sqlmock-backed *sqlx.DB into database.SQLX so
// ValidateAPIKey hits the mock instead of a real Postgres. Restored on
// test cleanup.
func setupAPIKeyDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	xdb := sqlx.NewDb(sqlDB, "postgres")

	old := database.SQLX
	database.SQLX = xdb
	t.Cleanup(func() {
		database.SQLX = old
		_ = sqlDB.Close()
		require.NoError(t, mock.ExpectationsWereMet())
	})

	return mock
}

func TestHashKey_Deterministic(t *testing.T) {
	h1 := HashKey("sk_example")
	h2 := HashKey("sk_example")
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64)
}

func TestValidateAPIKey_InvalidInput(t *testing.T) {
	id, ok := ValidateAPIKey("")
	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)

	id, ok = ValidateAPIKey("not-prefixed")
	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)
}

func TestValidateAPIKey_NotFound(t *testing.T) {
	mock := setupAPIKeyDB(t)
	key := APIKeyPrefix + "missing"

	// Repository selects id, application_id, key_hash, key_prefix, name, created_at
	// — no deleted_at because the WHERE clause already filters it out.
	cols := []string{"id", "application_id", "key_hash", "key_prefix", "name", "created_at"}
	mock.ExpectQuery(`SELECT .*FROM application_api_keys`).
		WithArgs(HashKey(key)).
		WillReturnRows(sqlmock.NewRows(cols))

	id, ok := ValidateAPIKey(key)
	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)
}

func TestValidateAPIKey_Success(t *testing.T) {
	mock := setupAPIKeyDB(t)
	appID := uuid.New()
	key := APIKeyPrefix + "valid_key_123"

	cols := []string{"id", "application_id", "key_hash", "key_prefix", "name", "created_at"}
	mock.ExpectQuery(`SELECT .*FROM application_api_keys`).
		WithArgs(HashKey(key)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uuid.New(), appID, HashKey(key), APIKeyPrefix+"valid", "test", time.Now(),
		))

	id, ok := ValidateAPIKey("   " + key + "   ")
	assert.True(t, ok)
	assert.Equal(t, appID, id)
}
