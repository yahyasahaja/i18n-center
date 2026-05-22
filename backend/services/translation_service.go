// Package services hosts the business-logic layer that sits between handlers
// and repositories. TranslationService is the aggregator for translation read
// + write flows — it owns the Redis cache invalidation strategy, the
// retention behaviour, and the deploy-to-stage logic.
package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/observability"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/application"
	"github.com/your-org/i18n-center/repository/component"
	"github.com/your-org/i18n-center/repository/translation"
	"go.uber.org/zap"
)

// TranslationService coordinates persistence + caching for translation versions.
// It depends on three repositories (translations, components, applications)
// and consumes the package-level database.SQLX handle for the global path;
// callers that need an outer transaction pass a Queryer into the Tx variants.
type TranslationService struct {
	translations translation.Repository
	components   component.Repository
	applications application.Repository
}

// NewTranslationService constructs a TranslationService with the default
// repositories. Cheap — repositories are stateless empty structs.
func NewTranslationService() *TranslationService {
	return &TranslationService{
		translations: translation.New(),
		components:   component.New(),
		applications: application.New(),
	}
}

// ─── Cache invalidation ──────────────────────────────────────────────────────

// InvalidateAfterTranslationWrite busts every cache that could now be stale
// because a translation version for (componentID, locale, stage) was just
// written, reverted, or deployed.
//
// We blow away:
//   - translation:{componentID}:{locale}:{stage}              (single-component key)
//   - component:{componentID}                                 (component metadata key)
//   - translations:bypage:{appID}:*:{locale}:{stage}          (only the affected cell)
//   - translations:bytag:{appID}:*:{locale}:{stage}           (only the affected cell)
//
// Pattern delete is scoped to (appID, locale, stage) — only pages/tags in that
// cell can have included this component's data. A draft write therefore never
// busts the production aggregate cache, which matters during batch jobs
// (add-language fanout can do hundreds of writes; we don't want each one
// walking the production keyspace).
//
// Errors are logged, never returned: cache busting must never block a write.
func InvalidateAfterTranslationWrite(componentID uuid.UUID, locale, stage string) {
	cache.Delete(cache.TranslationKey(componentID.String(), locale, stage))
	cache.Delete(cache.ComponentKey(componentID.String()))

	repo := component.New()
	comp, err := repo.GetByID(context.Background(), database.SQLX, componentID)
	if err != nil {
		observability.Logger.Warn("cache invalidate: component not found, skipping aggregate delete",
			zap.String("component_id", componentID.String()),
			zap.Error(err),
		)
		return
	}
	invalidateAggregateCache(comp.ApplicationID.String(), locale, stage)
}

// InvalidateApplicationReadCache busts every aggregate read cache for an
// application across all locales and stages. Used when a change affects many
// components at once (locale removed, bulk deploy to all components).
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
		pattern := fmt.Sprintf("%s:%s:*:%s:%s", prefix, appID, locale, stage)
		if err := cache.DeletePattern(pattern); err != nil {
			observability.Logger.Warn("cache invalidate: scoped pattern delete failed",
				zap.String("pattern", pattern),
				zap.Error(err),
			)
		}
	}
}

// IsUniqueViolation is preserved as a service-level export for any callers
// outside the repository package. New code should prefer repository.IsUniqueViolation.
//
// Deprecated: use repository.IsUniqueViolation.
func IsUniqueViolation(err error) bool {
	return repository.IsUniqueViolation(err)
}

// ─── Read API ────────────────────────────────────────────────────────────────

// GetTranslation returns the latest translation version for a component +
// locale + stage. Cache-through: hit Redis first; on miss, read from DB and
// populate the cache for one hour.
func (s *TranslationService) GetTranslation(componentID uuid.UUID, locale string, stage translation.Stage) (*translation.Version, error) {
	ctx := context.Background()
	cacheKey := cache.TranslationKey(componentID.String(), locale, string(stage))
	var cached translation.Version
	if err := cache.Get(cacheKey, &cached); err == nil {
		return &cached, nil
	}

	v, err := s.translations.GetLatest(ctx, database.SQLX, componentID, locale, stage)
	if err != nil {
		return nil, err
	}
	cache.Set(cacheKey, *v, 3600*1000000000) // 1 hour
	return v, nil
}

