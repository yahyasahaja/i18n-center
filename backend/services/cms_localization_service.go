package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"gorm.io/gorm"
)

// SaveCmsLocalizationVersion inserts a new CmsLocalization row with the next
// version number for (cmsItemID, locale, stage). The MAX(version)+1 read+insert
// is racy under concurrent writes — two callers can pick the same number — so
// we rely on idx_cms_loc_unique_version (partial unique index) to catch the
// collision and retry. Same pattern as TranslationService.saveVersion.
//
// Pass tx != nil to participate in an outer transaction. The function does NOT
// invalidate cache or trigger any retention sweep — callers handle that.
//
// Used by: cms_item_handler (SaveLocalization, DeployLocalization, RevertLocalization)
// and jobs/worker.go (processCmsTranslateJob).
func SaveCmsLocalizationVersion(tx *gorm.DB, cmsItemID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, sourceLocale string, userID uuid.UUID) (*models.CmsLocalization, error) {
	db := tx
	if db == nil {
		db = database.DB
	}

	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var latestVersion int
		db.Model(&models.CmsLocalization{}).
			Where("cms_item_id = ? AND locale = ? AND stage = ?", cmsItemID, locale, stage).
			Select("COALESCE(MAX(version), 0)").
			Scan(&latestVersion)

		loc := models.CmsLocalization{
			CmsItemID:    cmsItemID,
			Locale:       locale,
			Stage:        stage,
			Version:      latestVersion + 1,
			Data:         data,
			SourceLocale: sourceLocale,
			IsActive:     true,
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}
		if err := db.Create(&loc).Error; err == nil {
			return &loc, nil
		} else {
			lastErr = err
			if !IsUniqueViolation(err) {
				return nil, err
			}
			// Lost the race — another writer used our version number. Re-read MAX and retry.
		}
	}
	return nil, fmt.Errorf("SaveCmsLocalizationVersion: exhausted %d retries on unique version conflict: %w", maxAttempts, lastErr)
}
