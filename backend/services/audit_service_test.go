package services

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-org/i18n-center/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory sqlmock DB and wires it into database.DB.
func setupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	original := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = original })

	return db, mock
}

// ---- CompareValues pure-logic tests ----

func TestCompareValues_DetectsChangedField(t *testing.T) {
	before := map[string]interface{}{"name": "old", "active": true}
	after := map[string]interface{}{"name": "new", "active": true}

	diff := CompareValues(before, after)
	require.Contains(t, diff, "name")

	nameDiff, ok := diff["name"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "old", nameDiff["before"])
	assert.Equal(t, "new", nameDiff["after"])

	// Unchanged field should not appear in diff
	assert.NotContains(t, diff, "active")
}

func TestCompareValues_DetectsNewField(t *testing.T) {
	before := map[string]interface{}{"name": "foo"}
	after := map[string]interface{}{"name": "foo", "code": "bar"}

	diff := CompareValues(before, after)
	require.Contains(t, diff, "code")

	codeDiff, ok := diff["code"].(map[string]interface{})
	require.True(t, ok)
	assert.Nil(t, codeDiff["before"])
	assert.Equal(t, "bar", codeDiff["after"])
}

func TestCompareValues_DetectsDeletedField(t *testing.T) {
	before := map[string]interface{}{"name": "foo", "extra": "gone"}
	after := map[string]interface{}{"name": "foo"}

	diff := CompareValues(before, after)
	require.Contains(t, diff, "extra")

	extraDiff, ok := diff["extra"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "gone", extraDiff["before"])
	assert.Nil(t, extraDiff["after"])
}

func TestCompareValues_NoDiffWhenEqual(t *testing.T) {
	data := map[string]interface{}{"x": 1, "y": "hello"}
	diff := CompareValues(data, data)
	assert.Empty(t, diff)
}

func TestCompareValues_EmptyInputs(t *testing.T) {
	diff := CompareValues(nil, nil)
	assert.NotNil(t, diff)
}

// ---- LogCreate with sqlmock ----

func TestLogCreate_InsertAuditLog(t *testing.T) {
	_, mock := setupTestDB(t)

	userID := uuid.New()
	resourceID := uuid.New()

	// Expect an INSERT with RETURNING for the audit_logs table
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	svc := NewAuditService()
	err := svc.LogCreate(
		userID, "admin", "application", resourceID, "myapp",
		map[string]interface{}{"name": "test"},
		"127.0.0.1", "Go-test",
	)
	assert.NoError(t, err)
}

// ---- LogUpdate with sqlmock ----

func TestLogUpdate_InsertAuditLog(t *testing.T) {
	_, mock := setupTestDB(t)

	userID := uuid.New()
	resourceID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	svc := NewAuditService()
	err := svc.LogUpdate(
		userID, "admin", "component", resourceID, "header",
		map[string]interface{}{"name": "old"},
		map[string]interface{}{"name": "new"},
		"127.0.0.1", "Go-test",
	)
	assert.NoError(t, err)
}

// ---- LogDelete with sqlmock ----

func TestLogDelete_InsertAuditLog(t *testing.T) {
	_, mock := setupTestDB(t)

	userID := uuid.New()
	resourceID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	svc := NewAuditService()
	err := svc.LogDelete(
		userID, "admin", "user", resourceID, "bob",
		map[string]interface{}{"username": "bob"},
		"127.0.0.1", "Go-test",
	)
	assert.NoError(t, err)
}

// ---- GetAuditLogs ----

func TestGetAuditLogs_ReturnsLogs(t *testing.T) {
	_, mock := setupTestDB(t)

	auditCols := []string{
		"id", "user_id", "username", "action", "resource_type",
		"resource_id", "resource_code", "changes", "ip_address",
		"user_agent", "created_at",
	}
	logID := uuid.New()
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(auditCols).AddRow(
			logID, uuid.Nil, "admin", "CREATE", "application",
			uuid.Nil, "app1", []byte("{}"), "127.0.0.1",
			"Go-test", time.Now(),
		))

	svc := NewAuditService()
	logs, err := svc.GetAuditLogs("application", uuid.Nil, 10)
	assert.NoError(t, err)
	assert.Len(t, logs, 1)
}

// ---- GetAuditLogsByUser ----

func TestGetAuditLogsByUser_ReturnsEmpty(t *testing.T) {
	_, mock := setupTestDB(t)

	auditCols := []string{
		"id", "user_id", "username", "action", "resource_type",
		"resource_id", "resource_code", "changes", "ip_address",
		"user_agent", "created_at",
	}
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(auditCols))

	svc := NewAuditService()
	logs, err := svc.GetAuditLogsByUser(uuid.New(), 10)
	assert.NoError(t, err)
	assert.Empty(t, logs)
}