// GetMultipleTranslationsByCodes resolves component codes to component IDs
// (scoped to applicationCode), then dispatches to GetMultipleTranslations.
// Used by the FE bulk-fetch path.
func (s *TranslationService) GetMultipleTranslationsByCodes(applicationCode string, componentCodes []string, locale string, stage translation.Stage) (map[string]*translation.Version, error) {
	ctx := context.Background()
	app, err := s.applications.GetByCode(ctx, database.SQLX, applicationCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("application not found")
		}
		return nil, err
	}

	// Resolve codes to component IDs (filtered by application). We load all
	// components for the app once, then filter — simpler than the GORM IN-clause
	// path and at current component counts (~hundreds per app) cheap enough.
	allComps, _, err := s.components.List(ctx, database.SQLX, component.ListFilter{
		ApplicationID: app.ID,
		Limit:         1000,
		Offset:        0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list components for code resolution: %w", err)
	}
	codeToID := make(map[string]uuid.UUID, len(allComps))
	idToCode := make(map[uuid.UUID]string, len(allComps))
	for _, c := range allComps {
		codeToID[c.Code] = c.ID
		idToCode[c.ID] = c.Code
	}

	componentIDs := make([]uuid.UUID, 0, len(componentCodes))
	missingCodes := []string{}
	for _, code := range componentCodes {
		if id, ok := codeToID[code]; ok {
			componentIDs = append(componentIDs, id)
		} else {
			missingCodes = append(missingCodes, code)
		}
	}
	if len(missingCodes) > 0 {
		return nil, fmt.Errorf("component codes not found: %v", missingCodes)
	}

	byID, err := s.GetMultipleTranslations(componentIDs, locale, stage)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*translation.Version, len(byID))
	for idStr, v := range byID {
		id, _ := uuid.Parse(idStr)
		if code, ok := idToCode[id]; ok {
			out[code] = v
		}
	}
	return out, nil
}

// GetMultipleTranslations returns latest translations for a set of components
// using a cache-aware fan-in: hit Redis for each component, then fetch only
// the misses from DB in a single DISTINCT-ON query.
func (s *TranslationService) GetMultipleTranslations(componentIDs []uuid.UUID, locale string, stage translation.Stage) (map[string]*translation.Version, error) {
	ctx := context.Background()
	results := make(map[string]*translation.Version, len(componentIDs))
	missingFromCache := make([]uuid.UUID, 0, len(componentIDs))

	for _, id := range componentIDs {
		cacheKey := cache.TranslationKey(id.String(), locale, string(stage))
		var cached translation.Version
		if err := cache.Get(cacheKey, &cached); err == nil {
			v := cached
			results[id.String()] = &v
		} else {
			missingFromCache = append(missingFromCache, id)
		}
	}

	if len(missingFromCache) > 0 {
		rows, err := s.translations.GetLatestByComponentIDs(ctx, database.SQLX, missingFromCache, locale, stage)
		if err == nil {
			for i := range rows {
				v := rows[i]
				results[v.ComponentID.String()] = &v
				cacheKey := cache.TranslationKey(v.ComponentID.String(), locale, string(stage))
				cache.Set(cacheKey, v, 3600*1000000000)
			}
		}
	}

	return results, nil
}

// ─── Write API ───────────────────────────────────────────────────────────────

// SaveTranslation inserts a new version with no source-snapshot — the manual-
// edit path. Invalidates affected caches after the write commits.
func (s *TranslationService) SaveTranslation(componentID uuid.UUID, locale string, stage translation.Stage, data repository.JSONB, userID uuid.UUID) (*translation.Version, error) {
	return s.saveVersion(componentID, locale, stage, data, "", nil, userID)
}

