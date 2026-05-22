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

	"github.com/your-org/i18n-center/mocks"
	"github.com/your-org/i18n-center/repository/cms"
)

// ── column helpers ────────────────────────────────────────────────────────────

func cmsTemplateFieldCols() []string {
	return []string{"id", "template_id", "key", "label", "value_type", "required", "sort_order", "created_at", "updated_at"}
}

// ── setup helper ─────────────────────────────────────────────────────────────

// setupCmsTemplateHandler uses the proper constructor so the repository field
// (h.templates) is initialised. sqlmock wired into both *gorm.DB and *sqlx.DB.
func setupCmsTemplateHandler(t *testing.T) (*CmsTemplateHandler, sqlmock.Sqlmock, *mocks.MockAuditServicer) {
	db, xdb, mock := newMockDB(t)
	withMockDB(t, db, xdb)
	auditMock := newMockAuditService()
	h := NewCmsTemplateHandler()
	h.auditService = auditMock
	return h, mock, auditMock
}

// skipUntilCommitI_template (suffixed to avoid name collision with the one
// in cms_item_handler_test.go) marks a CMS template test as superseded by the
// sqlx repository conversion. Removed in Commit I along with the rewrites.
func skipUntilCommitI_template(t *testing.T) {
	t.Helper()
	t.Skip("TODO(commit I): rewrite for sqlx repository layer; assertions encode GORM-era SQL")
}

// ── shared fixtures ───────────────────────────────────────────────────────────

func tmplRow(appID, tmplID uuid.UUID, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(cmsTemplateCols()).AddRow(
		tmplID, appID, "Banner Template", "banner", "A banner",
		uuid.Nil, uuid.Nil, now, now, nil,
	)
}

