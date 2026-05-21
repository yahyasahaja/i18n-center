package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/observability"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TranslationService struct{}

func NewTranslationService() *TranslationService {
	return &TranslationService{}
}

// InvalidateAfterTranslationWrite busts every cache that could now be stale because
// a translation version for (componentID, locale, stage) was just written, reverted,
// or deployed.
//
// We blow away:
//   - translation:{componentID}:{locale}:{stage}                                   (single-component key)
//   - component:{componentID}                                                      (component metadata key)
//   - translations:bypage:{appID}:*:{locale}:{stage}  (only the affected cell)
//   - translations:bytag:{appID}:*:{locale}:{stage}   (only the affected cell)
//
// Pattern delete is scoped to (appID, locale, stage) — only pages/tags in that cell
// can have included this component's data. A draft write therefore never busts the
// production aggregate cache, which matters during batch jobs (add-language fanout
// can do hundreds of writes; we don't want each one walking the production keyspace).
//
// Errors are logged, never returned: cache busting must never block a write.
func InvalidateAfterTranslationWrite(componentID uuid.UUID, locale, stage string) {
	cache.Delete(cache.TranslationKey(componentID.String(), locale, stage))
	cache.Delete(cache.ComponentKey(componentID.String()))

	var component models.Component
	if err := database.DB.Select("application_id").First(&component, "id = ?", componentID).Error; err != nil {
		observability.Logger.Warn("cache invalidate: component not found, skipping aggregate delete",
			zap.String("component_id", componentID.String()),
			zap.Error(err),
		)
		return
	}
	invalidateAggregateCache(component.ApplicationID.String(), locale, stage)
}

// InvalidateApplicationReadCache busts every aggregate read cache for an application,
// across all locales and stages. Used when a change affects many components at once
// (locale removed, bulk deploy to all components).
func InvalidateApplicationReadCache(applicationID uuid.UUID) {
	appID := applicationID.String()
	cache.Delete(cache.ApplicationKey(appID))
	for _, prefix := range []string{"translations:bypage", "translations:bytag"} {
		pattern := fmt.Sprintf("%s:%s:*", prefix, appID)
		if err := cache.DeletePattern(pattern); err != nil {
			observability.Logger.Warn("cache invalidate: app-wide pattern delete failed",
				zap.String("pattern", pattern),
				zap.Error(err),
			)
		}
	}
}

func invalidateAggregateCache(appID, locale, stage string) {
	for _, prefix := range []string{"translations:bypage", "translations:bytag"} {
		// Pattern: prefix:{appID}:*:{locale}:{stage} — pages/tags wildcarded, locale+stage exact.
		pattern := fmt.Sprintf("%s:%s:*:%s:%s", prefix, appID, locale, stage)
		if err := cache.DeletePattern(pattern); err != nil {
			observability.Logger.Warn("cache invalidate: scoped pattern delete failed",
				zap.String("pattern", pattern),
				zap.Error(err),
			)
		}
	}
}

// GetTranslation retrieves the latest translation version for a component by locale and stage
func (s *TranslationService) GetTranslation(componentID uuid.UUID, locale string, stage models.DeploymentStage) (*models.TranslationVersion, error) {
	// Try cache first
	cacheKey := cache.TranslationKey(componentID.String(), locale, string(stage))
	var cached models.TranslationVersion
	if err := cache.Get(cacheKey, &cached); err == nil {
		return &cached, nil
	}

	var translation models.TranslationVersion
	result := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND is_active = ?",
		componentID, locale, stage, true).Order("version DESC").First(&translation)
	if result.Error != nil {
		return nil, result.Error
	}

	cache.Set(cacheKey, translation, 3600*1000000000) // 1 hour
	return &translation, nil
}