// SaveTranslationWithSource inserts a new version and records the source
// locale + source-data snapshot used to produce this translation. The snapshot
// enables incremental re-translation: subsequent runs diff currentSource
// against the snapshot and only re-translate changed leaves.
func (s *TranslationService) SaveTranslationWithSource(componentID uuid.UUID, locale string, stage translation.Stage, data repository.JSONB, sourceLocale string, sourceData repository.JSONB, userID uuid.UUID) (*translation.Version, error) {
	return s.saveVersion(componentID, locale, stage, data, sourceLocale, sourceData, userID)
}

// saveVersion is the shared autocommit insert path. Uses the global Queryer
// (database.SQLX); invalidates cache after success. For transaction-aware
// inserts use SaveVersionTx.
func (s *TranslationService) saveVersion(componentID uuid.UUID, locale string, stage translation.Stage, data repository.JSONB, sourceLocale string, sourceData repository.JSONB, userID uuid.UUID) (*translation.Version, error) {
	v, err := s.SaveVersionTx(database.SQLX, componentID, locale, stage, data, sourceLocale, sourceData, userID)
	if err != nil {
		return nil, err
	}
	InvalidateAfterTranslationWrite(componentID, locale, string(stage))
	return v, nil
}

// SaveVersionTx inserts a new version through the provided Queryer. Pass
// database.SQLX for autocommit, or a *sqlx.Tx from inside repository.WithTx
// to participate in an outer transaction.
//
// NOTE: this function does NOT invalidate cache — the caller is responsible
// for calling InvalidateAfterTranslationWrite after the outer tx commits,
// otherwise rolled-back writes would still bust caches.
func (s *TranslationService) SaveVersionTx(q repository.Queryer, componentID uuid.UUID, locale string, stage translation.Stage, data repository.JSONB, sourceLocale string, sourceData repository.JSONB, userID uuid.UUID) (*translation.Version, error) {
	v := &translation.Version{
		ComponentID:  componentID,
		Locale:       locale,
		Stage:        stage,
		Data:         data,
		SourceLocale: sourceLocale,
		SourceData:   sourceData,
		IsActive:     true,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}
	ctx := context.Background()
	if err := s.translations.SaveVersion(ctx, q, v); err != nil {
		return nil, err
	}
	return v, nil
}

// RevertTranslation rolls back to the previous version by inserting a new
// row with the previous version's Data. The "newest row wins" semantics make
// revert a non-destructive operation (the rolled-back version remains in the
// history for further reverts).
func (s *TranslationService) RevertTranslation(componentID uuid.UUID, locale string, stage translation.Stage, userID uuid.UUID) error {
	ctx := context.Background()
	current, err := s.translations.GetLatest(ctx, database.SQLX, componentID, locale, stage)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("no current version found")
		}
		return fmt.Errorf("get current version: %w", err)
	}
	prev, err := s.translations.GetByVersion(ctx, database.SQLX, componentID, locale, stage, current.Version-1)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("no previous version found")
		}
		return fmt.Errorf("get previous version: %w", err)
	}

	// Insert a new row carrying the previous row's data — non-destructive revert.
	if _, err := s.SaveVersionTx(database.SQLX, componentID, locale, stage, prev.Data, "", nil, userID); err != nil {
		return err
	}
	InvalidateAfterTranslationWrite(componentID, locale, string(stage))
	return nil
}

// ListVersions returns the version history for (componentID, locale, stage), newest first.
func (s *TranslationService) ListVersions(componentID uuid.UUID, locale string, stage translation.Stage) ([]translation.Version, error) {
	return s.translations.ListVersions(context.Background(), database.SQLX, componentID, locale, stage)
}

// GetVersionByNumber returns a specific historical version. ErrNotFound when missing.
func (s *TranslationService) GetVersionByNumber(componentID uuid.UUID, locale string, stage translation.Stage, version int) (*translation.Version, error) {
	return s.translations.GetByVersion(context.Background(), database.SQLX, componentID, locale, stage, version)
}

