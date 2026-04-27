package jobs

import (
	"context"
	"os"
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
	"github.com/your-org/i18n-center/observability"
	"github.com/your-org/i18n-center/services"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestResolveOpenAIService(t *testing.T) {
	t.Run("nil when no key", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		assert.Nil(t, resolveOpenAIService(""))
	})

	t.Run("uses app key", func(t *testing.T) {
		svc := resolveOpenAIService("mock")
		if assert.NotNil(t, svc) {
			assert.Equal(t, "mock", svc.APIKey)
		}
	})
}

func TestSilentDB(t *testing.T) {
	sqlDB, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	assert.NotNil(t, silentDB())
}

func TestGetJobStatus_NotFound(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	job, err := GetJobStatus(uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, job)
}

func TestMarkJobFailedFunctions(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "add_language_jobs"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	assert.NoError(t, markAddJobFailed(uuid.New(), "x", "y"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "translate_jobs"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	assert.NoError(t, markTranslateJobFailed(uuid.New(), "x", "y"))
}

func TestResolveOpenAIServiceWithEnvFallback(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")
	svc := resolveOpenAIService("")
	if assert.NotNil(t, svc) {
		assert.Equal(t, "env-key", svc.APIKey)
	}
}

func TestGetJobStatus_Found(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	appID := uuid.New()
	jobID := uuid.New()
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "application_id", "locale", "auto_translate", "status",
		"total_components", "completed_components", "error_message", "error_detail",
		"claimed_by", "created_by", "created_at", "updated_at", "deleted_at",
	}).AddRow(jobID, appID, "id", true, models.JobStatusPending, 0, 0, "", "", "", uuid.Nil, now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	job, err := GetJobStatus(appID, jobID)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, jobID, job.ID)
}

func TestRun_CancelledContext(t *testing.T) {
	oldLogger := observability.Logger
	observability.Logger = zap.NewNop()
	t.Cleanup(func() { observability.Logger = oldLogger })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t.Setenv("HOSTNAME", "")
	t.Setenv("WORKER_ID", "")
	Run(ctx)
}

func TestClaimAddLanguageJob(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	t.Run("none pending", func(t *testing.T) {
		mock.ExpectExec(`UPDATE add_language_jobs`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`UPDATE add_language_jobs`).WithArgs(models.JobStatusRunning, "inst-1", models.JobStatusPending).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.Nil))

		job, err := claimAddLanguageJob("inst-1")
		require.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("claimed", func(t *testing.T) {
		id := uuid.New()
		appID := uuid.New()
		now := time.Now()
		mock.ExpectExec(`UPDATE add_language_jobs`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`UPDATE add_language_jobs`).WithArgs(models.JobStatusRunning, "inst-2", models.JobStatusPending).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
		mock.ExpectQuery(`SELECT .*FROM "add_language_jobs"`).
			WithArgs(id, 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "locale", "auto_translate", "status",
				"total_components", "completed_components", "error_message", "error_detail",
				"claimed_by", "created_by", "created_at", "updated_at", "deleted_at",
			}).AddRow(id, appID, "id", true, models.JobStatusRunning, 1, 0, "", "", "inst-2", uuid.Nil, now, now, nil))

		job, err := claimAddLanguageJob("inst-2")
		require.NoError(t, err)
		require.NotNil(t, job)
		assert.Equal(t, id, job.ID)
	})
}

