package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/your-org/i18n-center/database"
)

func TestTagPageAndUtilityHandlers_ValidationPaths(t *testing.T) {
	db, xdb, mock := newMockDB(t)
	withMockDB(t, db, xdb)

	tagH := NewTagHandler()
	pageH := NewPageHandler()
	apiKeyH := NewAPIKeyHandler()
	auditH := NewAuditHandler()
	exportH := NewExportHandler()
	importH := NewImportHandler()
	bootstrapH := NewBootstrapHandler()
	healthH := NewHealthHandler()

	r := gin.New()
	r.GET("/applications/:id/tags", tagH.ListByApplication)
	r.POST("/applications/:id/tags", tagH.Create)
	r.GET("/tags/:id", tagH.Get)
	r.GET("/applications/:id/pages", pageH.ListByApplication)
	r.POST("/applications/:id/pages", pageH.Create)
	r.GET("/pages/:id", pageH.Get)
	r.POST("/applications/:id/api-keys", apiKeyH.Create)
	r.GET("/applications/:id/api-keys", apiKeyH.List)
	r.DELETE("/applications/:id/api-keys/:key_id", apiKeyH.Delete)
	r.GET("/audit/logs", auditH.GetAuditLogs)
	r.GET("/audit/history/:resource_type/:resource_id", auditH.GetResourceHistory)
	r.GET("/applications/:id/export", exportH.ExportApplication)
	r.GET("/components/:id/export", exportH.ExportComponent)
	r.POST("/components/:id/import", importH.ImportComponent)
	r.POST("/applications/:id/bootstrap", bootstrapH.BootstrapApplication)
	r.GET("/health", healthH.HealthCheck)
	r.GET("/ready", healthH.ReadinessCheck)
	r.GET("/live", healthH.LivenessCheck)

	tests := []struct {
		name   string
		method string
		path   string
		body   interface{}
		code   int
	}{
		{"TagList_InvalidAppID", http.MethodGet, "/applications/not-uuid/tags", nil, http.StatusBadRequest},
		{"TagCreate_InvalidAppID", http.MethodPost, "/applications/not-uuid/tags", map[string]string{"code": "x"}, http.StatusBadRequest},
		{"TagGet_NotFound", http.MethodGet, "/tags/" + uuid.New().String(), nil, http.StatusNotFound},
		{"PageList_InvalidAppID", http.MethodGet, "/applications/not-uuid/pages", nil, http.StatusBadRequest},
		{"PageCreate_InvalidAppID", http.MethodPost, "/applications/not-uuid/pages", map[string]string{"code": "x"}, http.StatusBadRequest},
		{"PageGet_NotFound", http.MethodGet, "/pages/" + uuid.New().String(), nil, http.StatusNotFound},
		{"APIKeyCreate_InvalidAppID", http.MethodPost, "/applications/not-uuid/api-keys", map[string]string{"name": "k"}, http.StatusBadRequest},
		{"APIKeyList_InvalidAppID", http.MethodGet, "/applications/not-uuid/api-keys", nil, http.StatusBadRequest},
		{"APIKeyDelete_InvalidAppID", http.MethodDelete, "/applications/not-uuid/api-keys/" + uuid.New().String(), nil, http.StatusBadRequest},
		{"APIKeyDelete_InvalidKeyID", http.MethodDelete, "/applications/" + uuid.New().String() + "/api-keys/not-uuid", nil, http.StatusBadRequest},
		{"AuditLogs_InvalidResourceID", http.MethodGet, "/audit/logs?resource_id=not-uuid", nil, http.StatusBadRequest},
		{"AuditLogs_InvalidUserID", http.MethodGet, "/audit/logs?user_id=not-uuid", nil, http.StatusBadRequest},
		{"AuditLogs_InvalidLimit", http.MethodGet, "/audit/logs?limit=abc", nil, http.StatusBadRequest},
		{"AuditHistory_InvalidResourceID", http.MethodGet, "/audit/history/application/not-uuid", nil, http.StatusBadRequest},
		{"ExportApplication_InvalidAppID", http.MethodGet, "/applications/not-uuid/export", nil, http.StatusBadRequest},
		{"ExportComponent_InvalidComponentID", http.MethodGet, "/components/not-uuid/export", nil, http.StatusBadRequest},
		{"ImportComponent_MissingLocale", http.MethodPost, "/components/" + uuid.New().String() + "/import", map[string]any{}, http.StatusBadRequest},
		{"ImportComponent_InvalidComponentID", http.MethodPost, "/components/not-uuid/import?locale=en", map[string]any{}, http.StatusBadRequest},
		{"ImportComponent_BadBody", http.MethodPost, "/components/" + uuid.New().String() + "/import?locale=en", map[string]any{}, http.StatusBadRequest},
		{"Bootstrap_InvalidAppID", http.MethodPost, "/applications/not-uuid/bootstrap", map[string]any{"data": map[string]any{}}, http.StatusBadRequest},
		{"Health_NoDB_Degraded", http.MethodGet, "/health", nil, http.StatusServiceUnavailable},
		{"Readiness_NoDB_NotReady", http.MethodGet, "/ready", nil, http.StatusServiceUnavailable},
		{"Liveness_Alive", http.MethodGet, "/live", nil, http.StatusOK},
	}

	// Ensure a few "not found" handlers get a DB lookup miss.
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id"})) // tag get
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id"})) // page get

	origDB := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = origDB })

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "Health_NoDB_Degraded" || tc.name == "Readiness_NoDB_NotReady" || tc.name == "Liveness_Alive" {
				database.DB = nil
			} else {
				database.DB = db
			}
			var payload []byte
			if tc.body != nil {
				payload, _ = json.Marshal(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBuffer(payload))
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.code, w.Code)
		})
	}
}

func TestParseInt(t *testing.T) {
	v, err := parseInt("42")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)

	_, err = parseInt("x")
	assert.Error(t, err)
}
