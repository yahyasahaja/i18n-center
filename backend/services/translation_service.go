package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
)

type TranslationService struct{}

func NewTranslationService() *TranslationService {
	return &TranslationService{}
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

// SaveTranslation adds a new version (always insert, never overwrite)
func (s *TranslationService) SaveTranslation(componentID uuid.UUID, locale string, stage models.DeploymentStage, data models.JSONB, userID uuid.UUID) (*models.TranslationVersion, error) {
	nextVersion := 1
	var current models.TranslationVersion
	err := database.DB.Where("component_id = ? AND locale = ? AND stage = ?",
		componentID, locale, stage).Order("version DESC").First(&current).Error
	if err == nil {
		nextVersion = current.Version + 1
	}

	newVersion := models.TranslationVersion{
		ComponentID: componentID,
		Locale:      locale,
		Stage:       stage,
		Version:     nextVersion,
		Data:        data,
		IsActive:    true,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}
	if err := database.DB.Create(&newVersion).Error; err != nil {
		return nil, err
	}

	cache.Delete(cache.TranslationKey(componentID.String(), locale, string(stage)))
	cache.Delete(cache.ComponentKey(componentID.String()))
	database.CleanupOldVersions()
	return &newVersion, nil
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
	cache.Delete(cache.TranslationKey(componentID.String(), locale, string(stage)))
	database.CleanupOldVersions()
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
	cache.Delete(cache.TranslationKey(v.ComponentID.String(), v.Locale, string(v.Stage)))
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
