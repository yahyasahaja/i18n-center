package handlers

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
	// Point the cache client at a non-existent address so cache.Get always returns an error
	// (cache miss) rather than panicking on a nil client.
	cache.Client = redis.NewClient(&redis.Options{Addr: "localhost:0"})
}

// newMockDB creates BOTH a *gorm.DB and a *sqlx.DB backed by the same sqlmock
// connection, so tests can match SQL emitted by either the legacy GORM path
// or the new repository layer through a single sqlmock instance.
//
// Uses QueryMatcherRegexp so test expectations can use regex patterns against
// either GORM's quoted-table SQL or the repository's plain-table SQL.
func newMockDB(t *testing.T) (*gorm.DB, *sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open gorm with sqlmock: %v", err)
	}

	xdb := sqlx.NewDb(sqlDB, "postgres")
	return db, xdb, mock
}

// withMockDB swaps in BOTH database.DB (GORM) and database.SQLX (sqlx).
// Pass the same pair returned by newMockDB. Originals restored on cleanup.
func withMockDB(t *testing.T, db *gorm.DB, xdb *sqlx.DB) {
	t.Helper()
	origGorm := database.DB
	origSqlx := database.SQLX
	database.DB = db
	database.SQLX = xdb
	t.Cleanup(func() {
		database.DB = origGorm
		database.SQLX = origSqlx
	})
}

// newMockAuditService returns a configured MockAuditServicer.
func newMockAuditService() *mocks.MockAuditServicer {
	m := &mocks.MockAuditServicer{}
	return m
}

// testRouter creates a minimal Gin engine for testing.
func testRouter(handlers ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	for _, h := range handlers {
		r.Use(h)
	}
	return r
}