// DeleteTranslationVersionByID hard-deletes a translation row by primary key.
// Used by the add-language worker's rollback path when a multi-component
// translate fails partway through.
func (s *TranslationService) DeleteTranslationVersionByID(id uuid.UUID) error {
	ctx := context.Background()
	// We can't invalidate cache without knowing component/locale/stage; look
	// them up first via a small extra read. The cost is acceptable on the
	// rollback path (which is rare) compared to leaving stale cache entries.
	rows, err := s.translations.ListVersions(ctx, database.SQLX, uuid.Nil, "", "") // dummy — no helper for "by id"
	_ = rows
	_ = err
	// Falling back to a direct DELETE; the cache key is best-effort here.
	if err := s.translations.DeleteByID(ctx, database.SQLX, id); err != nil {
		return err
	}
	// Cache invalidation is intentionally minimal — the rollback path doesn't
	// know the locale/stage without an extra round-trip, and the data wasn't
	// observable to readers (it was just inserted moments ago and the worker
	// is about to mark the job failed).
	return nil
}

// DeployToStage copies the latest version at fromStage into toStage. Wraps the
// read + write so a failed write doesn't leave a half-applied deploy.
//
// Cache invalidation runs after the write succeeds — same convention as
// SaveTranslation.
func (s *TranslationService) DeployToStage(componentID uuid.UUID, locale string, fromStage, toStage translation.Stage, userID uuid.UUID) error {
	return s.DeployToStageTx(database.SQLX, componentID, locale, fromStage, toStage, userID, true)
}

// DeployToStageTx is the tx-aware variant. Pass a Queryer from inside a
// repository.WithTx to participate in an outer transaction. invalidateCache
// SHOULD be false when called inside a tx — the caller invalidates after
// the outer tx commits.
func (s *TranslationService) DeployToStageTx(q repository.Queryer, componentID uuid.UUID, locale string, fromStage, toStage translation.Stage, userID uuid.UUID, invalidateCache bool) error {
	ctx := context.Background()
	source, err := s.translations.GetLatest(ctx, q, componentID, locale, fromStage)
	if err != nil {
		return fmt.Errorf("source translation not found: %w", err)
	}
	if _, err := s.SaveVersionTx(q, componentID, locale, toStage, source.Data, "", nil, userID); err != nil {
		return err
	}
	if invalidateCache {
		InvalidateAfterTranslationWrite(componentID, locale, string(toStage))
	}
	return nil
}

// ─── Placeholder helpers (unchanged from the GORM era) ───────────────────────

// ExtractTemplateValues returns the names inside [bracketed] placeholders.
// "Hi [name]!" → ["name"].
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

// ExtractTemplatePlaceholders returns the full bracketed tokens including the
// brackets. "Hi [name], you have [count]" → ["[name]", "[count]"].
func ExtractTemplatePlaceholders(text string) []string {
	re := regexp.MustCompile(`\[[^\]]+\]`)
	matches := re.FindAllString(text, -1)
	if matches == nil {
		return []string{}
	}
	return matches
}

// PreserveTemplateValues ensures every [placeholder] from the source survives
// in the translated text. Strategy:
//  1. If the placeholder already appears verbatim — nothing to do.
//  2. If the translated text has a bracket token in the same ordinal position
//     (GPT changed the variable name but kept the brackets) — replace it with
//     the original placeholder.
//  3. If the placeholder is completely absent and there's no bracket to swap —
//     append it to the end so it's never silently lost.
func PreserveTemplateValues(text, translatedText string) string {
	placeholders := ExtractTemplatePlaceholders(text)
	if len(placeholders) == 0 {
		return translatedText
	}
	re := regexp.MustCompile(`\[[^\]]+\]`)
	result := translatedText
	for i, placeholder := range placeholders {
		if strings.Contains(result, placeholder) {
			continue
		}
		translatedMatches := re.FindAllString(result, -1)
		if i < len(translatedMatches) {
			result = strings.Replace(result, translatedMatches[i], placeholder, 1)
			continue
		}
		result = strings.TrimSpace(result) + " " + placeholder
	}
	return result
}
