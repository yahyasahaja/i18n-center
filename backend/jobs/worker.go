package jobs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/observability"
	"github.com/your-org/i18n-center/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	pollInterval         = 5 * time.Second
	componentConcurrency = 5 // max parallel OpenAI calls within a single AddLanguageJob
)

// Run starts the in-process worker loop. Claims both AddLanguageJob and TranslateJob records
// from DB (no in-memory state); safe for multiple K8s replicas via FOR UPDATE SKIP LOCKED.
func Run(ctx context.Context) {
	instanceID := os.Getenv("HOSTNAME")
	if instanceID == "" {
		instanceID = os.Getenv("WORKER_ID")
	}
	if instanceID == "" {
		instanceID = "default"
	}

	translationService := services.NewTranslationService()

	for {
		select {
		case <-ctx.Done():
			observability.Logger.Info("Worker stopping", zap.String("instance", instanceID))
			return
		default:
		}

		// Try to claim and process one job of any type per tick
		processed := false

		if addJob, err := claimAddLanguageJob(instanceID); err != nil {
			observability.Logger.Warn("Worker claim error (AddLanguageJob)", zap.Error(err))
		} else if addJob != nil {
			processAddLanguageJob(ctx, addJob, translationService)
			processed = true
		}

		if !processed {
			if trJob, err := claimTranslateJob(instanceID); err != nil {
				observability.Logger.Warn("Worker claim error (TranslateJob)", zap.Error(err))
			} else if trJob != nil {
				processTranslateJob(ctx, trJob, translationService)
				processed = true
			}
		}

		// Poll interval
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// ─── AddLanguageJob ──────────────────────────────────────────────────────────

// claimAddLanguageJob atomically claims one pending AddLanguageJob.
func claimAddLanguageJob(instanceID string) (*models.AddLanguageJob, error) {
	db := silentDB()

	// Reset stuck running jobs
	_ = db.Exec(`
		UPDATE add_language_jobs
		SET status = $1, claimed_by = '', updated_at = NOW()
		WHERE status = $2 AND updated_at < NOW() - INTERVAL '15 minutes'
	`, models.JobStatusPending, models.JobStatusRunning)

	var idRow struct{ ID uuid.UUID }
	err := db.Raw(`
		UPDATE add_language_jobs
		SET status = $1, claimed_by = $2, updated_at = NOW()
		WHERE id = (
			SELECT id FROM add_language_jobs
			WHERE status = $3
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id
	`, models.JobStatusRunning, instanceID, models.JobStatusPending).Scan(&idRow).Error
	if err != nil {
		return nil, err
	}
	if idRow.ID == uuid.Nil {
		return nil, nil
	}
	var job models.AddLanguageJob
	if err := db.First(&job, "id = ?", idRow.ID).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// processAddLanguageJob runs the add-language auto-translate logic with a goroutine pool
// (componentConcurrency parallel OpenAI calls). All state in DB; safe for K8s.
func processAddLanguageJob(ctx context.Context, job *models.AddLanguageJob, translationService *services.TranslationService) {
	appIDStr := job.ApplicationID.String()
	defer func() {
		if r := recover(); r != nil {
			observability.Logger.Error("Worker panic (AddLanguageJob)", zap.Any("panic", r), zap.String("job_id", job.ID.String()))
			_ = markAddJobFailed(job.ID, "Worker panic", fmt.Sprintf("%v", r))
		}
	}()

	var application models.Application
	if err := database.DB.First(&application, "id = ?", job.ApplicationID).Error; err != nil {
		_ = markAddJobFailed(job.ID, "Application not found", err.Error())
		return
	}

	openAIService := resolveOpenAIService(application.OpenAIKey)
	if openAIService == nil {
		_ = markAddJobFailed(job.ID, "OpenAI API key not configured", "Configure in Application settings")
		return
	}

	var components []models.Component
	if err := database.DB.Where("application_id = ?", job.ApplicationID).Find(&components).Error; err != nil {
		_ = markAddJobFailed(job.ID, "Failed to load components", err.Error())
		return
	}

	// Record total up-front so the frontend can show "X / N" immediately.
	_ = database.DB.Model(&models.AddLanguageJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"total_components":     len(components),
		"completed_components": 0,
		"updated_at":           time.Now(),
	}).Error

	// Goroutine pool — semaphore pattern
	type result struct {
		versionID uuid.UUID
		err       error
		compCode  string
	}

	sem := make(chan struct{}, componentConcurrency)
	results := make(chan result, len(components))
	var wg sync.WaitGroup

	for _, comp := range components {
		wg.Add(1)
		comp := comp      // capture loop variable
		sem <- struct{}{} // acquire slot (blocks when all workers are busy)
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release slot

			// Bail early if context was cancelled while we were waiting for a slot
			if err := ctx.Err(); err != nil {
				results <- result{err: fmt.Errorf("component %s: cancelled", comp.Code), compCode: comp.Code}
				return
			}

			sourceTranslation, err := translationService.GetTranslation(comp.ID, comp.DefaultLocale, models.StageDraft)
			if err != nil {
				results <- result{err: fmt.Errorf("component %s: no draft translation for default locale %s", comp.Code, comp.DefaultLocale), compCode: comp.Code}
				// Still count as "processed" so the progress bar advances even on per-component errors.
				_ = database.DB.Exec(
					"UPDATE add_language_jobs SET completed_components = completed_components + 1, updated_at = NOW() WHERE id = ?",
					job.ID,
				).Error
				return
			}

			// Pass ctx so the HTTP call to OpenAI can be cancelled on SIGTERM
			translatedData, err := openAIService.TranslateJSON(ctx, sourceTranslation.Data, comp.DefaultLocale, job.Locale)
			if err != nil {
				results <- result{err: fmt.Errorf("component %s: %w", comp.Code, err), compCode: comp.Code}
				_ = database.DB.Exec(
					"UPDATE add_language_jobs SET completed_components = completed_components + 1, updated_at = NOW() WHERE id = ?",
					job.ID,
				).Error
				return
			}

			tr, err := translationService.SaveTranslation(comp.ID, job.Locale, models.StageDraft, translatedData, job.CreatedBy)
			if err != nil {
				results <- result{err: fmt.Errorf("component %s: %w", comp.Code, err), compCode: comp.Code}
				_ = database.DB.Exec(
					"UPDATE add_language_jobs SET completed_components = completed_components + 1, updated_at = NOW() WHERE id = ?",
					job.ID,
				).Error
				return
			}

			_ = database.DB.Exec(
				"UPDATE add_language_jobs SET completed_components = completed_components + 1, updated_at = NOW() WHERE id = ?",
				job.ID,
			).Error
			results <- result{versionID: tr.ID}
		}()
	}

	wg.Wait()
	close(results)

	// Check for context cancellation after draining goroutines
	select {
	case <-ctx.Done():
		// All goroutines finished — results are in the channel. We'll roll back everything below.
		_ = markAddJobFailed(job.ID, "Worker cancelled", "Context cancelled")
		// Drain results to collect any IDs that were saved before cancellation
		for r := range results {
			if r.versionID != uuid.Nil {
				_ = translationService.DeleteTranslationVersionByID(r.versionID)
			}
		}
		return
	default:
	}

	// Collect ALL results before deciding — ensures full rollback even when failures are interleaved
	var createdIDs []uuid.UUID
	var firstErr error
	for r := range results {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		if r.versionID != uuid.Nil {
			createdIDs = append(createdIDs, r.versionID)
		}
	}
	if firstErr != nil {
		for _, vid := range createdIDs {
			_ = translationService.DeleteTranslationVersionByID(vid)
		}
		_ = markAddJobFailed(job.ID, "Translation process failed (rolled back)", firstErr.Error())
		return
	}

	// Create/reset deploy tracking.
	// Two cases to handle:
	//   1. Locale was deleted while job was running → locale no longer in enabled_languages;
	//      skip creating the deploy record to avoid ghost entries in pending deploys.
	//   2. Locale was previously deployed to production and is being re-translated;
	//      the record already exists so Create would fail — upsert back to 'draft' instead.
	var currentApp models.Application
	localeStillEnabled := false
	if err := database.DB.Select("enabled_languages").First(&currentApp, job.ApplicationID).Error; err == nil {
		for _, l := range currentApp.EnabledLanguages {
			if strings.EqualFold(l, job.Locale) {
				localeStillEnabled = true
				break
			}
		}
	}
	if localeStillEnabled {
		var existingDeploy models.ApplicationLocaleDeploy
		err := database.DB.Where("application_id = ? AND locale = ?", job.ApplicationID, job.Locale).First(&existingDeploy).Error
		if err != nil {
			// No record yet — create fresh
			newDeploy := models.ApplicationLocaleDeploy{
				ApplicationID:  job.ApplicationID,
				Locale:         job.Locale,
				StageCompleted: "draft",
			}
			if err := database.DB.Create(&newDeploy).Error; err != nil {
				observability.Logger.Warn("Failed to create ApplicationLocaleDeploy", zap.Error(err))
			}
		} else {
			// Record exists (e.g. locale was re-translated after production) — reset to draft
			if err := database.DB.Model(&existingDeploy).Update("stage_completed", "draft").Error; err != nil {
				observability.Logger.Warn("Failed to reset ApplicationLocaleDeploy to draft", zap.Error(err))
			}
		}
	} else {
		observability.Logger.Info("Skipping deploy record: locale was removed while job ran",
			zap.String("job_id", job.ID.String()),
			zap.String("locale", job.Locale),
		)
	}

	cache.Delete(cache.ApplicationKey(appIDStr))
	_ = database.DB.Model(&models.AddLanguageJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"status":     models.JobStatusCompleted,
		"updated_at": time.Now(),
	}).Error
	observability.Logger.Info("AddLanguageJob completed",
		zap.String("job_id", job.ID.String()),
		zap.String("locale", job.Locale),
		zap.Int("components", len(components)),
	)
}

