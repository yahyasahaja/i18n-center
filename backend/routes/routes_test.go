package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupRoutes_BasicEndpointsAndCORS(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "https://example.test")
	r := SetupRoutes()

	// CORS middleware should handle OPTIONS early.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/applications", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://example.test", w.Header().Get("Access-Control-Allow-Origin"))

	// Public health endpoint should be registered.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/live", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Public auth login endpoint should exist (body omitted => 400 from handler).
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Keep linter happy for imported os in this package context.
	_ = os.Getenv("CORS_ORIGIN")
}