// GetMultipleTranslationsByCodes retrieves translations for multiple components by codes
// Uses Redis cache efficiently - checks cache first, then database
// applicationCode is required to differentiate components with the same code in different applications
func (s *TranslationService) GetMultipleTranslationsByCodes(applicationCode string, componentCodes []string, locale string, stage models.DeploymentStage) (map[string]*models.TranslationVersion, error) {
	// First, get the application by code
	var application models.Application
	if err := database.DB.Where("code = ?", applicationCode).First(&application).Error; err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// Resolve codes to component IDs, filtered by application
	var components []models.Component
	if err := database.DB.Where("code IN ? AND application_id = ?", componentCodes, application.ID).Find(&components).Error; err != nil {
		return nil, fmt.Errorf("failed to resolve component codes: %w", err)
	}

	// Create mapping: code -> component ID
	codeToID := make(map[string]uuid.UUID)
	idToCode := make(map[uuid.UUID]string)
	componentIDs := make([]uuid.UUID, 0, len(components))

	for _, component := range components {
		codeToID[component.Code] = component.ID
		idToCode[component.ID] = component.Code
		componentIDs = append(componentIDs, component.ID)
	}

	// Check for missing codes
	missingCodes := make([]string, 0)
	for _, code := range componentCodes {
		if _, exists := codeToID[code]; !exists {
			missingCodes = append(missingCodes, code)
		}
	}

	if len(missingCodes) > 0 {
		return nil, fmt.Errorf("component codes not found: %v", missingCodes)
	}

	// Get translations by IDs
	translations, err := s.GetMultipleTranslations(componentIDs, locale, stage)
	if err != nil {
		return nil, err
	}

	// Map results back to codes
	result := make(map[string]*models.TranslationVersion)
	for _, translation := range translations {
		if code, exists := idToCode[translation.ComponentID]; exists {
			result[code] = translation
		}
	}

	return result, nil
}

// GetMultipleTranslations retrieves translations for multiple components
// Uses Redis cache efficiently - checks cache first, then database
func (s *TranslationService) GetMultipleTranslations(componentIDs []uuid.UUID, locale string, stage models.DeploymentStage) (map[string]*models.TranslationVersion, error) {
	results := make(map[string]*models.TranslationVersion)
	missingFromCache := make([]uuid.UUID, 0)

	// First pass: Try to get from cache
	for _, componentID := range componentIDs {
		cacheKey := cache.TranslationKey(componentID.String(), locale, string(stage))
		var cached models.TranslationVersion
		if err := cache.Get(cacheKey, &cached); err == nil {
			results[componentID.String()] = &cached
		} else {
			missingFromCache = append(missingFromCache, componentID)
		}
	}

	// Second pass: Get latest version per component from database (PostgreSQL DISTINCT ON)
	if len(missingFromCache) > 0 {
		var translations []models.TranslationVersion
		err := database.DB.Raw(`
			SELECT DISTINCT ON (component_id) *
			FROM translation_versions
			WHERE component_id IN ? AND locale = ? AND stage = ? AND is_active = ?
			ORDER BY component_id, version DESC
		`, missingFromCache, locale, stage, true).Scan(&translations).Error

		if err == nil {
			for i := range translations {
				translation := translations[i]
				results[translation.ComponentID.String()] = &translation
				cacheKey := cache.TranslationKey(translation.ComponentID.String(), locale, string(stage))
				cache.Set(cacheKey, translation, 3600*1000000000)
			}
		}
	}

	return results, nil
}

// SaveTranslation adds a new version (always insert, never overwrite).
// Used for manual edits — no source snapshot is recorded.
func (s *TranslationService) SaveTranslation(componentID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, userID uuid.UUID) (*models.TranslationVersion, error) {
	return s.saveVersion(componentID, locale, stage, data, "", nil, userID)
}

// SaveTranslationWithSource adds a new version and records the source locale + source data
// snapshot that was used to produce this translation. The snapshot enables incremental
// re-translation: next time only changed/new keys are sent to the AI.
func (s *TranslationService) SaveTranslationWithSource(componentID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, sourceLocale string, sourceData models.JSONB, userID uuid.UUID) (*models.TranslationVersion, error) {
	return s.saveVersion(componentID, locale, stage, data, sourceLocale, sourceData, userID)
}

// saveVersion is the shared insert implementation for both Save* methods.
// Uses the global DB connection; invalidates cache and triggers cleanup.
func (s *TranslationService) saveVersion(componentID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, sourceLocale string, sourceData models.JSONB, userID uuid.UUID) (*models.TranslationVersion, error) {
	v, err := s.saveVersionTx(nil, componentID, locale, stage, data, sourceLocale, sourceData, userID)
	if err != nil {
		return nil, err
	}
	InvalidateAfterTranslationWrite(componentID, locale, string(stage))
	return v, nil
}

