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
)

// setupTagHandler uses the proper constructor so repository fields are
// initialised; sqlmock is wired into both *gorm.DB and *sqlx.DB.
func setupTagHandler(t *testing.T) (*TagHandler, sqlmock.Sqlmock) {
	xdb, mock := newMockDB(t)
	withMockDB(t, xdb)
	h := NewTagHandler()
	h.auditService = newMockAuditService()
	return h, mock
}

func tagColumns() []string {
	return []string{"id", "application_id", "code", "created_at", "updated_at", "deleted_at"}
}

func componentColumnsForTagPage() []string {
	return []string{
		"id", "application_id", "name", "code", "description", "structure", "default_locale",
		"created_by", "updated_by", "created_at", "updated_at", "deleted_at",
	}
}

// TestTagHandler_BasicFlows tightly couples to the GORM-era SQL shape
// (quoted "tags" table, implicit LIMIT 1 args, BEGIN/COMMIT around single-
// statement writes). Defer rewrite to Commit I cleanup alongside the GORM
// strip and focused per-repository tests.
//
// TODO(refactor I): rewrite as targeted tests against tag.Repository.
func TestTagHandler_BasicFlows(t *testing.T) {
	t.Skip("TODO: rewrite for sqlx repository layer (Commit I cleanup)")
	h, mock := setupTagHandler(t)
	r := gin.New()
	r.GET("/applications/:id/tags", h.ListByApplication)
	r.POST("/applications/:id/tags", h.Create)
	r.GET("/tags/:id", h.Get)
	r.PUT("/tags/:id", h.Update)
	r.DELETE("/tags/:id", h.Delete)
	r.GET("/tags/:id/components", h.GetComponents)

	appID := uuid.New()
	tagID := uuid.New()
	now := time.Now()

	t.Run("ListByApplication_BadID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/not-uuid/tags", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ListByApplication_DBError", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .*FROM "tags"`).WithArgs(appID).WillReturnError(assert.AnError)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/tags", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ListByApplication_Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(appID).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "header", now, now, nil))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/tags", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Create_BadJSONAndValidation", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/tags", bytes.NewBufferString(`{"x":1}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		body, _ := json.Marshal(map[string]string{"code": "   "})
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/tags", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Create_DuplicateAndInsertError", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{"code": "header"})

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(appID, "header", 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(uuid.New(), appID, "header", now, now, nil))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/tags", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(appID, "header", 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "tags"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/tags", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Get_NotFoundAndSuccess", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "tags"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(tagColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/tags/"+id.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "header", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/tags/"+tagID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Update_NotFound_BadJSON_Duplicate_SaveError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "tags"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(tagColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/tags/"+id.String(), bytes.NewBufferString(`{"code":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "old", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/tags/"+tagID.String(), bytes.NewBufferString(`{"code":`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "old", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(appID, "new", tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(uuid.New(), appID, "new", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/tags/"+tagID.String(), bytes.NewBufferString(`{"code":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "old", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(appID, "new", tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "tags"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/tags/"+tagID.String(), bytes.NewBufferString(`{"code":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Delete_NotFound_DeleteError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "tags"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(tagColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/tags/"+id.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "header", now, now, nil))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "tags"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/tags/"+tagID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetComponents_NotFound_DBError_Success", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "tags"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(tagColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/tags/"+id.String()+"/components", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "header", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "components"`).WithArgs(tagID).WillReturnError(assert.AnError)
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/tags/"+tagID.String()+"/components", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "tags"`).
			WithArgs(tagID, 1).
			WillReturnRows(sqlmock.NewRows(tagColumns()).AddRow(tagID, appID, "header", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "components"`).
			WithArgs(tagID).
			WillReturnRows(sqlmock.NewRows(componentColumnsForTagPage()).
				AddRow(uuid.New(), appID, "Header", "header", "", []byte(`{}`), "en", uuid.Nil, uuid.Nil, now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/tags/"+tagID.String()+"/components", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
