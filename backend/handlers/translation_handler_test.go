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
	"github.com/your-org/i18n-center/middleware"
)

func setupTranslationHandler(t *testing.T) *TranslationHandler {
	t.Helper()
	db, _ := newMockDB(t)
	withMockDB(t, db)
	return NewTranslationHandler()
}

func setupTranslationHandlerWithMock(t *testing.T) (*TranslationHandler, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := newMockDB(t)
	withMockDB(t, db)
	return NewTranslationHandler(), mock
}

func TestTranslationHandler_ValidationPaths(t *testing.T) {
	h := setupTranslationHandler(t)
	r := gin.New()

	r.GET("/components/:id/translations", h.GetTranslation)
	r.GET("/translations/bulk", h.GetMultipleTranslations)
	r.GET("/applications/:id/translations/by-tag/:tagCode", h.GetTranslationsByTag)
	r.GET("/applications/:id/translations/by-page/:pageCode", h.GetTranslationsByPage)
	r.POST("/components/:id/translations", h.SaveTranslation)
	r.POST("/components/:id/translations/revert", h.RevertTranslation)
	r.POST("/components/:id/translations/deploy", h.DeployTranslation)
	r.POST("/components/:id/translations/auto-translate", h.AutoTranslate)
	r.POST("/components/:id/translations/backfill", h.BackfillTranslations)
	r.GET("/translate-jobs/:job_id", h.GetTranslateJobStatus)
	r.GET("/components/:id/translate-jobs", h.ListComponentTranslateJobs)
	r.GET("/components/:id/translations/compare", h.GetVersionComparison)
	r.GET("/components/:id/translations/versions", h.ListVersions)

	cases := []struct {
		name   string
		method string
		path   string
		body   interface{}
		code   int
	}{
		{"GetTranslation_InvalidUUID", http.MethodGet, "/components/not-uuid/translations?locale=en&stage=draft", nil, http.StatusBadRequest},
		{"GetMultipleTranslations_MissingQuery", http.MethodGet, "/translations/bulk", nil, http.StatusBadRequest},
		{"GetMultipleTranslations_ComponentCodesNeedsAppCode", http.MethodGet, "/translations/bulk?component_codes=header", nil, http.StatusBadRequest},
		{"GetMultipleTranslations_InvalidComponentID", http.MethodGet, "/translations/bulk?component_ids=abc", nil, http.StatusBadRequest},
		{"GetTranslationsByTag_InvalidAppID", http.MethodGet, "/applications/not-uuid/translations/by-tag/header", nil, http.StatusBadRequest},
		{"GetTranslationsByTag_EmptyTag", http.MethodGet, "/applications/" + uuid.New().String() + "/translations/by-tag/%20", nil, http.StatusBadRequest},
		{"GetTranslationsByPage_InvalidAppID", http.MethodGet, "/applications/not-uuid/translations/by-page/home", nil, http.StatusBadRequest},
		{"GetTranslationsByPage_EmptyPage", http.MethodGet, "/applications/" + uuid.New().String() + "/translations/by-page/%20", nil, http.StatusBadRequest},
		{"SaveTranslation_InvalidComponentID", http.MethodPost, "/components/not-uuid/translations", map[string]any{}, http.StatusBadRequest},
		{"SaveTranslation_BadBody", http.MethodPost, "/components/" + uuid.New().String() + "/translations", map[string]any{"locale": "en"}, http.StatusBadRequest},
		{"RevertTranslation_MissingLocale", http.MethodPost, "/components/" + uuid.New().String() + "/translations/revert", nil, http.StatusBadRequest},
		{"RevertTranslation_InvalidComponentID", http.MethodPost, "/components/not-uuid/translations/revert?locale=en", nil, http.StatusBadRequest},
		{"DeployTranslation_InvalidComponentID", http.MethodPost, "/components/not-uuid/translations/deploy", map[string]string{}, http.StatusBadRequest},
		{"DeployTranslation_BadBody", http.MethodPost, "/components/" + uuid.New().String() + "/translations/deploy", map[string]any{"locale": "en"}, http.StatusBadRequest},
		{"AutoTranslate_InvalidComponentID", http.MethodPost, "/components/not-uuid/translations/auto-translate", map[string]any{}, http.StatusBadRequest},
		{"AutoTranslate_BadBody", http.MethodPost, "/components/" + uuid.New().String() + "/translations/auto-translate", map[string]any{"source_locale": "en"}, http.StatusBadRequest},
		{"BackfillTranslations_InvalidComponentID", http.MethodPost, "/components/not-uuid/translations/backfill", map[string]any{}, http.StatusBadRequest},
		{"BackfillTranslations_BadBody", http.MethodPost, "/components/" + uuid.New().String() + "/translations/backfill", map[string]any{"source_locale": "en"}, http.StatusBadRequest},
		{"GetTranslateJobStatus_InvalidJobID", http.MethodGet, "/translate-jobs/not-uuid", nil, http.StatusBadRequest},
		{"ListComponentTranslateJobs_InvalidComponentID", http.MethodGet, "/components/not-uuid/translate-jobs", nil, http.StatusBadRequest},
		{"GetVersionComparison_MissingLocale", http.MethodGet, "/components/" + uuid.New().String() + "/translations/compare", nil, http.StatusBadRequest},
		{"GetVersionComparison_InvalidComponentID", http.MethodGet, "/components/not-uuid/translations/compare?locale=en", nil, http.StatusBadRequest},
		{"GetVersionComparison_InvalidVersionA", http.MethodGet, "/components/" + uuid.New().String() + "/translations/compare?locale=en&version_a=abc&version_b=1", nil, http.StatusBadRequest},
		{"ListVersions_MissingLocale", http.MethodGet, "/components/" + uuid.New().String() + "/translations/versions", nil, http.StatusBadRequest},
		{"ListVersions_InvalidComponentID", http.MethodGet, "/components/not-uuid/translations/versions?locale=en", nil, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyBytes []byte
			if tc.body != nil {
				bodyBytes, _ = json.Marshal(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBuffer(bodyBytes))
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.code, w.Code)
		})
	}
}