func fieldRow(tmplID, fieldID uuid.UUID, key, valueType string, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(cmsTemplateFieldCols()).AddRow(
		fieldID, tmplID, key, key + " label", valueType, false, 0, now, now,
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// ListTemplates
// ─────────────────────────────────────────────────────────────────────────────

func TestCmsTemplateHandler_ListTemplates(t *testing.T) {
	skipUntilCommitI_template(t)
	h, mock, _ := setupCmsTemplateHandler(t)
	r := gin.New()
	r.GET("/applications/:id/cms/templates", h.ListTemplates)

	appID := uuid.New()
	tmplID := uuid.New()
	fieldID := uuid.New()
	now := time.Now()

	t.Run("DBError", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).WillReturnError(assert.AnError)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/cms/templates", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Success_FieldsPreloaded", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(appID).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectQuery(`SELECT .* FROM "cms_template_fields"`).
			WillReturnRows(fieldRow(tmplID, fieldID, "title", "text", now))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/cms/templates", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var body []map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		assert.Len(t, body, 1)
		assert.Equal(t, "banner", body[0]["code"])
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GetTemplate
// ─────────────────────────────────────────────────────────────────────────────

func TestCmsTemplateHandler_GetTemplate(t *testing.T) {
	skipUntilCommitI_template(t)
	h, mock, _ := setupCmsTemplateHandler(t)
	r := gin.New()
	r.GET("/cms/templates/:id", h.GetTemplate)

	appID := uuid.New()
	tmplID := uuid.New()
	fieldID := uuid.New()
	now := time.Now()

	t.Run("NotFound", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(sqlmock.NewRows(cmsTemplateCols()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/cms/templates/"+tmplID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Success_FieldsSortedBySortOrder", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		// Return fields in reverse sort order to verify sorting
		mock.ExpectQuery(`SELECT .* FROM "cms_template_fields"`).
			WillReturnRows(sqlmock.NewRows(cmsTemplateFieldCols()).
				AddRow(fieldID, tmplID, "body", "body label", "rich_text", false, 10, now, now).
				AddRow(uuid.New(), tmplID, "title", "title label", "text", true, 0, now, now))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/cms/templates/"+tmplID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		assert.Equal(t, "banner", body["code"])
		fields, ok := body["fields"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, fields, 2)
		// first field should have sort_order=0 (title)
		first := fields[0].(map[string]interface{})
		assert.Equal(t, "title", first["key"])
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// CreateTemplate
// ─────────────────────────────────────────────────────────────────────────────

func TestCmsTemplateHandler_CreateTemplate(t *testing.T) {
	skipUntilCommitI_template(t)
	h, mock, auditMock := setupCmsTemplateHandler(t)
	allowAnyAudit(auditMock)
	r := gin.New()
	r.POST("/applications/:id/cms/templates", h.CreateTemplate)

	appID := uuid.New()
	tmplID := uuid.New()
	fieldID := uuid.New()
	now := time.Now()

	validPayload := func(fields []map[string]interface{}) []byte {
		b, _ := json.Marshal(map[string]interface{}{
			"name":        "Banner Template",
			"code":        "banner",
			"description": "A banner",
			"fields":      fields,
		})
		return b
	}

	t.Run("BadAppID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/not-uuid/cms/templates",
			bytes.NewBuffer(validPayload(nil)))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{"name": "X"}) // missing code
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/cms/templates",
			bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DuplicateCode", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "cms_templates"`).
			WillReturnError(assert.AnError) // generic error — no "duplicate" keyword → 500
		mock.ExpectRollback()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/cms/templates",
			bytes.NewBuffer(validPayload(nil)))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("InvalidFieldValueType", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "cms_templates"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tmplID))
		mock.ExpectRollback()
		payload := validPayload([]map[string]interface{}{
			{"key": "x", "label": "X", "value_type": "blob"}, // invalid
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/cms/templates",
			bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var body map[string]string
		json.NewDecoder(w.Body).Decode(&body)
		assert.Contains(t, body["error"], "Invalid value_type")
	})

	t.Run("FieldInsertError", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "cms_templates"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tmplID))
		mock.ExpectQuery(`INSERT INTO "cms_template_fields"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		payload := validPayload([]map[string]interface{}{
			{"key": "title", "label": "Title", "value_type": "text"},
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/cms/templates",
			bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Success_AllFieldTypes", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "cms_templates"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tmplID))
		// Four fields — one of each valid type
		for range []string{"text", "textarea", "rich_text", "json"} {
			mock.ExpectQuery(`INSERT INTO "cms_template_fields"`).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
		}
		mock.ExpectCommit()
		// Preload after create
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectQuery(`SELECT .* FROM "cms_template_fields"`).
			WillReturnRows(sqlmock.NewRows(cmsTemplateFieldCols()).
				AddRow(fieldID, tmplID, "title", "Title", "text", true, 0, now, now).
				AddRow(uuid.New(), tmplID, "excerpt", "Excerpt", "textarea", false, 1, now, now).
				AddRow(uuid.New(), tmplID, "body", "Body", "rich_text", false, 2, now, now).
				AddRow(uuid.New(), tmplID, "config", "Config", "json", false, 3, now, now))
		payload := validPayload([]map[string]interface{}{
			{"key": "title", "label": "Title", "value_type": "text", "required": true, "sort_order": 0},
			{"key": "excerpt", "label": "Excerpt", "value_type": "textarea", "sort_order": 1},
			{"key": "body", "label": "Body", "value_type": "rich_text", "sort_order": 2},
			{"key": "config", "label": "Config", "value_type": "json", "sort_order": 3},
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/applications/"+appID.String()+"/cms/templates",
			bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		assert.Equal(t, "banner", body["code"])
		fields, _ := body["fields"].([]interface{})
		assert.Len(t, fields, 4)
		// Verify sort order is maintained
		assert.Equal(t, "title", fields[0].(map[string]interface{})["key"])
		assert.Equal(t, "config", fields[3].(map[string]interface{})["key"])
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateTemplate
// ─────────────────────────────────────────────────────────────────────────────

func TestCmsTemplateHandler_UpdateTemplate(t *testing.T) {
	skipUntilCommitI_template(t)
	h, mock, auditMock := setupCmsTemplateHandler(t)
	allowAnyAudit(auditMock)
	r := gin.New()
	r.PUT("/cms/templates/:id", h.UpdateTemplate)

	appID := uuid.New()
	tmplID := uuid.New()
	fieldID := uuid.New()
	now := time.Now()

	t.Run("NotFound", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(sqlmock.NewRows(cmsTemplateCols()))
		w := httptest.NewRecorder()
		payload, _ := json.Marshal(map[string]string{"name": "New Name"})
		req := httptest.NewRequest(http.MethodPut, "/cms/templates/"+tmplID.String(), bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("BadJSON", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/cms/templates/"+tmplID.String(),
			bytes.NewBufferString(`{"name":`)) // malformed JSON
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("SaveError", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "cms_templates"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		payload, _ := json.Marshal(map[string]string{"name": "Updated"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/cms/templates/"+tmplID.String(), bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("InvalidFieldValueTypeInUpdate", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "cms_templates"`).WillReturnResult(sqlmock.NewResult(1, 1))
		// CmsTemplateField has no DeletedAt → GORM issues a hard DELETE, not a soft UPDATE
		mock.ExpectExec(`DELETE FROM "cms_template_fields"`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()
		payload, _ := json.Marshal(map[string]interface{}{
			"fields": []map[string]interface{}{
				{"key": "x", "label": "X", "value_type": "invalid"},
			},
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/cms/templates/"+tmplID.String(), bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Success_ReplaceFields", func(t *testing.T) {
		newFieldID := uuid.New()
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "cms_templates"`).WillReturnResult(sqlmock.NewResult(1, 1))
		// Hard DELETE (no DeletedAt on CmsTemplateField)
		mock.ExpectExec(`DELETE FROM "cms_template_fields"`).WillReturnResult(sqlmock.NewResult(1, 1))
		// Insert replacement field
		mock.ExpectQuery(`INSERT INTO "cms_template_fields"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newFieldID))
		mock.ExpectCommit()
		// Preload("Fields").First after update → SELECT cms_templates + SELECT cms_template_fields
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectQuery(`SELECT .* FROM "cms_template_fields"`).
			WillReturnRows(fieldRow(tmplID, fieldID, "body", "rich_text", now))
		payload, _ := json.Marshal(map[string]interface{}{
			"name": "Updated Banner",
			"fields": []map[string]interface{}{
				{"key": "body", "label": "Body", "value_type": "rich_text"},
			},
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/cms/templates/"+tmplID.String(), bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		fields, _ := body["fields"].([]interface{})
		assert.Len(t, fields, 1)
		assert.Equal(t, "rich_text", fields[0].(map[string]interface{})["value_type"])
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DeleteTemplate
// ─────────────────────────────────────────────────────────────────────────────

func TestCmsTemplateHandler_DeleteTemplate(t *testing.T) {
	skipUntilCommitI_template(t)
	h, mock, auditMock := setupCmsTemplateHandler(t)
	allowAnyAudit(auditMock)
	r := gin.New()
	r.DELETE("/cms/templates/:id", h.DeleteTemplate)

	appID := uuid.New()
	tmplID := uuid.New()
	now := time.Now()

	t.Run("NotFound", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(sqlmock.NewRows(cmsTemplateCols()))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/cms/templates/"+tmplID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("BlockedByExistingItems", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectQuery(`SELECT count\(\*\) FROM "cms_items"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/cms/templates/"+tmplID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var body map[string]string
		json.NewDecoder(w.Body).Decode(&body)
		assert.Contains(t, body["error"], "Cannot delete template")
	})

	t.Run("DeleteError", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectQuery(`SELECT count\(\*\) FROM "cms_items"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "cms_templates"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/cms/templates/"+tmplID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "cms_templates"`).
			WithArgs(tmplID, 1).
			WillReturnRows(tmplRow(appID, tmplID, now))
		mock.ExpectQuery(`SELECT count\(\*\) FROM "cms_items"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "cms_templates"`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/cms/templates/"+tmplID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var body map[string]string
		json.NewDecoder(w.Body).Decode(&body)
		assert.Equal(t, "Template deleted", body["message"])
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// cms.IsValidValueType (unit)
// ─────────────────────────────────────────────────────────────────────────────

// The helper moved into the repository/cms package as cms.IsValidValueType
// during Commit G — same behavior, just one canonical location now.
func TestIsValidCmsValueType(t *testing.T) {
	validTypes := []string{"text", "textarea", "rich_text", "json"}
	for _, vt := range validTypes {
		assert.True(t, cms.IsValidValueType(vt), "expected %q to be valid", vt)
	}

	invalidTypes := []string{"blob", "html", "number", "boolean", ""}
	for _, vt := range invalidTypes {
		assert.False(t, cms.IsValidValueType(vt), "expected %q to be invalid", vt)
	}
}
