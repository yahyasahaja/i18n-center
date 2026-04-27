package auth

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupAPIKeyDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	old := database.DB
	database.DB = gdb
	t.Cleanup(func() {
		database.DB = old
		_ = sqlDB.Close()
		require.NoError(t, mock.ExpectationsWereMet())
	})

	return mock
}

func TestHashKey_Deterministic(t *testing.T) {
	h1 := hashKey("sk_example")
	h2 := hashKey("sk_example")
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
	key := models.APIKeyPrefix + "missing"

	cols := []string{"id", "application_id", "key_hash", "key_prefix", "name", "created_at", "deleted_at"}
	mock.ExpectQuery(`SELECT .*FROM "application_api_keys"`).
		WithArgs(hashKey(key), 1).
		WillReturnRows(sqlmock.NewRows(cols))

	id, ok := ValidateAPIKey(key)
	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)
}

func TestValidateAPIKey_Success(t *testing.T) {
	mock := setupAPIKeyDB(t)
	appID := uuid.New()
	key := models.APIKeyPrefix + "valid_key_123"

	cols := []string{"id", "application_id", "key_hash", "key_prefix", "name", "created_at", "deleted_at"}
	mock.ExpectQuery(`SELECT .*FROM "application_api_keys"`).
		WithArgs(hashKey(key), 1).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uuid.New(), appID, hashKey(key), models.APIKeyPrefix+"valid", "test", time.Now(), nil,
		))

	id, ok := ValidateAPIKey("   " + key + "   ")
	assert.True(t, ok)
	assert.Equal(t, appID, id)
}