func TestTranslationHandler_ContextHelpers(t *testing.T) {
	h := setupTranslationHandler(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	id := uuid.New()
	c.Set("user_id", id.String())
	c.Set("username", "translator")
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("User-Agent", "ua-translation")

	userID, username := h.getCurrentUser(c)
	assert.Equal(t, id, userID)
	assert.Equal(t, "translator", username)

	ip, ua := h.getClientInfo(c)
	assert.NotEmpty(t, ip)
	assert.Equal(t, "ua-translation", ua)
}

func TestTranslationHandler_BehavioralBranches(t *testing.T) {
	h, mock := setupTranslationHandlerWithMock(t)
	r := gin.New()
	r.GET("/components/:id/translations", h.GetTranslation)
	r.POST("/components/:id/translations", h.SaveTranslation)
	r.POST("/components/:id/translations/auto-translate", h.AutoTranslate)
	r.GET("/translate-jobs/:job_id", h.GetTranslateJobStatus)

	t.Run("GetTranslation_NotFound", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(sqlmock.AnyArg(), "en", "production", true, 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "component_id", "locale", "stage", "version", "data", "is_active",
				"created_by", "updated_by", "created_at", "updated_at", "deleted_at",
			}))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/components/"+uuid.New().String()+"/translations?locale=en&stage=production", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("SaveTranslation_ComponentNotFound", func(t *testing.T) {
		componentID := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "components"`).
			WithArgs(componentID, 1).
			WillReturnRows(sqlmock.NewRows(componentColumns()))
		body, _ := json.Marshal(map[string]any{
			"locale": "id",
			"stage":  "draft",
			"data":   map[string]any{"title": "Halo"},
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/components/"+componentID.String()+"/translations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("AutoTranslate_ComponentNotFound", func(t *testing.T) {
		componentID := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "components"`).
			WithArgs(componentID, 1).
			WillReturnRows(sqlmock.NewRows(componentColumns()))
		body, _ := json.Marshal(map[string]any{
			"source_locale": "en",
			"target_locale": "id",
			"stage":         "draft",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/components/"+componentID.String()+"/translations/auto-translate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GetTranslateJobStatus_NotFound", func(t *testing.T) {
		jobID := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "translate_jobs"`).
			WithArgs(jobID, 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "component_id", "job_type", "source_locale", "target_locales",
				"status", "error_message", "error_detail", "claimed_by", "created_by", "created_at", "updated_at", "deleted_at",
			}))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/translate-jobs/"+jobID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestTranslationHandler_APIKeyRestrictedBranches(t *testing.T) {
	h, mock := setupTranslationHandlerWithMock(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.CtxAPIKeyApplicationID, uuid.New().String())
	})
	r.GET("/translations/bulk", h.GetMultipleTranslations)
	r.GET("/applications/:id/translations/by-tag/:tagCode", h.GetTranslationsByTag)
	r.GET("/applications/:id/translations/by-page/:pageCode", h.GetTranslationsByPage)

	t.Run("GetMultipleTranslations_ComponentIDsRejectedForAPIKey", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/translations/bulk?component_ids="+uuid.New().String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetTranslationsByTag_APIKeyForbidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+uuid.New().String()+"/translations/by-tag/header", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("GetTranslationsByPage_APIKeyForbidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+uuid.New().String()+"/translations/by-page/home", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("GetMultipleTranslations_ComponentCodesWithUnknownApp", func(t *testing.T) {
		// API-key branch validates application by code and returns not found if missing.
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs("missing-app", 1).
			WillReturnRows(sqlmock.NewRows(appColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/translations/bulk?application_code=missing-app&component_codes=header", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
