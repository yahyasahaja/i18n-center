package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/your-org/i18n-center/auth"
	"github.com/your-org/i18n-center/database"
)

// withMockDB swaps database.SQLX for a sqlmock-backed *sqlx.DB so the
// middleware's ValidateAPIKey path can run against deterministic expectations
// instead of a live Postgres. Restored on test cleanup.
func withMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	assert.NoError(t, err)
	xdb := sqlx.NewDb(sqlDB, "postgres")
	orig := database.SQLX
	database.SQLX = xdb
	t.Cleanup(func() {
		database.SQLX = orig
		_ = sqlDB.Close()
	})
	return mock
}

func TestTranslationAuthMiddleware_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TranslationAuthMiddleware())
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTranslationAuthMiddleware_InvalidAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := withMockDB(t)
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	r := gin.New()
	r.Use(TranslationAuthMiddleware())
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-API-Key", "sk_invalid")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTranslationAuthMiddleware_InvalidJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "test-secret")
	r := gin.New()
	r.Use(TranslationAuthMiddleware())
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTranslationAuthMiddleware_ValidJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := auth.GenerateToken(uuid.New(), "tester", "operator")
	assert.NoError(t, err)

	r := gin.New()
	r.Use(TranslationAuthMiddleware())
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireTranslationAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Allows API key context", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(CtxAPIKeyApplicationID, uuid.New().String())
			c.Next()
		})
		r.Use(RequireTranslationAccess("super_admin", "operator"))
		r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Unauthorized when no auth", func(t *testing.T) {
		r := gin.New()
		r.Use(RequireTranslationAccess("super_admin", "operator"))
		r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Forbidden on wrong role", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("role", "viewer")
			c.Next()
		})
		r.Use(RequireTranslationAccess("super_admin", "operator"))
		r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestGetAPIKeyApplicationID(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, uuid.Nil, GetAPIKeyApplicationID(c))

	id := uuid.New()
	c.Set(CtxAPIKeyApplicationID, id.String())
	assert.Equal(t, id, GetAPIKeyApplicationID(c))
}
