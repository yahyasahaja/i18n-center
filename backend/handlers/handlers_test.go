package handlers

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"

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

// newMockDB creates a sqlmock-backed *sqlx.DB. All repositories use sqlx
// after Commit I, so this is the only handle tests need to swap in.
//
// QueryMatcherRegexp lets tests assert against the actual SQL emitted by the
// repository layer (raw text, $1/$2/... placeholders, no quoted identifiers).
func newMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	xdb := sqlx.NewDb(sqlDB, "postgres")
	return xdb, mock
}

// withMockDB swaps in database.SQLX, restoring the original on test cleanup.
func withMockDB(t *testing.T, xdb *sqlx.DB) {
	t.Helper()
	orig := database.SQLX
	database.SQLX = xdb
	t.Cleanup(func() {
		database.SQLX = orig
	})
}

// newMockAuditService returns a MockAuditServicer with permissive Maybe()
// matchers wired for every Log* method. Audit calls from a handler are
// best-effort — they must never affect the response — so this baseline
// catches every signature without forcing per-test setup. Tests that need
// to assert audit content can still attach explicit On() expectations.
func newMockAuditService() *mocks.MockAuditServicer {
	m := &mocks.MockAuditServicer{}
	m.On("LogCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	m.On("LogUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	m.On("LogDelete", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	m.On("LogAction", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
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
