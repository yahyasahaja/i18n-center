package jobs

import (
	"context"
	"fmt"
	"os"
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

const pollInterval = 5 * time.Second

// Run starts the in-process worker loop. Claims jobs from DB (no in-memory state); safe for multiple K8s replicas.
func Run(ctx context.Context) {
	instanceID := os.Getenv("HOSTNAME") // K8s sets this per pod
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
			job, err := claimJob(instanceID)
			if err != nil {
				observability.Logger.Warn("Worker claim error", zap.Error(err))
			}
			if job != nil {
				processJob(ctx, job, translationService)
			}
		}

		// Poll interval (no in-memory queue; always poll DB)
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// claimJob atomically claims one pending job. Uses FOR UPDATE SKIP LOCKED so only one replica gets each job (K8s-safe).
// Uses a silent DB session so idle polling does not flood logs.
func claimJob(instanceID string) (*models.AddLanguageJob, error) {
	db := database.DB.Session(&gorm.Session{Logger: database.DB.Logger.LogMode(logger.Silent)})

	// Reset stuck jobs (running too long) so they can be retried
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


// processJob runs the add-language auto-translate logic and updates job status. All state in DB.
func processJob(ctx context.Context, job *models.AddLanguageJob, translationService *services.TranslationService) {
	appIDStr := job.ApplicationID.String()
	defer func() {
		// Ensure job status is updated on panic
		if r := recover(); r != nil {
			observability.Logger.Error("Worker panic", zap.Any("panic", r), zap.String("job_id", job.ID.String()))
			_ = markJobFailed(job.ID, "Worker panic", fmt.Sprintf("%v", r))
		}
	}()

	var application models.Application
	if err := database.DB.First(&application, "id = ?", job.ApplicationID).Error; err != nil {
		_ = markJobFailed(job.ID, "Application not found", err.Error())
		return
	}

	openAIService := services.NewOpenAIService(application.OpenAIKey)
	if application.OpenAIKey == "" {
		openAIService = services.NewOpenAIService(services.GetDefaultOpenAIKey())
	}
	if openAIService.APIKey == "" {
		_ = markJobFailed(job.ID, "OpenAI API key not configured", "Configure in Application settings")
		return
	}

	var components []models.Component
	if err := database.DB.Where("application_id = ?", job.ApplicationID).Find(&components).Error; err != nil {
		_ = markJobFailed(job.ID, "Failed to load components", err.Error())
		return
	}

	var createdVersionIDs []uuid.UUID
	for _, comp := range components {
		select {
		case <-ctx.Done():
			for _, vid := range createdVersionIDs {
				_ = translationService.DeleteTranslationVersionByID(vid)
			}
			_ = markJobFailed(job.ID, "Worker cancelled", "Context cancelled")
			return
		default:
		}

		sourceTranslation, err := translationService.GetTranslation(comp.ID, comp.DefaultLocale, models.StageDraft)
		if err != nil {
			for _, vid := range createdVersionIDs {
				_ = translationService.DeleteTranslationVersionByID(vid)
			}
			_ = markJobFailed(job.ID, "Translation process failed (rolled back)",
				fmt.Sprintf("Component %s: no draft translation for default locale %s", comp.Code, comp.DefaultLocale))
			return
		}

		translatedData, err := openAIService.TranslateJSON(sourceTranslation.Data, comp.DefaultLocale, job.Locale)
		if err != nil {
			for _, vid := range createdVersionIDs {
				_ = translationService.DeleteTranslationVersionByID(vid)
			}
			_ = markJobFailed(job.ID, "Translation process failed (rolled back)",
				fmt.Sprintf("Component %s: %v", comp.Code, err))
			return
		}

		tr, err := translationService.SaveTranslation(comp.ID, job.Locale, models.StageDraft, translatedData, job.CreatedBy)
		if err != nil {
			for _, vid := range createdVersionIDs {
				_ = translationService.DeleteTranslationVersionByID(vid)
			}
			_ = markJobFailed(job.ID, "Translation process failed (rolled back)",
				fmt.Sprintf("Component %s: %v", comp.Code, err))
			return
		}
		createdVersionIDs = append(createdVersionIDs, tr.ID)
	}

	// Create deploy tracking so locale appears in "Pending locale deploys"
	deploy := models.ApplicationLocaleDeploy{
		ApplicationID:  job.ApplicationID,
		Locale:         job.Locale,
		StageCompleted: "draft",
	}
	if err := database.DB.Create(&deploy).Error; err != nil {
		observability.Logger.Warn("Failed to create ApplicationLocaleDeploy", zap.Error(err))
	}

	cache.Delete(cache.ApplicationKey(appIDStr))
	_ = database.DB.Model(&models.AddLanguageJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"status":       models.JobStatusCompleted,
		"updated_at":   time.Now(),
	}).Error
	observability.Logger.Info("AddLanguageJob completed", zap.String("job_id", job.ID.String()), zap.String("locale", job.Locale))
}

func markJobFailed(jobID uuid.UUID, msg, detail string) error {
	return database.DB.Model(&models.AddLanguageJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        models.JobStatusFailed,
		"error_message": msg,
		"error_detail":  detail,
		"updated_at":    time.Now(),
	}).Error
}

// GetJobStatus returns job by ID and application ID (for auth scope). Used by API handler.
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
