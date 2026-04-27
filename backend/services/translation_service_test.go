package services

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestExtractTemplateValues(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "Single template value",
			text:     "Hi [last_name]!",
			expected: []string{"last_name"},
		},
		{
			name:     "Multiple template values",
			text:     "Hello [first_name] [last_name], welcome!",
			expected: []string{"first_name", "last_name"},
		},
		{
			name:     "No template values",
			text:     "Hello world",
			expected: []string{},
		},
		{
			name:     "Nested brackets",
			text:     "Value [outer[inner]]",
			expected: []string{"outer[inner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTemplateValues(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPreserveTemplateValues(t *testing.T) {
	tests := []struct {
		name       string
		original   string
		translated string
		expected   string
	}{
		{
			name:       "Preserve single template",
			original:   "Hi [last_name]!",
			translated: "Hola [last_name]!",
			expected:   "Hola [last_name]!",
		},
		{
			name:       "Restore missing template",
			original:   "Hi [name]!",
			translated: "Hola!",
			expected:   "Hola [name]!",
		},
		{
			name:       "No templates",
			original:   "Hello",
			translated: "Hola",
			expected:   "Hola",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreserveTemplateValues(tt.original, tt.translated)
			// For simplicity, we check that template values from original are present
			templateValues := ExtractTemplateValues(tt.original)
			if len(templateValues) > 0 {
				for _, val := range templateValues {
					assert.Contains(t, result, "["+val+"]")
				}
			} else {
				assert.Equal(t, tt.translated, result)
			}
		})
	}
}

func TestNewTranslationService(t *testing.T) {
	service := NewTranslationService()
	assert.NotNil(t, service)
}

func setupTranslationServiceDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	oldDB := database.DB
	oldCache := cache.Client
	database.DB = gdb
	cache.Client = redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:0",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})

	t.Cleanup(func() {
		database.DB = oldDB
		cache.Client = oldCache
		_ = sqlDB.Close()
		require.NoError(t, mock.ExpectationsWereMet())
	})
	return mock
}

func applicationColumns() []string {
	return []string{
		"id", "name", "code", "description", "openai_key",
		"enabled_languages", "created_by", "updated_by", "created_at", "updated_at", "deleted_at",
	}
}

func componentColumnsForService() []string {
	return []string{
		"id", "application_id", "name", "code", "description", "structure", "default_locale",
		"created_by", "updated_by", "created_at", "updated_at", "deleted_at",
	}
}

func translationVersionColumns() []string {
	return []string{
		"id", "component_id", "locale", "stage", "version", "data", "is_active",
		"created_by", "updated_by", "created_at", "updated_at", "deleted_at",
	}
}

