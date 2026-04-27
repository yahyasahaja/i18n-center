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

func setupComponentHandler(t *testing.T) (*ComponentHandler, sqlmock.Sqlmock) {
	db, mock := newMockDB(t)
	withMockDB(t, db)
	audit := newMockAuditService()
	h := &ComponentHandler{auditService: audit}
	return h, mock
}

func componentColumns() []string {
	return []string{
		"id", "application_id", "name", "code", "description",
		"structure", "default_locale", "created_by", "updated_by",
		"created_at", "updated_at", "deleted_at",
	}
}

func componentRow(id, appID uuid.UUID, name, code string) *sqlmock.Rows {
	return sqlmock.NewRows(componentColumns()).AddRow(
		id, appID, name, code, "",
		[]byte("{}"), "en", uuid.Nil, uuid.Nil,
		time.Now(), time.Now(), nil,
	)
}

// TestCreateComponent_MissingRequired verifies that a request without required fields returns 400.
func TestCreateComponent_MissingRequired(t *testing.T) {
	h, _ := setupComponentHandler(t)

	r := gin.New()
	r.POST("/components", h.CreateComponent)

	// Missing name, code, application_id, default_locale
	payload, _ := json.Marshal(map[string]string{"description": "some desc"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/components", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateComponent_MissingCode verifies that missing code returns 400.
func TestCreateComponent_MissingCode(t *testing.T) {
	h, _ := setupComponentHandler(t)

	r := gin.New()
	r.POST("/components", h.CreateComponent)

	payload, _ := json.Marshal(map[string]interface{}{
		"name":           "My Component",
		"application_id": uuid.New().String(),
		"default_locale": "en",
		// code is missing
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/components", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetComponents_Success verifies that GET /components returns 200 with a list.
func TestGetComponents_Success(t *testing.T) {
	h, mock := setupComponentHandler(t)

	appID := uuid.New()
	compID := uuid.New()

	// Main SELECT for components
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(componentRow(compID, appID, "Header", "header"))
	// Preload Tags
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "application_id", "code", "created_at", "updated_at", "deleted_at"}))
	// Preload Pages
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "application_id", "code", "created_at", "updated_at", "deleted_at"}))

	r := gin.New()
	r.GET("/components", h.GetComponents)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/components", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestDeleteComponent_NotFound verifies deleting a missing component returns 404.
func TestDeleteComponent_NotFound(t *testing.T) {
	h, mock := setupComponentHandler(t)

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(componentColumns()))

	r := gin.New()
	r.DELETE("/components/:id", h.DeleteComponent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/components/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetComponent_NotFound verifies that a missing component by ID returns 404.
func TestGetComponent_NotFound(t *testing.T) {
	h, mock := setupComponentHandler(t)

	// Both lookups (by ID, then by code) return no rows
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(componentColumns()))
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(componentColumns()))

	r := gin.New()
	r.GET("/components/:id", h.GetComponent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/components/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestUpdateComponent_NotFound verifies updating a missing component returns 404.
func TestUpdateComponent_NotFound(t *testing.T) {
	h, mock := setupComponentHandler(t)

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(componentColumns()))

	r := gin.New()
	r.PUT("/components/:id", h.UpdateComponent)

	payload, _ := json.Marshal(map[string]string{"name": "New Name"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/components/"+uuid.New().String(), bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestComponentHandler_ValidationAndHelpers(t *testing.T) {
	h, mock := setupComponentHandler(t)

	r := gin.New()
	r.POST("/components", h.CreateComponent)
	r.GET("/components", h.GetComponents)
	r.GET("/components/:id", h.GetComponent)
	r.PUT("/components/:id", h.UpdateComponent)
	r.DELETE("/components/:id", h.DeleteComponent)

	// Force one GetComponents path to return DB error.
	mock.ExpectQuery(`SELECT`).WillReturnError(assert.AnError)

	t.Run("GetComponents_DBError", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/components", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("CreateComponent_BadJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/components", bytes.NewBufferString(`{"name":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetComponent_NotFoundByCodePath", func(t *testing.T) {
		// Two lookups return empty rows => not found.
		mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(componentColumns()))
		mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(componentColumns()))

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/components/not-found-code", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("UpdateComponent_BadJSONAfterLookup", func(t *testing.T) {
		id := uuid.New()
		appID := uuid.New()
		mock.ExpectQuery(`SELECT`).WillReturnRows(componentRow(id, appID, "Header", "header"))
		mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id", "application_id", "code", "created_at", "updated_at", "deleted_at"}))
		mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id", "application_id", "code", "created_at", "updated_at", "deleted_at"}))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/components/"+id.String(), bytes.NewBufferString(`{"name":`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DeleteComponent_NotFoundAgain", func(t *testing.T) {
		mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(componentColumns()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/components/"+uuid.New().String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("ContextHelpers", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		id := uuid.New()
		c.Set("user_id", id.String())
		c.Set("username", "bob")
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Header.Set("User-Agent", "ua-component")
		userID, username := h.getCurrentUser(c)
		assert.Equal(t, id, userID)
		assert.Equal(t, "bob", username)
		ip, ua := h.getClientInfo(c)
		assert.NotEmpty(t, ip)
		assert.Equal(t, "ua-component", ua)
	})
}