func TestClaimTranslateJob(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	t.Run("none pending", func(t *testing.T) {
		mock.ExpectExec(`UPDATE translate_jobs`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`UPDATE translate_jobs`).WithArgs(models.JobStatusRunning, "inst-a", models.JobStatusPending).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.Nil))
		job, err := claimTranslateJob("inst-a")
		require.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("claimed", func(t *testing.T) {
		id := uuid.New()
		appID := uuid.New()
		compID := uuid.New()
		now := time.Now()
		mock.ExpectExec(`UPDATE translate_jobs`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`UPDATE translate_jobs`).WithArgs(models.JobStatusRunning, "inst-b", models.JobStatusPending).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
		mock.ExpectQuery(`SELECT .*FROM "translate_jobs"`).
			WithArgs(id, 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "application_id", "component_id", "job_type", "source_locale", "target_locales",
				"status", "error_message", "error_detail", "claimed_by", "created_by", "created_at", "updated_at", "deleted_at",
			}).AddRow(id, appID, compID, models.TranslateJobTypeAutoTranslate, "en", "{id}", models.JobStatusRunning, "", "", "inst-b", uuid.Nil, now, now, nil))
		job, err := claimTranslateJob("inst-b")
		require.NoError(t, err)
		require.NotNil(t, job)
		assert.Equal(t, id, job.ID)
	})
}