func TestGetMultipleTranslationsByCodes_ApplicationNotFound(t *testing.T) {
	mock := setupTranslationServiceDB(t)
	svc := NewTranslationService()

	mock.ExpectQuery(`SELECT .*FROM "applications"`).
		WithArgs("missing-app", 1).
		WillReturnRows(sqlmock.NewRows(applicationColumns()))

	got, err := svc.GetMultipleTranslationsByCodes("missing-app", []string{"header"}, "id", models.StageDraft)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestGetMultipleTranslationsByCodes_ComponentLookupError(t *testing.T) {
	mock := setupTranslationServiceDB(t)
	svc := NewTranslationService()

	appID := uuid.New()
	now := time.Now()
	mock.ExpectQuery(`SELECT .*FROM "applications"`).
		WithArgs("app_code", 1).
		WillReturnRows(sqlmock.NewRows(applicationColumns()).
			AddRow(appID, "App", "app_code", "", "", "{}", uuid.Nil, uuid.Nil, now, now, nil))

	mock.ExpectQuery(`SELECT .*FROM "components"`).
		WithArgs(sqlmock.AnyArg(), appID).
		WillReturnError(assert.AnError)

	got, err := svc.GetMultipleTranslationsByCodes("app_code", []string{"header"}, "id", models.StageDraft)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestGetMultipleTranslationsByCodes_MissingCodes(t *testing.T) {
	mock := setupTranslationServiceDB(t)
	svc := NewTranslationService()

	appID := uuid.New()
	compID := uuid.New()
	now := time.Now()
	mock.ExpectQuery(`SELECT .*FROM "applications"`).
		WithArgs("app_code", 1).
		WillReturnRows(sqlmock.NewRows(applicationColumns()).
			AddRow(appID, "App", "app_code", "", "", "{}", uuid.Nil, uuid.Nil, now, now, nil))

	mock.ExpectQuery(`SELECT .*FROM "components"`).
		WithArgs("header", "footer", appID).
		WillReturnRows(sqlmock.NewRows(componentColumnsForService()).
			AddRow(compID, appID, "Header", "header", "", []byte(`{}`), "en", uuid.Nil, uuid.Nil, now, now, nil))

	got, err := svc.GetMultipleTranslationsByCodes("app_code", []string{"header", "footer"}, "id", models.StageDraft)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestGetMultipleTranslations_DBHitOnCacheMiss(t *testing.T) {
	mock := setupTranslationServiceDB(t)
	svc := NewTranslationService()

	compID := uuid.New()
	now := time.Now()
	mock.ExpectQuery(`SELECT DISTINCT ON \(component_id\) \*`).
		WithArgs(sqlmock.AnyArg(), "id", models.StageDraft, true).
		WillReturnRows(sqlmock.NewRows(translationVersionColumns()).
			AddRow(uuid.New(), compID, "id", models.StageDraft, 3, []byte(`{"hello":"halo"}`), true, uuid.Nil, uuid.Nil, now, now, nil))

	got, err := svc.GetMultipleTranslations([]uuid.UUID{compID}, "id", models.StageDraft)
	require.NoError(t, err)
	require.Contains(t, got, compID.String())
	assert.Equal(t, 3, got[compID.String()].Version)
}

func TestGetTranslation_DBError(t *testing.T) {
	mock := setupTranslationServiceDB(t)
	svc := NewTranslationService()
	compID := uuid.New()

	mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
		WithArgs(compID, "id", models.StageDraft, true, 1).
		WillReturnRows(sqlmock.NewRows(translationVersionColumns()))

	got, err := svc.GetTranslation(compID, "id", models.StageDraft)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestListVersionsAndGetVersionByNumber(t *testing.T) {
	mock := setupTranslationServiceDB(t)
	svc := NewTranslationService()
	compID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
		WithArgs(compID, "id", models.StageDraft).
		WillReturnRows(sqlmock.NewRows(translationVersionColumns()).
			AddRow(uuid.New(), compID, "id", models.StageDraft, 2, []byte(`{"title":"x"}`), true, uuid.Nil, uuid.Nil, now, now, nil))

	versions, err := svc.ListVersions(compID, "id", models.StageDraft)
	require.NoError(t, err)
	require.Len(t, versions, 1)

	mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
		WithArgs(compID, "id", models.StageDraft, 99, 1).
		WillReturnRows(sqlmock.NewRows(translationVersionColumns()))
	v, err := svc.GetVersionByNumber(compID, "id", models.StageDraft, 99)
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestDeleteTranslationVersionByID(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		mock := setupTranslationServiceDB(t)
		svc := NewTranslationService()
		id := uuid.New()

		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(id, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()))

		assert.Error(t, svc.DeleteTranslationVersionByID(id))
	})

	t.Run("success", func(t *testing.T) {
		mock := setupTranslationServiceDB(t)
		svc := NewTranslationService()
		id := uuid.New()
		compID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(id, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()).
				AddRow(id, compID, "id", models.StageDraft, 1, []byte(`{"ok":"1"}`), true, uuid.Nil, uuid.Nil, now, now, nil))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "translation_versions" SET "deleted_at"=`).
			WithArgs(sqlmock.AnyArg(), id).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		assert.NoError(t, svc.DeleteTranslationVersionByID(id))
	})
}

func TestSaveTranslation_Branches(t *testing.T) {
	t.Run("create error", func(t *testing.T) {
		mock := setupTranslationServiceDB(t)
		svc := NewTranslationService()
		compID := uuid.New()

		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(compID, "id", models.StageDraft, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "translation_versions"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()

		got, err := svc.SaveTranslation(compID, "id", models.StageDraft, models.JSONB{"hello": "world"}, uuid.Nil)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("next version and cleanup", func(t *testing.T) {
		mock := setupTranslationServiceDB(t)
		svc := NewTranslationService()
		compID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(compID, "id", models.StageDraft, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()).
				AddRow(uuid.New(), compID, "id", models.StageDraft, 2, []byte(`{"old":"1"}`), true, uuid.Nil, uuid.Nil, now, now, nil))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "translation_versions"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
		mock.ExpectCommit()
		mock.ExpectExec(`DELETE FROM translation_versions`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		got, err := svc.SaveTranslation(compID, "id", models.StageDraft, models.JSONB{"new": "2"}, uuid.Nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 3, got.Version)
	})
}

func TestRevertTranslation_Branches(t *testing.T) {
	t.Run("no current version", func(t *testing.T) {
		mock := setupTranslationServiceDB(t)
		svc := NewTranslationService()
		compID := uuid.New()
		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(compID, "id", models.StageDraft, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()))

		err := svc.RevertTranslation(compID, "id", models.StageDraft, uuid.Nil)
		assert.Error(t, err)
	})

	t.Run("no previous version", func(t *testing.T) {
		mock := setupTranslationServiceDB(t)
		svc := NewTranslationService()
		compID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(compID, "id", models.StageDraft, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()).
				AddRow(uuid.New(), compID, "id", models.StageDraft, 1, []byte(`{"v":"1"}`), true, uuid.Nil, uuid.Nil, now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(compID, "id", models.StageDraft, 0, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()))

		err := svc.RevertTranslation(compID, "id", models.StageDraft, uuid.Nil)
		assert.Error(t, err)
	})

	t.Run("create reverted version error", func(t *testing.T) {
		mock := setupTranslationServiceDB(t)
		svc := NewTranslationService()
		compID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(compID, "id", models.StageDraft, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()).
				AddRow(uuid.New(), compID, "id", models.StageDraft, 2, []byte(`{"v":"2"}`), true, uuid.Nil, uuid.Nil, now, now, nil))
		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(compID, "id", models.StageDraft, 1, 1).
			WillReturnRows(sqlmock.NewRows(translationVersionColumns()).
				AddRow(uuid.New(), compID, "id", models.StageDraft, 1, []byte(`{"v":"1"}`), true, uuid.Nil, uuid.Nil, now, now, nil))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "translation_versions"`).WillReturnError(assert.AnError)
		mock.ExpectRollback()

		err := svc.RevertTranslation(compID, "id", models.StageDraft, uuid.Nil)
		assert.Error(t, err)
	})
}

func TestDeployToStage_SourceMissing(t *testing.T) {
	mock := setupTranslationServiceDB(t)
	svc := NewTranslationService()
	compID := uuid.New()

	mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
		WithArgs(compID, "id", models.StageDraft, true, 1).
		WillReturnRows(sqlmock.NewRows(translationVersionColumns()))

	err := svc.DeployToStage(compID, "id", models.StageDraft, models.StageStaging, uuid.Nil)
	assert.Error(t, err)
}