// saveVersionTx is the tx-aware variant. Pass tx=nil to use the global DB.
//
// Concurrency: two writers can compute the same nextVersion under load (read of
// MAX(version) is not atomic with the Create). A partial unique index on
// (component_id, locale, stage, version) WHERE deleted_at IS NULL turns the
// collision into a duplicate-key error that we retry by re-reading MAX(version).
// The retry budget is intentionally small — a high collision count means the
// caller is hammering one (component, locale, stage), which is application-level
// pathological, not normal load.
//
// NOTE: when tx != nil this function does NOT invalidate cache or run
// CleanupOldVersions — the caller must do that after the outer tx commits,
// otherwise rolled-back writes would still bust caches and trigger cleanups.
func (s *TranslationService) saveVersionTx(tx *gorm.DB, componentID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, sourceLocale string, sourceData models.JSONB, userID uuid.UUID) (*models.TranslationVersion, error) {
	db := tx
	if db == nil {
		db = database.DB
	}

	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		nextVersion := 1
		var current models.TranslationVersion
		err := db.Where("component_id = ? AND locale = ? AND stage = ?",
			componentID, locale, stage).Order("version DESC").First(&current).Error
		if err == nil {
			nextVersion = current.Version + 1
		}

		newVersion := models.TranslationVersion{
			ComponentID:  componentID,
			Locale:       locale,
			Stage:        stage,
			Version:      nextVersion,
			Data:         data,
			SourceLocale: sourceLocale,
			SourceData:   sourceData,
			IsActive:     true,
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}
		createErr := db.Create(&newVersion).Error
		if createErr == nil {
			// Old-version cleanup runs out-of-band on a periodic ticker (see
			// jobs.RunCleanupTicker). Running it on the write path adds an
			// unbounded sort over translation_versions to every save and offers
			// no correctness guarantee — only retention enforcement.
			return &newVersion, nil
		}
		lastErr = createErr
		if !isUniqueViolation(createErr) {
			return nil, createErr
		}
		// Lost the race — another writer used our nextVersion. Re-read and retry.
	}
	return nil, fmt.Errorf("saveVersion: exhausted %d retries on unique version conflict: %w", maxAttempts, lastErr)
}

// isUniqueViolation returns true if err is a Postgres SQLSTATE 23505 (unique_violation).
// We match on the error message to avoid taking a hard dependency on jackc/pgconn.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "unique constraint")
}

// RevertTranslation adds a new version with the previous version's data (undo as new version)
func (s *TranslationService) RevertTranslation(componentID uuid.UUID, locale string, stage models.DeploymentStage, userID uuid.UUID) error {
	var current, previous models.TranslationVersion
	err := database.DB.Where("component_id = ? AND locale = ? AND stage = ?",
		componentID, locale, stage).Order("version DESC").First(&current).Error
	if err != nil {
		return fmt.Errorf("no current version found: %w", err)
	}
	err = database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, current.Version-1).First(&previous).Error
	if err != nil {
		return fmt.Errorf("no previous version found: %w", err)
	}

	newVersion := models.TranslationVersion{
		ComponentID: componentID,
		Locale:      locale,
		Stage:       stage,
		Version:     current.Version + 1,
		Data:        previous.Data,
		IsActive:    true,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}
	if err := database.DB.Create(&newVersion).Error; err != nil {
		return err
	}
	InvalidateAfterTranslationWrite(componentID, locale, string(stage))
	return nil
}

// ListVersions returns all versions for a component/locale/stage, newest first
func (s *TranslationService) ListVersions(componentID uuid.UUID, locale string, stage models.DeploymentStage) ([]models.TranslationVersion, error) {
	var versions []models.TranslationVersion
	err := database.DB.Where("component_id = ? AND locale = ? AND stage = ?",
		componentID, locale, stage).Order("version DESC").Find(&versions).Error
	return versions, err
}