func TestRun_UsesWorkerIDEenv(t *testing.T) {
	oldLogger := observability.Logger
	observability.Logger = zap.NewNop()
	t.Cleanup(func() { observability.Logger = oldLogger })

	oldHost := os.Getenv("HOSTNAME")
	oldWorker := os.Getenv("WORKER_ID")
	t.Cleanup(func() {
		_ = os.Setenv("HOSTNAME", oldHost)
		_ = os.Setenv("WORKER_ID", oldWorker)
	})
	_ = os.Setenv("HOSTNAME", "")
	_ = os.Setenv("WORKER_ID", "worker-1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Run(ctx)
}

func appColsForWorker() []string {
	return []string{
		"id", "name", "code", "description", "openai_key",
		"enabled_languages", "created_by", "updated_by",
		"created_at", "updated_at", "deleted_at",
	}
}

func TestProcessAddLanguageJob_EarlyFailures(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	t.Run("application not found", func(t *testing.T) {
		job := &models.AddLanguageJob{ID: uuid.New(), ApplicationID: uuid.New(), Locale: "id"}
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(job.ApplicationID, 1).
			WillReturnRows(sqlmock.NewRows(appColsForWorker()))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "add_language_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		processAddLanguageJob(context.Background(), job, services.NewTranslationService())
	})

	t.Run("missing openai key", func(t *testing.T) {
		job := &models.AddLanguageJob{ID: uuid.New(), ApplicationID: uuid.New(), Locale: "id"}
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(job.ApplicationID, 1).
			WillReturnRows(sqlmock.NewRows(appColsForWorker()).
				AddRow(job.ApplicationID, "App", "app", "", "", "{en}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "add_language_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		processAddLanguageJob(context.Background(), job, services.NewTranslationService())
	})
}

func TestProcessTranslateJob_EarlyFailures(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	t.Run("application not found", func(t *testing.T) {
		job := &models.TranslateJob{ID: uuid.New(), ApplicationID: uuid.New()}
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(job.ApplicationID, 1).
			WillReturnRows(sqlmock.NewRows(appColsForWorker()))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "translate_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		processTranslateJob(context.Background(), job, services.NewTranslationService())
	})

	t.Run("no target locales", func(t *testing.T) {
		job := &models.TranslateJob{
			ID:            uuid.New(),
			ApplicationID: uuid.New(),
			ComponentID:   uuid.New(),
			SourceLocale:  "en",
			TargetLocales: models.StringArray{},
		}
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(job.ApplicationID, 1).
			WillReturnRows(sqlmock.NewRows(appColsForWorker()).
				AddRow(job.ApplicationID, "App", "app", "", "mock", "{en}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "translate_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		processTranslateJob(context.Background(), job, services.NewTranslationService())
	})

	t.Run("context cancelled before translation", func(t *testing.T) {
		job := &models.TranslateJob{
			ID:            uuid.New(),
			ApplicationID: uuid.New(),
			ComponentID:   uuid.New(),
			SourceLocale:  "en",
			TargetLocales: models.StringArray{"id"},
		}
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(job.ApplicationID, 1).
			WillReturnRows(sqlmock.NewRows(appColsForWorker()).
				AddRow(job.ApplicationID, "App", "app", "", "mock", "{en}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "translate_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		processTranslateJob(ctx, job, services.NewTranslationService())
	})

	t.Run("source translation not found", func(t *testing.T) {
		oldCache := cache.Client
		cache.Client = redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
		t.Cleanup(func() { cache.Client = oldCache })

		job := &models.TranslateJob{
			ID:            uuid.New(),
			ApplicationID: uuid.New(),
			ComponentID:   uuid.New(),
			SourceLocale:  "en",
			TargetLocales: models.StringArray{"id"},
		}
		mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(job.ApplicationID, 1).
			WillReturnRows(sqlmock.NewRows(appColsForWorker()).
				AddRow(job.ApplicationID, "App", "app", "", "mock", "{en}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil))
		mock.ExpectQuery(`SELECT .*FROM "translation_versions"`).
			WithArgs(job.ComponentID, "en", models.StageDraft, true, 1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "component_id", "locale", "stage", "version", "data", "is_active",
				"created_by", "updated_by", "created_at", "updated_at", "deleted_at",
			}))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "translate_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		processTranslateJob(context.Background(), job, services.NewTranslationService())
	})
}

func TestProcessAddLanguageJob_LoadComponentsError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	job := &models.AddLanguageJob{ID: uuid.New(), ApplicationID: uuid.New(), Locale: "id"}
	mock.ExpectQuery(`SELECT .*FROM "applications"`).WithArgs(job.ApplicationID, 1).
		WillReturnRows(sqlmock.NewRows(appColsForWorker()).
			AddRow(job.ApplicationID, "App", "app", "", "mock", "{en}", uuid.Nil, uuid.Nil, time.Now(), time.Now(), nil))
	mock.ExpectQuery(`SELECT .*FROM "components"`).WithArgs(job.ApplicationID).WillReturnError(assert.AnError)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "add_language_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	processAddLanguageJob(context.Background(), job, services.NewTranslationService())
}

// ─── Helper unit tests ────────────────────────────────────────────────────────

func TestChangedOrNewKeys_NoChange(t *testing.T) {
	current := map[string]interface{}{"title": "Hello", "count": float64(5)}
	prev := map[string]interface{}{"title": "Hello", "count": float64(5)}
	got := changedOrNewKeys(current, prev)
	assert.Empty(t, got)
}

func TestChangedOrNewKeys_ScalarChanged(t *testing.T) {
	current := map[string]interface{}{"title": "Hi", "sub": "same"}
	prev := map[string]interface{}{"title": "Hello", "sub": "same"}
	got := changedOrNewKeys(current, prev)
	assert.Equal(t, map[string]interface{}{"title": "Hi"}, got)
}

func TestChangedOrNewKeys_NewKey(t *testing.T) {
	current := map[string]interface{}{"title": "Hello", "new": "Added"}
	prev := map[string]interface{}{"title": "Hello"}
	got := changedOrNewKeys(current, prev)
	assert.Equal(t, map[string]interface{}{"new": "Added"}, got)
}

func TestChangedOrNewKeys_NestedPartial(t *testing.T) {
	current := map[string]interface{}{
		"button": map[string]interface{}{"save": "Save", "cancel": "Cancel (mod)"},
		"title":  "Hello",
	}
	prev := map[string]interface{}{
		"button": map[string]interface{}{"save": "Save", "cancel": "Cancel"},
		"title":  "Hello",
	}
	got := changedOrNewKeys(current, prev)
	// Only the changed nested key should appear, title unchanged.
	assert.NotContains(t, got, "title")
	btn, ok := got["button"].(map[string]interface{})
	require.True(t, ok, "button should be a map")
	assert.Equal(t, "Cancel (mod)", btn["cancel"])
	assert.NotContains(t, btn, "save")
}

func TestChangedOrNewKeys_TypeChanged(t *testing.T) {
	// Was a string, now a map → treat as changed.
	current := map[string]interface{}{"k": map[string]interface{}{"a": "1"}}
	prev := map[string]interface{}{"k": "old"}
	got := changedOrNewKeys(current, prev)
	assert.Contains(t, got, "k")
}

func TestHasRemovedKeys_NoRemoval(t *testing.T) {
	prev := map[string]interface{}{"a": "1"}
	current := map[string]interface{}{"a": "1", "b": "2"}
	assert.False(t, hasRemovedKeys(prev, current))
}

func TestHasRemovedKeys_TopLevel(t *testing.T) {
	prev := map[string]interface{}{"a": "1", "b": "2"}
	current := map[string]interface{}{"a": "1"}
	assert.True(t, hasRemovedKeys(prev, current))
}

func TestHasRemovedKeys_Nested(t *testing.T) {
	prev := map[string]interface{}{
		"btn": map[string]interface{}{"save": "Save", "delete": "Delete"},
	}
	current := map[string]interface{}{
		"btn": map[string]interface{}{"save": "Save"},
	}
	assert.True(t, hasRemovedKeys(prev, current))
}

func TestPruneToShape_RemovesExtraKey(t *testing.T) {
	target := map[string]interface{}{"a": "translated-a", "stale": "old"}
	source := map[string]interface{}{"a": "a"}
	got := pruneToShape(target, source)
	assert.Equal(t, "translated-a", got["a"])
	assert.NotContains(t, got, "stale")
}

func TestPruneToShape_NestedRemoval(t *testing.T) {
	target := map[string]interface{}{
		"btn": map[string]interface{}{"save": "저장", "stale": "오래됨"},
	}
	source := map[string]interface{}{
		"btn": map[string]interface{}{"save": "Save"},
	}
	got := pruneToShape(target, source)
	btn := got["btn"].(map[string]interface{})
	assert.Equal(t, "저장", btn["save"])
	assert.NotContains(t, btn, "stale")
}

func TestPruneToShape_PreservesAll(t *testing.T) {
	target := map[string]interface{}{"a": "T-a", "b": "T-b"}
	source := map[string]interface{}{"a": "a", "b": "b"}
	got := pruneToShape(target, source)
	assert.Equal(t, "T-a", got["a"])
	assert.Equal(t, "T-b", got["b"])
}

func TestMergeTranslations_Overlay(t *testing.T) {
	base := map[string]interface{}{"title": "안녕", "sub": "기존"}
	additions := map[string]interface{}{"sub": "수정됨", "new": "추가"}
	got := mergeTranslations(base, additions)
	assert.Equal(t, "안녕", got["title"])
	assert.Equal(t, "수정됨", got["sub"])
	assert.Equal(t, "추가", got["new"])
}

func TestMergeTranslations_NestedOverlay(t *testing.T) {
	base := map[string]interface{}{
		"btn": map[string]interface{}{"save": "저장", "cancel": "취소"},
	}
	additions := map[string]interface{}{
		"btn": map[string]interface{}{"cancel": "취소됨"},
	}
	got := mergeTranslations(base, additions)
	btn := got["btn"].(map[string]interface{})
	assert.Equal(t, "저장", btn["save"])
	assert.Equal(t, "취소됨", btn["cancel"])
}

func TestJsonEqual(t *testing.T) {
	assert.True(t, jsonEqual(float64(5), float64(5)))
	assert.False(t, jsonEqual(float64(5), float64(6)))
	assert.True(t, jsonEqual("hello", "hello"))
	assert.False(t, jsonEqual("hello", "world"))
	assert.True(t, jsonEqual(nil, nil))
	assert.True(t, jsonEqual(true, true))
	assert.False(t, jsonEqual(true, false))
}

func TestCountLeaves(t *testing.T) {
	m := map[string]interface{}{
		"a": "1",
		"b": map[string]interface{}{"c": "2", "d": "3"},
	}
	assert.Equal(t, 3, countLeaves(m))
}

func TestMarkTranslateJobCompleted(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old })

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "translate_jobs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	assert.NoError(t, markTranslateJobCompleted(uuid.New()))
}