func markAddJobFailed(jobID uuid.UUID, msg, detail string) error {
	return database.DB.Model(&models.AddLanguageJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        models.JobStatusFailed,
		"error_message": msg,
		"error_detail":  detail,
		"updated_at":    time.Now(),
	}).Error
}

// GetJobStatus returns AddLanguageJob by ID and application ID (for auth scope). Used by API handler.
func GetJobStatus(applicationID, jobID uuid.UUID) (*models.AddLanguageJob, error) {
	var job models.AddLanguageJob
	err := database.DB.Where("id = ? AND application_id = ?", jobID, applicationID).First(&job).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

// ─── TranslateJob ─────────────────────────────────────────────────────────────

// claimTranslateJob atomically claims one pending TranslateJob.
func claimTranslateJob(instanceID string) (*models.TranslateJob, error) {
	db := silentDB()

	// Reset stuck running jobs
	_ = db.Exec(`
		UPDATE translate_jobs
		SET status = $1, claimed_by = '', updated_at = NOW()
		WHERE status = $2 AND updated_at < NOW() - INTERVAL '15 minutes'
	`, models.JobStatusPending, models.JobStatusRunning)

	var idRow struct{ ID uuid.UUID }
	err := db.Raw(`
		UPDATE translate_jobs
		SET status = $1, claimed_by = $2, updated_at = NOW()
		WHERE id = (
			SELECT id FROM translate_jobs
			WHERE status = $3
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id
	`, models.JobStatusRunning, instanceID, models.JobStatusPending).Scan(&idRow).Error
	if err != nil {
		return nil, err
	}
	if idRow.ID == uuid.Nil {
		return nil, nil
	}
	var job models.TranslateJob
	if err := db.First(&job, "id = ?", idRow.ID).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// processTranslateJob handles both auto_translate and backfill job types.
// Each TranslateJob carries exactly one target locale (backfill is fanned out by the handler).
func processTranslateJob(ctx context.Context, job *models.TranslateJob, translationService *services.TranslationService) {
	defer func() {
		if r := recover(); r != nil {
			observability.Logger.Error("Worker panic (TranslateJob)", zap.Any("panic", r), zap.String("job_id", job.ID.String()))
			_ = markTranslateJobFailed(job.ID, "Worker panic", fmt.Sprintf("%v", r))
		}
	}()

	var application models.Application
	if err := database.DB.First(&application, "id = ?", job.ApplicationID).Error; err != nil {
		_ = markTranslateJobFailed(job.ID, "Application not found", err.Error())
		return
	}

	openAIService := resolveOpenAIService(application.OpenAIKey)
	if openAIService == nil {
		_ = markTranslateJobFailed(job.ID, "OpenAI API key not configured", "Configure in Application settings")
		return
	}

	if len(job.TargetLocales) == 0 {
		_ = markTranslateJobFailed(job.ID, "No target locales specified", "")
		return
	}
	targetLocale := job.TargetLocales[0]

	// Check context before starting the potentially slow OpenAI call
	select {
	case <-ctx.Done():
		_ = markTranslateJobFailed(job.ID, "Worker cancelled", "Context cancelled")
		return
	default:
	}

	sourceTranslation, err := translationService.GetTranslation(job.ComponentID, job.SourceLocale, models.StageDraft)
	if err != nil {
		_ = markTranslateJobFailed(job.ID, "Source translation not found",
			fmt.Sprintf("component %s locale %s: %v", job.ComponentID, job.SourceLocale, err))
		return
	}

	translatedData, err := openAIService.TranslateJSON(ctx, sourceTranslation.Data, job.SourceLocale, targetLocale)
	if err != nil {
		_ = markTranslateJobFailed(job.ID, "Translation failed", err.Error())
		return
	}

	if _, err := translationService.SaveTranslation(job.ComponentID, targetLocale, models.StageDraft, translatedData, job.CreatedBy); err != nil {
		_ = markTranslateJobFailed(job.ID, "Failed to save translation", err.Error())
		return
	}

	cache.Delete(cache.ApplicationKey(job.ApplicationID.String()))
	_ = database.DB.Model(&models.TranslateJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"status":     models.JobStatusCompleted,
		"updated_at": time.Now(),
	}).Error
	observability.Logger.Info("TranslateJob completed",
		zap.String("job_id", job.ID.String()),
		zap.String("component_id", job.ComponentID.String()),
		zap.String("target_locale", targetLocale),
		zap.String("job_type", job.JobType),
	)
}

func markTranslateJobFailed(jobID uuid.UUID, msg, detail string) error {
	return database.DB.Model(&models.TranslateJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        models.JobStatusFailed,
		"error_message": msg,
		"error_detail":  detail,
		"updated_at":    time.Now(),
	}).Error
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// silentDB returns a gorm session with logging suppressed (idle poll queries shouldn't flood logs).
func silentDB() *gorm.DB {
	return database.DB.Session(&gorm.Session{Logger: database.DB.Logger.LogMode(logger.Silent)})
}

// resolveOpenAIService returns an OpenAIService using the app's key or the environment fallback.
// Returns nil if no key is available at all.
func resolveOpenAIService(appKey string) *services.OpenAIService {
	key := appKey
	if key == "" {
		key = services.GetDefaultOpenAIKey()
	}
	if key == "" {
		return nil
	}
	return services.NewOpenAIService(key)
}
