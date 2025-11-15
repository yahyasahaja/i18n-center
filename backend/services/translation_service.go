package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"gorm.io/gorm"
)

type TranslationService struct{}

func NewTranslationService() *TranslationService {
	return &TranslationService{}
}

// GetTranslation retrieves translation for a component by locale and stage
func (s *TranslationService) GetTranslation(componentID uuid.UUID, locale string, stage models.DeploymentStage) (*models.TranslationVersion, error) {
	// Try cache first
	cacheKey := cache.TranslationKey(componentID.String(), locale, string(stage))
	var cached models.TranslationVersion
	if err := cache.Get(cacheKey, &cached); err == nil {
		return &cached, nil
	}

	// Get from database
	var translation models.TranslationVersion
	result := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND is_active = ? AND version = ?",
		componentID, locale, stage, true, 2).First(&translation)

	if result.Error == gorm.ErrRecordNotFound {
		// Try version 1 if version 2 doesn't exist
		result = database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND is_active = ? AND version = ?",
			componentID, locale, stage, true, 1).First(&translation)
	}

	if result.Error != nil {
		return nil, result.Error
	}

	// Cache for 1 hour
	cache.Set(cacheKey, translation, 3600*1000000000) // 1 hour in nanoseconds

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

	// Second pass: Get missing translations from database
	if len(missingFromCache) > 0 {
		var translations []models.TranslationVersion
		componentIDStrings := make([]string, len(missingFromCache))
		for i, id := range missingFromCache {
			componentIDStrings[i] = id.String()
		}

		// Query database for missing translations
		err := database.DB.Where("component_id IN ? AND locale = ? AND stage = ? AND is_active = ? AND version = ?",
			missingFromCache, locale, stage, true, 2).Find(&translations).Error

		if err == nil {
			// Process found translations
			for i := range translations {
				translation := translations[i]
				results[translation.ComponentID.String()] = &translation

				// Cache for future use
				cacheKey := cache.TranslationKey(translation.ComponentID.String(), locale, string(stage))
				cache.Set(cacheKey, translation, 3600*1000000000) // 1 hour
			}

			// For components not found with version 2, try version 1
			foundIDs := make(map[string]bool)
			for _, t := range translations {
				foundIDs[t.ComponentID.String()] = true
			}

			stillMissing := make([]uuid.UUID, 0)
			for _, id := range missingFromCache {
				if !foundIDs[id.String()] {
					stillMissing = append(stillMissing, id)
				}
			}

			if len(stillMissing) > 0 {
				var version1Translations []models.TranslationVersion
				database.DB.Where("component_id IN ? AND locale = ? AND stage = ? AND is_active = ? AND version = ?",
					stillMissing, locale, stage, true, 1).Find(&version1Translations)

				for i := range version1Translations {
					translation := version1Translations[i]
					results[translation.ComponentID.String()] = &translation

					// Cache for future use
					cacheKey := cache.TranslationKey(translation.ComponentID.String(), locale, string(stage))
					cache.Set(cacheKey, translation, 3600*1000000000) // 1 hour
				}
			}
		}
	}

	return results, nil
}

// SaveTranslation saves a translation version
func (s *TranslationService) SaveTranslation(componentID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, userID uuid.UUID) (*models.TranslationVersion, error) {
	// Get existing version 2 (after save)
	var existing models.TranslationVersion
	result := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, 2).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new version 2
		existing = models.TranslationVersion{
			ComponentID: componentID,
			Locale:      locale,
			Stage:       stage,
			Version:     2,
			Data:        data,
			IsActive:    true,
			CreatedBy:   userID,
			UpdatedBy:   userID,
		}
		if err := database.DB.Create(&existing).Error; err != nil {
			return nil, err
		}
	} else {
		// Update existing version 2
		existing.Data = data
		existing.UpdatedBy = userID
		if err := database.DB.Save(&existing).Error; err != nil {
			return nil, err
		}
	}

	// Ensure version 1 exists (before save)
	var beforeSave models.TranslationVersion
	result = database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, 1).First(&beforeSave)

	if result.Error == gorm.ErrRecordNotFound {
		// Create version 1 from current data
		beforeSave = models.TranslationVersion{
			ComponentID: componentID,
			Locale:      locale,
			Stage:       stage,
			Version:     1,
			Data:        data,
			IsActive:    true,
		}
		database.DB.Create(&beforeSave)
	}

	// Invalidate cache
	cache.Delete(cache.TranslationKey(componentID.String(), locale, string(stage)))
	cache.Delete(cache.ComponentKey(componentID.String()))

	// Cleanup old versions
	database.CleanupOldVersions()

	return &existing, nil
}

// RevertTranslation reverts to version 1 (before save)
func (s *TranslationService) RevertTranslation(componentID uuid.UUID, locale string, stage models.DeploymentStage) error {
	// Get version 1
	var version1 models.TranslationVersion
	if err := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, 1).First(&version1).Error; err != nil {
		return fmt.Errorf("no previous version found: %w", err)
	}

	// Update version 2 with version 1 data
	var version2 models.TranslationVersion
	result := database.DB.Where("component_id = ? AND locale = ? AND stage = ? AND version = ?",
		componentID, locale, stage, 2).First(&version2)

	if result.Error == gorm.ErrRecordNotFound {
		// Create version 2 from version 1
		version2 = models.TranslationVersion{
			ComponentID: componentID,
			Locale:      locale,
			Stage:       stage,
			Version:     2,
			Data:        version1.Data,
			IsActive:    true,
		}
		return database.DB.Create(&version2).Error
	}

	version2.Data = version1.Data
	if err := database.DB.Save(&version2).Error; err != nil {
		return err
	}

	// Invalidate cache
	cache.Delete(cache.TranslationKey(componentID.String(), locale, string(stage)))

	return nil
}

// DeployToStage deploys translations from draft to staging or staging to production
func (s *TranslationService) DeployToStage(componentID uuid.UUID, locale string, fromStage, toStage models.DeploymentStage, userID uuid.UUID) error {
	// Get source translation
	source, err := s.GetTranslation(componentID, locale, fromStage)
	if err != nil {
		return fmt.Errorf("source translation not found: %w", err)
	}

	// Save to target stage
	_, err = s.SaveTranslation(componentID, locale, toStage, source.Data, userID)
	return err
}

// ExtractTemplateValues extracts template values from text (values in brackets)
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

// PreserveTemplateValues preserves template values during translation
func PreserveTemplateValues(text string, translatedText string) string {
	// Extract template values from original
	templateValues := ExtractTemplateValues(text)
	if len(templateValues) == 0 {
		return translatedText
	}

	// Replace template placeholders in translated text
	result := translatedText
	for i, value := range templateValues {
		// Try to find and preserve the template value
		placeholder := fmt.Sprintf("[%s]", value)
		if !strings.Contains(result, placeholder) {
			// If the template value is missing, try to restore it
			// This is a simple approach - in production, you might want more sophisticated matching
			re := regexp.MustCompile(`\[([^\]]+)\]`)
			matches := re.FindAllStringSubmatch(result, -1)
			if i < len(matches) {
				// Replace the i-th match with the original template value
				result = strings.Replace(result, matches[i][0], placeholder, 1)
			}
		}
	}

	return result
}

