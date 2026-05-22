// services/cms_localization_service.go contains the legacy SaveCmsLocalizationVersion
// helper. As of Commit G it's a thin wrapper around cms.LocalizationRepository
// — the race-retry logic moved into the repository layer. The exported function
// stays so the worker (still on GORM until Commit H) doesn't need to change.
//
// TODO(commit H): once the worker is converted, delete this file and call
// cms.LocalizationRepository.SaveLocalizationVersion directly.
package services

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/cms"
	"github.com/your-org/i18n-center/repository/translation"
)

// SaveCmsLocalizationVersion preserves the GORM-era signature for the worker.
// The tx parameter is accepted but only used as a signal — when non-nil we
// still route through the sqlx repository against database.SQLX, accepting
// that the outer GORM tx and this insert are NOT atomic with each other.
//
// This is a transitional bridge. Commit H rewrites the worker to use sqlx
// transactions and the repository directly; this function gets deleted then.
func SaveCmsLocalizationVersion(tx *gorm.DB, cmsItemID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, sourceLocale string, userID uuid.UUID) (*models.CmsLocalization, error) {
	repo := cms.NewLocalizationRepository()
	l := &cms.Localization{
		CmsItemID:    cmsItemID,
		Locale:       locale,
		Stage:        translation.Stage(stage),
		Data:         repository.JSONB(data),
		SourceLocale: sourceLocale,
		IsActive:     true,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}
	if err := repo.SaveLocalizationVersion(context.Background(), database.SQLX, l); err != nil {
		return nil, err
	}
	// Translate back to the GORM model shape the worker expects.
	return &models.CmsLocalization{
		ID:           l.ID,
		CmsItemID:    l.CmsItemID,
		Locale:       l.Locale,
		Stage:        stage,
		Version:      l.Version,
		Data:         data,
		SourceLocale: l.SourceLocale,
		IsActive:     l.IsActive,
		CreatedBy:    l.CreatedBy,
		UpdatedBy:    l.UpdatedBy,
		CreatedAt:    l.CreatedAt,
		UpdatedAt:    l.UpdatedAt,
	}, nil
}