// GetVersionByNumber returns a specific version or nil if not found
func (s *TranslationService) GetVersionByNumber(componentID uuid.UUID, locale string, stage models.DeploymentStage, version int) (*models.TranslationVersion, error) {
	var v models.TranslationVersion
	err := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, version).First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// DeleteTranslationVersionByID deletes a translation version by ID (for rollback). Invalidates cache.
func (s *TranslationService) DeleteTranslationVersionByID(id uuid.UUID) error {
	var v models.TranslationVersion
	if err := database.DB.First(&v, "id = ?", id).Error; err != nil {
		return err
	}
	if err := database.DB.Delete(&v).Error; err != nil {
		return err
	}
	InvalidateAfterTranslationWrite(v.ComponentID, v.Locale, string(v.Stage))
	return nil
}

// DeployToStage deploys translations from draft to staging or staging to production,
// outside of any transaction. Use DeployToStageTx when participating in a batch.
func (s *TranslationService) DeployToStage(componentID uuid.UUID, locale string, fromStage, toStage models.DeploymentStage, userID uuid.UUID) error {
	return s.DeployToStageTx(nil, componentID, locale, fromStage, toStage, userID)
}

// DeployToStageTx is the tx-aware variant. Pass tx=nil to use the global DB; pass
// a *gorm.DB from inside a Transaction callback to participate in the outer
// transaction. The function does NOT invalidate cache — the caller does that
// after the outer tx commits.
func (s *TranslationService) DeployToStageTx(tx *gorm.DB, componentID uuid.UUID, locale string, fromStage, toStage models.DeploymentStage, userID uuid.UUID) error {
	db := tx
	if db == nil {
		db = database.DB
	}
	// Read the latest fromStage version for this locale directly from db so we
	// see uncommitted writes inside the same transaction.
	var source models.TranslationVersion
	if err := db.Where("component_id = ? AND locale = ? AND stage = ? AND is_active = ?",
		componentID, locale, fromStage, true).Order("version DESC").First(&source).Error; err != nil {
		return fmt.Errorf("source translation not found: %w", err)
	}

	if _, err := s.saveVersionTx(db, componentID, locale, toStage, source.Data, "", nil, userID); err != nil {
		return err
	}
	return nil
}

// ExtractTemplateValues extracts template variable names from text (without brackets).
// e.g. "Hi [name]!" → ["name"]
func ExtractTemplateValues(text string) []string {
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	matches := re.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			values = append(values, match[1])
		}
	}
	return values
}

// ExtractTemplatePlaceholders extracts full bracketed placeholders from text.
// e.g. "Hi [name], you have [count] items" → ["[name]", "[count]"]
// Used by validatePlaceholders to check placeholder survival after translation.
func ExtractTemplatePlaceholders(text string) []string {
	re := regexp.MustCompile(`\[[^\]]+\]`)
	matches := re.FindAllString(text, -1)
	if matches == nil {
		return []string{}
	}
	return matches
}

// PreserveTemplateValues ensures every [placeholder] from the source text is
// present in the translated text. Strategy:
//  1. If the placeholder already survived — nothing to do.
//  2. If the translated text has a bracket token in the same ordinal position
//     (GPT changed the variable name but kept the brackets) — replace it with
//     the original placeholder.
//  3. If the placeholder is completely absent and there's no bracket to swap —
//     append it to the end of the string so it's never silently lost.
func PreserveTemplateValues(text string, translatedText string) string {
	placeholders := ExtractTemplatePlaceholders(text)
	if len(placeholders) == 0 {
		return translatedText
	}

	re := regexp.MustCompile(`\[[^\]]+\]`)
	result := translatedText

	for i, placeholder := range placeholders {
		if strings.Contains(result, placeholder) {
			continue // already present — nothing to do
		}

		// Try ordinal swap: replace the i-th bracket token in the translated text
		translatedMatches := re.FindAllString(result, -1)
		if i < len(translatedMatches) {
			result = strings.Replace(result, translatedMatches[i], placeholder, 1)
			continue
		}

		// Last resort: append so we never lose the placeholder
		result = strings.TrimSpace(result) + " " + placeholder
	}

	return result
}
