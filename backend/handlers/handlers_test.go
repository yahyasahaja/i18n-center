package handlers

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/mocks"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
	// Point the cache client at a non-existent address so cache.Get always returns an error
	// (cache miss) rather than panicking on a nil client.
	cache.Client = redis.NewClient(&redis.Options{Addr: "localhost:0"})
}

// newMockDB creates a *gorm.DB backed by go-sqlmock.
// Uses QueryMatcherRegexp so GORM-generated SQL patterns can be matched with regexes.
func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

	return db, mock
}

// withMockDB sets database.DB to the provided mock and restores original on cleanup.
func withMockDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	original := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = original })
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
