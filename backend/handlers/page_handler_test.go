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

func setupPageHandler(t *testing.T) (*PageHandler, sqlmock.Sqlmock) {
	db, xdb, mock := newMockDB(t)
	withMockDB(t, db, xdb)
	return &PageHandler{auditService: newMockAuditService()}, mock
}

func pageColumns() []string {
	return []string{"id", "application_id", "code", "created_at", "updated_at", "deleted_at"}
}

func TestPageHandler_BasicFlows(t *testing.T) {
	h, mock := setupPageHandler(t)
	r := gin.New()
	r.GET("/applications/:id/pages", h.ListByApplication)
	r.POST("/applications/:id/pages", h.Create)
	r.GET("/pages/:id", h.Get)
	r.PUT("/pages/:id", h.Update)
	r.DELETE("/pages/:id", h.Delete)
	r.GET("/pages/:id/components", h.GetComponents)

	appID := uuid.New()
	pageID := uuid.New()
	now := time.Now()

	t.Run("ListByApplication_BadID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/not-uuid/pages", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ListByApplication_DBError_AndSuccess", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .*FROM "pages"`).WithArgs(appID).WillReturnError(assert.AnError)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/pages", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(appID).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "home", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/pages", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Create_Validation_Duplicate_InsertError", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/pages", bytes.NewBufferString(`{"x":1}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		payload, _ := json.Marshal(map[string]string{"code": "home"})

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(appID, "home", 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(uuid.New(), appID, "home", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/pages", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(appID, "home", 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "pages"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/pages", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Get_NotFound_AndSuccess", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "pages"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(pageColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/pages/"+id.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "home", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/pages/"+pageID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Update_NotFound_BadJSON_Duplicate_SaveError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "pages"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(pageColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/pages/"+id.String(), bytes.NewBufferString(`{"code":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "old", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/pages/"+pageID.String(), bytes.NewBufferString(`{"code":`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "old", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(appID, "new", pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(uuid.New(), appID, "new", now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/pages/"+pageID.String(), bytes.NewBufferString(`{"code":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "old", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(appID, "new", pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "pages"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/pages/"+pageID.String(), bytes.NewBufferString(`{"code":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Delete_NotFound_DeleteError", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "pages"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(pageColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/pages/"+id.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "home", now, now, nil))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "pages"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/pages/"+pageID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetComponents_NotFound_DBError_Success", func(t *testing.T) {
		id := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "pages"`).WithArgs(id, 1).WillReturnRows(sqlmock.NewRows(pageColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/pages/"+id.String()+"/components", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "home", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "components"`).WithArgs(pageID).WillReturnError(assert.AnError)
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/pages/"+pageID.String()+"/components", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mock.ExpectQuery(`SELECT .*FROM "pages"`).
			WithArgs(pageID, 1).
			WillReturnRows(sqlmock.NewRows(pageColumns()).AddRow(pageID, appID, "home", now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "components"`).
			WithArgs(pageID).
			WillReturnRows(sqlmock.NewRows(componentColumnsForTagPage()).
				AddRow(uuid.New(), appID, "Hero", "hero", "", []byte(`{}`), "en", uuid.Nil, uuid.Nil, now, now, nil))
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/pages/"+pageID.String()+"/components", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
