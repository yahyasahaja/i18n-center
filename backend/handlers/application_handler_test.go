package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/your-org/i18n-center/models"
)

func setupApplicationHandler(t *testing.T) (*ApplicationHandler, sqlmock.Sqlmock) {
	db, mock := newMockDB(t)
	withMockDB(t, db)
	audit := newMockAuditService()
	h := &ApplicationHandler{auditService: audit}
	return h, mock
}

func appColumns() []string {
	return []string{
		"id", "name", "code", "description", "openai_key",
		"enabled_languages", "created_by", "updated_by",
		"created_at", "updated_at", "deleted_at",
	}
}

func appRow(id uuid.UUID, name, code string) *sqlmock.Rows {
	return sqlmock.NewRows(appColumns()).AddRow(
		id, name, code, "", "",
		"{}", uuid.Nil, uuid.Nil,
		time.Now(), time.Now(), nil,
	)
}

// TestCreateApplication_MissingName verifies that a missing "name" field returns 400.
func TestCreateApplication_MissingName(t *testing.T) {
	h, _ := setupApplicationHandler(t)

	r := gin.New()
	r.POST("/applications", h.CreateApplication)

	payload, _ := json.Marshal(map[string]string{"code": "myapp"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateApplication_MissingCode verifies that a missing "code" field returns 400.
func TestCreateApplication_MissingCode(t *testing.T) {
	h, _ := setupApplicationHandler(t)

	r := gin.New()
	r.POST("/applications", h.CreateApplication)

	payload, _ := json.Marshal(map[string]string{"name": "My App"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetApplications_ReturnsArray verifies the list endpoint returns 200 with an array.
func TestGetApplications_ReturnsArray(t *testing.T) {
	h, mock := setupApplicationHandler(t)

	// Mock a SELECT that returns one application row
	appID := uuid.New()
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(appRow(appID, "TestApp", "testapp"))

	r := gin.New()
	r.GET("/applications", h.GetApplications)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
}

// TestGetApplications_Empty verifies that an empty table returns 200 with empty array.
func TestGetApplications_Empty(t *testing.T) {
	h, mock := setupApplicationHandler(t)

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(appColumns()))

	r := gin.New()
	r.GET("/applications", h.GetApplications)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestUpdateApplication_MissingName verifies updating with no "name" returns 400.
func TestUpdateApplication_MissingName(t *testing.T) {
	h, mock := setupApplicationHandler(t)

	// First SELECT to find application
	appID := uuid.New()
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(appRow(appID, "Old Name", "myapp"))

	r := gin.New()
	r.PUT("/applications/:id", h.UpdateApplication)

	payload, _ := json.Marshal(map[string]string{"description": "new desc"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/applications/"+appID.String(), bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetApplication_NotFound verifies that a missing application returns 404.
func TestGetApplication_NotFound(t *testing.T) {
	h, mock := setupApplicationHandler(t)

	// Both lookups (by ID then by code) return no rows
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(appColumns()))
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(appColumns()))

	r := gin.New()
	r.GET("/applications/:id", h.GetApplication)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/applications/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDeleteApplication_NotFound verifies that deleting a missing app returns 404.
func TestDeleteApplication_NotFound(t *testing.T) {
	h, mock := setupApplicationHandler(t)

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(appColumns()))

	r := gin.New()
	r.DELETE("/applications/:id", h.DeleteApplication)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/applications/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestApplicationHandler_ValidationEndpoints(t *testing.T) {
	h, _ := setupApplicationHandler(t)

	r := gin.New()
	r.POST("/applications/:id/languages", h.AddLanguage)
	r.GET("/applications/:id/pending-deploys", h.GetPendingDeploys)
	r.POST("/applications/:id/deploy-locale", h.DeployLocale)
	r.GET("/applications/:id/jobs/:job_id", h.GetAddLanguageJobStatus)
	r.GET("/applications/:id/active-jobs", h.GetActiveJobs)
	r.DELETE("/applications/:id/languages/:locale", h.DeleteLanguage)

	tests := []struct {
		name   string
		method string
		path   string
		body   interface{}
		code   int
	}{
		{"AddLanguage_InvalidAppID", http.MethodPost, "/applications/not-uuid/languages", map[string]any{"locale": "id"}, http.StatusBadRequest},
		{"AddLanguage_BadJSON", http.MethodPost, "/applications/" + uuid.New().String() + "/languages", map[string]any{}, http.StatusBadRequest},
		{"AddLanguage_EmptyLocale", http.MethodPost, "/applications/" + uuid.New().String() + "/languages", map[string]any{"locale": " "}, http.StatusBadRequest},
		{"GetPendingDeploys_InvalidAppID", http.MethodGet, "/applications/not-uuid/pending-deploys", nil, http.StatusBadRequest},
		{"DeployLocale_InvalidAppID", http.MethodPost, "/applications/not-uuid/deploy-locale", map[string]any{"locale": "id"}, http.StatusBadRequest},
		{"DeployLocale_BadJSON", http.MethodPost, "/applications/" + uuid.New().String() + "/deploy-locale", map[string]any{}, http.StatusBadRequest},
		{"GetAddLanguageJobStatus_InvalidAppID", http.MethodGet, "/applications/not-uuid/jobs/" + uuid.New().String(), nil, http.StatusBadRequest},
		{"GetAddLanguageJobStatus_InvalidJobID", http.MethodGet, "/applications/" + uuid.New().String() + "/jobs/not-uuid", nil, http.StatusBadRequest},
		{"GetActiveJobs_InvalidAppID", http.MethodGet, "/applications/not-uuid/active-jobs", nil, http.StatusBadRequest},
		{"DeleteLanguage_InvalidAppID", http.MethodDelete, "/applications/not-uuid/languages/id", nil, http.StatusBadRequest},
		{"DeleteLanguage_EmptyLocale", http.MethodDelete, "/applications/" + uuid.New().String() + "/languages/%20", nil, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

func TestApplicationHandler_ContextHelpers(t *testing.T) {
	h, _ := setupApplicationHandler(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	id := uuid.New()
	c.Set("user_id", id.String())
	c.Set("username", "alice")
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("User-Agent", "ua-test")

	userID, username := h.getCurrentUser(c)
	assert.Equal(t, id, userID)
	assert.Equal(t, "alice", username)

	ip, ua := h.getClientInfo(c)
	assert.NotEmpty(t, ip)
	assert.Equal(t, "ua-test", ua)
}

func TestNewApplicationHandler(t *testing.T) {
	h := NewApplicationHandler()
	assert.NotNil(t, h)
	assert.NotNil(t, h.auditService)
}

func TestApplicationHandler_BehavioralBranches(t *testing.T) {
	h, mock := setupApplicationHandler(t)
	r := gin.New()
	r.GET("/applications", h.GetApplications)
	r.GET("/applications/:id", h.GetApplication)
	r.POST("/applications", h.CreateApplication)
	r.PUT("/applications/:id", h.UpdateApplication)
	r.DELETE("/applications/:id", h.DeleteApplication)
	r.POST("/applications/:id/languages", h.AddLanguage)
	r.GET("/applications/:id/pending-deploys", h.GetPendingDeploys)
	r.POST("/applications/:id/deploy-locale", h.DeployLocale)
	r.GET("/applications/:id/jobs/:job_id", h.GetAddLanguageJobStatus)
	r.GET("/applications/:id/active-jobs", h.GetActiveJobs)
	r.DELETE("/applications/:id/languages/:locale", h.DeleteLanguage)

	t.Run("GetApplications_DBError", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WillReturnError(assert.AnError)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetApplication_ByCodeFallback", func(t *testing.T) {
		appID := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs("my-app", 1).WillReturnRows(sqlmock.NewRows(appColumns()))
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs("my-app", 1).WillReturnRows(appRow(appID, "My App", "my-app"))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/my-app", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("CreateApplication_DuplicateAndGenericError", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"name": "App",
			"code": "app-code",
		})

		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "applications"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("UpdateApplication_SaveError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(appRow(id, "Old", "old"))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "applications"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		body, _ := json.Marshal(map[string]any{"name": "New"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/applications/"+id.String(), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DeleteApplication_DeleteError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(appRow(id, "App", "app"))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "applications"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/applications/"+id.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("AddLanguage_AppNotFound", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(appColumns()))
		body, _ := json.Marshal(map[string]any{"locale": "id"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+id.String()+"/languages", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("AddLanguage_LocaleAlreadyEnabled", func(t *testing.T) {
		id := uuid.New()
		row := sqlmock.NewRows(appColumns()).AddRow(
			id, "App", "app", "", "",
			"{en,id}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil,
		)
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(row)
		body, _ := json.Marshal(map[string]any{"locale": "ID"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+id.String()+"/languages", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetPendingDeploys_DBError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "application_locale_deploys"`).WithArgs(id, "production").WillReturnError(assert.AnError)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+id.String()+"/pending-deploys", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetPendingDeploys_Success", func(t *testing.T) {
		id := uuid.New()
		now := time.Now()
		mock.ExpectQuery(`SELECT .*FROM "application_locale_deploys"`).WithArgs(id, "production").
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "locale", "stage_completed", "created_at", "updated_at", "deleted_at",
			}).
				AddRow(uuid.New(), id, "id", "draft", now, now, nil).
				AddRow(uuid.New(), id, "fr", "staging", now, now, nil))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+id.String()+"/pending-deploys", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DeployLocale_NotFoundAndAlreadyProduction", func(t *testing.T) {
		id := uuid.New()
		body, _ := json.Marshal(map[string]any{"locale": "id"})

		mock.ExpectQuery(`SELECT .*FROM "application_locale_deploys"`).WithArgs(id, "id", 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "locale", "stage_completed", "created_at", "updated_at", "deleted_at",
			}))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+id.String()+"/deploy-locale", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		now := time.Now()
		mock.ExpectQuery(`SELECT .*FROM "application_locale_deploys"`).WithArgs(id, "id", 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "locale", "stage_completed", "created_at", "updated_at", "deleted_at",
			}).AddRow(uuid.New(), id, "id", "production", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/applications/"+id.String()+"/deploy-locale", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DeployLocale_ComponentsQueryError", func(t *testing.T) {
		id := uuid.New()
		now := time.Now()
		mock.ExpectQuery(`SELECT .*FROM "application_locale_deploys"`).WithArgs(id, "id", 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "locale", "stage_completed", "created_at", "updated_at", "deleted_at",
			}).AddRow(uuid.New(), id, "id", "draft", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "components"`).WithArgs(id).WillReturnError(assert.AnError)
		body, _ := json.Marshal(map[string]any{"locale": "id"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+id.String()+"/deploy-locale", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetAddLanguageJobStatus_NotFoundAndFailedStatus", func(t *testing.T) {
		appID := uuid.New()
		jobID := uuid.New()

		mock.ExpectQuery(`SELECT .*FROM "add_language_jobs"`).
			WithArgs(jobID, appID, 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "locale", "auto_translate", "status",
				"total_components", "completed_components", "error_message", "error_detail",
				"claimed_by", "created_by", "created_at", "updated_at", "deleted_at",
			}))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/jobs/"+jobID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		now := time.Now()
		mock.ExpectQuery(`SELECT .*FROM "add_language_jobs"`).
			WithArgs(jobID, appID, 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "locale", "auto_translate", "status",
				"total_components", "completed_components", "error_message", "error_detail",
				"claimed_by", "created_by", "created_at", "updated_at", "deleted_at",
			}).AddRow(jobID, appID, "id", true, models.JobStatusFailed, 10, 4, "boom", "detail", "", uuid.Nil, now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/jobs/"+jobID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetActiveJobs_AddJobsError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "add_language_jobs"`).WithArgs(id, sqlmock.AnyArg()).WillReturnError(assert.AnError)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+id.String()+"/active-jobs", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DeleteLanguage_LocaleNotEnabled", func(t *testing.T) {
		id := uuid.New()
		row := sqlmock.NewRows(appColumns()).AddRow(
			id, "App", "app", "", "",
			"{en,fr}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil,
		)
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(row)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/applications/"+id.String()+"/languages/id", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

}

func TestApplicationHandler_AddLanguage_SyncSuccess(t *testing.T) {
	h, mock := setupApplicationHandler(t)
	r := gin.New()
	r.POST("/applications/:id/languages", h.AddLanguage)

	id := uuid.New()
	row := sqlmock.NewRows(appColumns()).AddRow(
		id, "App", "app", "", "",
		"{en}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil,
	)
	mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(row)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "applications"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	body, _ := json.Marshal(map[string]any{"locale": "id", "auto_translate": false})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/applications/"+id.String()+"/languages", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestApplicationHandler_AddLanguage_AutoTranslate_NoKey(t *testing.T) {
	h, mock := setupApplicationHandler(t)
	r := gin.New()
	r.POST("/applications/:id/languages", h.AddLanguage)

	id := uuid.New()
	now := time.Now()
	row := sqlmock.NewRows(appColumns()).AddRow(
		id, "App", "app", "", "",
		"{en}", uuid.Nil, uuid.Nil, now, now, nil,
	)
	mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(row)
	mock.ExpectQuery(`SELECT .*FROM "add_language_jobs"`).
		WithArgs(id, "id", sqlmock.AnyArg(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "application_id", "locale", "auto_translate", "status",
			"total_components", "completed_components", "error_message", "error_detail",
			"claimed_by", "created_by", "created_at", "updated_at", "deleted_at",
		}))

	body, _ := json.Marshal(map[string]any{"locale": "id", "auto_translate": true})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/applications/"+id.String()+"/languages", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApplicationHandler_DeleteLanguage_ComponentsQueryError(t *testing.T) {
	h, mock := setupApplicationHandler(t)
	r := gin.New()
	r.DELETE("/applications/:id/languages/:locale", h.DeleteLanguage)

	id := uuid.New()
	now := time.Now()
	mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(id, 1).WillReturnRows(
		sqlmock.NewRows(appColumns()).AddRow(id, "App", "app", "", "", "{en,id}", uuid.Nil, uuid.Nil, now, now, nil),
	)
	mock.ExpectQuery(`SELECT .*FROM "components"`).WithArgs(id).WillReturnError(assert.AnError)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/applications/"+id.String()+"/languages/id", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
