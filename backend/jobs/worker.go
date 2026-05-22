// Package jobs hosts the in-process async worker. As of Commit H it's fully
// off GORM and uses the new sqlx-backed repositories. Three job tables drive
// it (AddLanguage / Translate / CmsTranslate); each polls with the same
// claim → process → mark-completed/failed shape.
//
// The worker is K8s-safe: every claim goes through
// repository/job.*Repository.ClaimNext, which is a
// `UPDATE ... FOR UPDATE SKIP LOCKED ... RETURNING *` so multiple replicas
// can run side-by-side without claiming the same row twice. Stuck-running
// rows older than 15 minutes get reset to pending via ResetStuck before each
// claim attempt — crashed pods don't orphan in-flight work for long.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/observability"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/application"
	"github.com/your-org/i18n-center/repository/cms"
	"github.com/your-org/i18n-center/repository/component"
	"github.com/your-org/i18n-center/repository/job"
	"github.com/your-org/i18n-center/repository/localedeploy"
	"github.com/your-org/i18n-center/repository/translation"
	"github.com/your-org/i18n-center/services"
)

const (
	pollInterval         = 5 * time.Second
	componentConcurrency = 5                // max parallel OpenAI calls inside a single AddLanguageJob
	stuckJobAfter        = 15 * time.Minute // rows running longer than this get reset to pending
)

// Package-level repository handles — instantiated once. The repos are
// stateless (no per-instance config) so sharing is cheap and avoids
// reallocating closures on every poll tick.
var (
	addLangRepo      = job.NewAddLanguageRepository()
	translateRepo    = job.NewTranslateRepository()
	cmsTranslateRepo = job.NewCmsTranslateRepository()
	deployRepo       = localedeploy.New()
	appRepo          = application.New()
	componentRepo    = component.New()
	templateRepo     = cms.NewTemplateRepository()
	itemRepo         = cms.NewItemRepository(templateRepo)
	cmsLocRepo       = cms.NewLocalizationRepository()
)

// Run starts the in-process worker loop. Claims jobs from all three job tables
// (AddLanguage / Translate / CmsTranslate) on each tick. Safe for multiple K8s
// replicas via FOR UPDATE SKIP LOCKED in each ClaimNext.
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

		processed := false

		if addJob, err := claimAddLanguageJob(ctx, instanceID); err != nil {
			observability.Logger.Warn("Worker claim error (AddLanguageJob)", zap.Error(err))
		} else if addJob != nil {
			processAddLanguageJob(ctx, addJob, translationService)
			processed = true
		}

		if !processed {
			if trJob, err := claimTranslateJob(ctx, instanceID); err != nil {
				observability.Logger.Warn("Worker claim error (TranslateJob)", zap.Error(err))
			} else if trJob != nil {
				processTranslateJob(ctx, trJob, translationService)
				processed = true
			}
		}

		if !processed {
			if cmsJob, err := claimCmsTranslateJob(ctx, instanceID); err != nil {
				observability.Logger.Warn("Worker claim error (CmsTranslateJob)", zap.Error(err))
			} else if cmsJob != nil {
				processCmsTranslateJob(ctx, cmsJob)
				processed = true
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// ─── AddLanguageJob ──────────────────────────────────────────────────────────

func claimAddLanguageJob(ctx context.Context, instanceID string) (*job.AddLanguageJob, error) {
	if err := addLangRepo.ResetStuck(ctx, database.SQLX, stuckJobAfter); err != nil {
		// Don't fail the whole tick — the claim below can still try without reset.
		observability.Logger.Warn("ResetStuck failed (AddLanguageJob)", zap.Error(err))
	}
	return addLangRepo.ClaimNext(ctx, database.SQLX, instanceID)
}

// processAddLanguageJob runs the add-language auto-translate logic with a
// goroutine pool (componentConcurrency parallel OpenAI calls). All state in
// DB; safe for K8s.
func processAddLanguageJob(ctx context.Context, j *job.AddLanguageJob, translationService *services.TranslationService) {
	appIDStr := j.ApplicationID.String()
	defer func() {
		if r := recover(); r != nil {
			observability.Logger.Error("Worker panic (AddLanguageJob)", zap.Any("panic", r), zap.String("job_id", j.ID.String()))
			_ = addLangRepo.MarkFailed(context.Background(), database.SQLX, j.ID, "Worker panic", fmt.Sprintf("%v", r))
		}
	}()

	app, err := appRepo.GetByID(ctx, database.SQLX, j.ApplicationID)
	if err != nil {
		_ = addLangRepo.MarkFailed(ctx, database.SQLX, j.ID, "Application not found", err.Error())
		return
	}

	openAIService := resolveOpenAIService(app.OpenAIKey)
	if openAIService == nil {
		_ = addLangRepo.MarkFailed(ctx, database.SQLX, j.ID, "OpenAI API key not configured", "Configure in Application settings")
		return
	}

	components, _, err := componentRepo.List(ctx, database.SQLX, component.ListFilter{
		ApplicationID: j.ApplicationID,
		// No limit — fan-out over every component in the app.
	})
	if err != nil {
		_ = addLangRepo.MarkFailed(ctx, database.SQLX, j.ID, "Failed to load components", err.Error())
		return
	}

	// Record the total up-front so the frontend can show "X / N" immediately.
	if err := addLangRepo.UpdateTotals(ctx, database.SQLX, j.ID, len(components), 0); err != nil {
		observability.Logger.Warn("UpdateTotals failed", zap.Error(err))
	}

	// Goroutine pool — semaphore pattern.
	type result struct {
		versionID uuid.UUID
		err       error
		compCode  string
	}

	sem := make(chan struct{}, componentConcurrency)
	results := make(chan result, len(components))
	var wg sync.WaitGroup
	var completedCount atomic.Int64

	for _, c := range components {
		wg.Add(1)
		c := c
		sem <- struct{}{} // acquire slot (blocks when all workers are busy)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			// Bail early if context was cancelled while we were waiting for a slot.
			if err := ctx.Err(); err != nil {
				results <- result{err: fmt.Errorf("component %s: cancelled", c.Code), compCode: c.Code}
				return
			}

			sourceTranslation, err := translationService.GetTranslation(c.ID, c.DefaultLocale, translation.StageDraft)
			if err != nil {
				results <- result{err: fmt.Errorf("component %s: no draft translation for default locale %s", c.Code, c.DefaultLocale), compCode: c.Code}
				_ = addLangRepo.IncrementCompleted(ctx, database.SQLX, j.ID)
				completedCount.Add(1)
				return
			}

			translatedData, err := openAIService.TranslateJSON(ctx, sourceTranslation.Data, jsonbToStringMap(c.KeyContexts), c.DefaultLocale, j.Locale)
			if err != nil {
				results <- result{err: fmt.Errorf("component %s: %w", c.Code, err), compCode: c.Code}
				_ = addLangRepo.IncrementCompleted(ctx, database.SQLX, j.ID)
				completedCount.Add(1)
				return
			}

			tr, err := translationService.SaveTranslation(c.ID, j.Locale, translation.StageDraft, translatedData, j.CreatedBy)
			if err != nil {
				results <- result{err: fmt.Errorf("component %s: %w", c.Code, err), compCode: c.Code}
				_ = addLangRepo.IncrementCompleted(ctx, database.SQLX, j.ID)
				completedCount.Add(1)
				return
			}

			_ = addLangRepo.IncrementCompleted(ctx, database.SQLX, j.ID)
			completedCount.Add(1)
			results <- result{versionID: tr.ID}
		}()
	}

	wg.Wait()
	close(results)

	// Honour cancellation after draining goroutines.
	select {
	case <-ctx.Done():
		_ = addLangRepo.MarkFailed(context.Background(), database.SQLX, j.ID, "Worker cancelled", "Context cancelled")
		for r := range results {
			if r.versionID != uuid.Nil {
				_ = translationService.DeleteTranslationVersionByID(r.versionID)
			}
		}
		return
	default:
	}

	// Collect all results before deciding — ensures full rollback even when
	// failures are interleaved with successes.
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
		_ = addLangRepo.MarkFailed(ctx, database.SQLX, j.ID, "Translation process failed (rolled back)", firstErr.Error())
		return
	}

	// Create/reset deploy tracking. Two cases to handle:
	//   1. Locale was deleted while job was running → locale no longer in
	//      enabled_languages; skip creating the deploy record so we don't
	//      leave a ghost entry in pending deploys.
	//   2. Locale was previously deployed to production and is being
	//      re-translated; upsert back to 'draft' instead of creating.
	localeStillEnabled := false
	if currentApp, err := appRepo.GetByID(ctx, database.SQLX, j.ApplicationID); err == nil {
		for _, l := range currentApp.EnabledLanguages {
			if strings.EqualFold(l, j.Locale) {
				localeStillEnabled = true
				break
			}
		}
	}
	if localeStillEnabled {
		deploy := &localedeploy.Deploy{
			ApplicationID:  j.ApplicationID,
			Locale:         j.Locale,
			StageCompleted: localedeploy.StageDraft,
		}
		if err := deployRepo.Upsert(ctx, database.SQLX, deploy); err != nil {
			observability.Logger.Warn("Failed to upsert ApplicationLocaleDeploy", zap.Error(err))
		}
	} else {
		observability.Logger.Info("Skipping deploy record: locale was removed while job ran",
			zap.String("job_id", j.ID.String()),
			zap.String("locale", j.Locale),
		)
	}

	cache.Delete(cache.ApplicationKey(appIDStr))
	if err := addLangRepo.MarkCompleted(ctx, database.SQLX, j.ID); err != nil {
		observability.Logger.Warn("MarkCompleted failed (AddLanguageJob)", zap.Error(err))
	}
	observability.Logger.Info("AddLanguageJob completed",
		zap.String("job_id", j.ID.String()),
		zap.String("locale", j.Locale),
		zap.Int("components", len(components)),
		zap.Int64("processed", completedCount.Load()),
	)
}

// GetJobStatus returns AddLanguageJob by ID and application ID (for auth scope).
// Used by the API handler. Returns (nil, nil) for not-found so the handler can
// 404 without translating sentinel errors.
func GetJobStatus(applicationID, jobID uuid.UUID) (*job.AddLanguageJob, error) {
	ctx := context.Background()
	j, err := addLangRepo.GetByIDForApp(ctx, database.SQLX, jobID, applicationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return j, nil
}

// ─── TranslateJob ─────────────────────────────────────────────────────────────

func claimTranslateJob(ctx context.Context, instanceID string) (*job.TranslateJob, error) {
	if err := translateRepo.ResetStuck(ctx, database.SQLX, stuckJobAfter); err != nil {
		observability.Logger.Warn("ResetStuck failed (TranslateJob)", zap.Error(err))
	}
	return translateRepo.ClaimNext(ctx, database.SQLX, instanceID)
}

// processTranslateJob handles both auto_translate and backfill job types.
// Each TranslateJob carries exactly one target locale (backfill is fanned
// out by the handler into N single-target jobs).
//
// Incremental translation strategy:
//   - If the existing target translation has a source snapshot (SourceData),
//     we diff currentSource vs snapshot to find only the changed/new keys,
//     send those to AI, then merge back into the existing target and prune
//     removed keys. No AI call at all if nothing changed.
//   - If there is no snapshot (first translate, or target was manually
//     edited), the full source is translated as before.
func processTranslateJob(ctx context.Context, j *job.TranslateJob, translationService *services.TranslationService) {
	defer func() {
		if r := recover(); r != nil {
			observability.Logger.Error("Worker panic (TranslateJob)", zap.Any("panic", r), zap.String("job_id", j.ID.String()))
			_ = translateRepo.MarkFailed(context.Background(), database.SQLX, j.ID, "Worker panic", fmt.Sprintf("%v", r))
		}
	}()

	app, err := appRepo.GetByID(ctx, database.SQLX, j.ApplicationID)
	if err != nil {
		_ = translateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Application not found", err.Error())
		return
	}

	openAIService := resolveOpenAIService(app.OpenAIKey)
	if openAIService == nil {
		_ = translateRepo.MarkFailed(ctx, database.SQLX, j.ID, "OpenAI API key not configured", "Configure in Application settings")
		return
	}

	if len(j.TargetLocales) == 0 {
		_ = translateRepo.MarkFailed(ctx, database.SQLX, j.ID, "No target locales specified", "")
		return
	}
	targetLocale := j.TargetLocales[0]

	select {
	case <-ctx.Done():
		_ = translateRepo.MarkFailed(context.Background(), database.SQLX, j.ID, "Worker cancelled", "Context cancelled")
		return
	default:
	}

	sourceTranslation, err := translationService.GetTranslation(j.ComponentID, j.SourceLocale, translation.StageDraft)
	if err != nil {
		_ = translateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Source translation not found",
			fmt.Sprintf("component %s locale %s: %v", j.ComponentID, j.SourceLocale, err))
		return
	}

	// Load component KeyContexts to inject as AI hints. Missing record is
	// fine — we just translate without hints.
	var keyContexts map[string]string
	if comp, err := componentRepo.GetByID(ctx, database.SQLX, j.ComponentID); err == nil {
		keyContexts = jsonbToStringMap(comp.KeyContexts)
	}

	currentSource := map[string]interface{}(sourceTranslation.Data)
	var finalData map[string]interface{}

	// Try to load the existing target translation and its stored source snapshot.
	existingTarget, _ := translationService.GetTranslation(j.ComponentID, targetLocale, translation.StageDraft)

	if existingTarget != nil && len(existingTarget.SourceData) > 0 {
		// ── Incremental path ────────────────────────────────────────────────
		prevSource := map[string]interface{}(existingTarget.SourceData)
		changed := changedOrNewKeys(currentSource, prevSource)
		hasRemovals := hasRemovedKeys(prevSource, currentSource)

		if len(changed) == 0 && !hasRemovals {
			observability.Logger.Info("TranslateJob skipped (source unchanged)",
				zap.String("job_id", j.ID.String()),
				zap.String("target_locale", targetLocale),
			)
			_ = translateRepo.MarkCompleted(ctx, database.SQLX, j.ID)
			return
		}

		existingTargetData := map[string]interface{}(existingTarget.Data)
		if len(changed) > 0 {
			translatedPartial, err := openAIService.TranslateJSON(ctx, changed, keyContexts, j.SourceLocale, targetLocale)
			if err != nil {
				_ = translateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Translation failed", err.Error())
				return
			}
			finalData = mergeTranslations(existingTargetData, translatedPartial)
		} else {
			finalData = existingTargetData
		}

		// Drop keys removed from source so all locales stay structurally in sync.
		finalData = pruneToShape(finalData, currentSource)

		observability.Logger.Info("TranslateJob incremental",
			zap.String("job_id", j.ID.String()),
			zap.String("target_locale", targetLocale),
			zap.Int("changed_keys", countLeaves(changed)),
			zap.Bool("had_removals", hasRemovals),
		)
	} else {
		// ── Full-translate path (first run or no snapshot) ─────────────────
		finalData, err = openAIService.TranslateJSON(ctx, currentSource, keyContexts, j.SourceLocale, targetLocale)
		if err != nil {
			_ = translateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Translation failed", err.Error())
			return
		}
		observability.Logger.Info("TranslateJob full",
			zap.String("job_id", j.ID.String()),
			zap.String("target_locale", targetLocale),
		)
	}

	// Save with source snapshot so future runs can diff against it.
	if _, err := translationService.SaveTranslationWithSource(
		j.ComponentID, targetLocale, translation.StageDraft,
		repository.JSONB(finalData),
		j.SourceLocale, repository.JSONB(currentSource),
		j.CreatedBy,
	); err != nil {
		_ = translateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Failed to save translation", err.Error())
		return
	}

	cache.Delete(cache.ApplicationKey(j.ApplicationID.String()))
	if err := translateRepo.MarkCompleted(ctx, database.SQLX, j.ID); err != nil {
		observability.Logger.Warn("MarkCompleted failed (TranslateJob)", zap.Error(err))
	}
	observability.Logger.Info("TranslateJob completed",
		zap.String("job_id", j.ID.String()),
		zap.String("component_id", j.ComponentID.String()),
		zap.String("target_locale", targetLocale),
		zap.String("job_type", j.JobType),
	)
}

// ─── CmsTranslateJob ─────────────────────────────────────────────────────────

func claimCmsTranslateJob(ctx context.Context, instanceID string) (*job.CmsTranslateJob, error) {
	if err := cmsTranslateRepo.ResetStuck(ctx, database.SQLX, stuckJobAfter); err != nil {
		observability.Logger.Warn("ResetStuck failed (CmsTranslateJob)", zap.Error(err))
	}
	return cmsTranslateRepo.ClaimNext(ctx, database.SQLX, instanceID)
}

func processCmsTranslateJob(ctx context.Context, j *job.CmsTranslateJob) {
	defer func() {
		if r := recover(); r != nil {
			observability.Logger.Error("Worker panic (CmsTranslateJob)", zap.Any("panic", r), zap.String("job_id", j.ID.String()))
			_ = cmsTranslateRepo.MarkFailed(context.Background(), database.SQLX, j.ID, "Worker panic", fmt.Sprintf("%v", r))
		}
	}()

	app, err := appRepo.GetByID(ctx, database.SQLX, j.ApplicationID)
	if err != nil {
		_ = cmsTranslateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Application not found", err.Error())
		return
	}

	openAIService := resolveOpenAIService(app.OpenAIKey)
	if openAIService == nil {
		_ = cmsTranslateRepo.MarkFailed(ctx, database.SQLX, j.ID, "OpenAI API key not configured", "Configure in Application settings")
		return
	}

	// CMS item with template + fields preloaded.
	item, err := itemRepo.GetByIDWithTemplate(ctx, database.SQLX, j.CmsItemID)
	if err != nil {
		_ = cmsTranslateRepo.MarkFailed(ctx, database.SQLX, j.ID, "CMS item not found", err.Error())
		return
	}

	sourceLoc, err := cmsLocRepo.GetLatest(ctx, database.SQLX, j.CmsItemID, j.SourceLocale, j.Stage)
	if err != nil {
		_ = cmsTranslateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Source localization not found",
			fmt.Sprintf("locale %s stage %s: %v", j.SourceLocale, j.Stage, err))
		return
	}

	fieldTypes := make(map[string]string, len(item.Template.Fields))
	for _, f := range item.Template.Fields {
		fieldTypes[f.Key] = f.ValueType
	}

	translatedData, err := openAIService.TranslateCMSFields(
		ctx,
		map[string]interface{}(sourceLoc.Data),
		fieldTypes,
		j.SourceLocale,
		j.TargetLocale,
	)
	if err != nil {
		_ = cmsTranslateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Translation failed", err.Error())
		return
	}

	// Insert next version with race-safe retry. SaveLocalizationVersion handles
	// the partial unique index collision (two workers picking the same version
	// number under concurrency) and retries up to 5 times before erroring out.
	newLoc := &cms.Localization{
		CmsItemID:    j.CmsItemID,
		Locale:       j.TargetLocale,
		Stage:        j.Stage,
		Data:         repository.JSONB(translatedData),
		SourceLocale: j.SourceLocale,
		IsActive:     true,
		CreatedBy:    j.CreatedBy,
		UpdatedBy:    j.CreatedBy,
	}
	if err := cmsLocRepo.SaveLocalizationVersion(ctx, database.SQLX, newLoc); err != nil {
		_ = cmsTranslateRepo.MarkFailed(ctx, database.SQLX, j.ID, "Failed to save localization", err.Error())
		return
	}

	if err := cmsTranslateRepo.MarkCompleted(ctx, database.SQLX, j.ID); err != nil {
		observability.Logger.Warn("MarkCompleted failed (CmsTranslateJob)", zap.Error(err))
	}

	observability.Logger.Info("CmsTranslateJob completed",
		zap.String("job_id", j.ID.String()),
		zap.String("cms_item_id", j.CmsItemID.String()),
		zap.String("target_locale", j.TargetLocale),
	)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// changedOrNewKeys returns a subset of current containing only keys whose value
// changed (recursively for nested objects) or that are absent from prev.
// Keys present in prev but absent from current are NOT included here — see hasRemovedKeys.
func changedOrNewKeys(current, prev map[string]interface{}) map[string]interface{} {
	changed := make(map[string]interface{})
	for k, cv := range current {
		pv, exists := prev[k]
		if !exists {
			changed[k] = cv
			continue
		}
		cvMap, cvIsMap := cv.(map[string]interface{})
		pvMap, pvIsMap := pv.(map[string]interface{})
		if cvIsMap && pvIsMap {
			nested := changedOrNewKeys(cvMap, pvMap)
			if len(nested) > 0 {
				changed[k] = nested
			}
			continue
		}
		if !jsonEqual(cv, pv) {
			changed[k] = cv
		}
	}
	return changed
}

// hasRemovedKeys returns true if any key present in prev is absent from current (recursively).
func hasRemovedKeys(prev, current map[string]interface{}) bool {
	for k, pv := range prev {
		cv, exists := current[k]
		if !exists {
			return true
		}
		pvMap, pvIsMap := pv.(map[string]interface{})
		cvMap, cvIsMap := cv.(map[string]interface{})
		if pvIsMap && cvIsMap {
			if hasRemovedKeys(pvMap, cvMap) {
				return true
			}
		}
	}
	return false
}

// pruneToShape rebuilds target keeping only keys that exist in source (recursively).
// Keys present in target but absent from source are silently dropped — this keeps
// all locale translations structurally identical to the source.
func pruneToShape(target, source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source))
	for k := range source {
		tv, exists := target[k]
		if !exists {
			continue
		}
		sv := source[k]
		svMap, svIsMap := sv.(map[string]interface{})
		tvMap, tvIsMap := tv.(map[string]interface{})
		if svIsMap && tvIsMap {
			result[k] = pruneToShape(tvMap, svMap)
		} else {
			result[k] = tv
		}
	}
	return result
}

// mergeTranslations overlays additions onto base (recursively for nested objects).
// Values in additions take priority; keys only in base are preserved.
func mergeTranslations(base, additions map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range additions {
		if addMap, ok := v.(map[string]interface{}); ok {
			if baseMap, ok := result[k].(map[string]interface{}); ok {
				result[k] = mergeTranslations(baseMap, addMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// jsonbToStringMap flattens a repository.JSONB into a map[string]string,
// dropping non-string values. Used to feed component KeyContexts into the
// OpenAI service.
func jsonbToStringMap(j repository.JSONB) map[string]string {
	if len(j) == 0 {
		return nil
	}
	out := make(map[string]string, len(j))
	for k, v := range j {
		if s, ok := v.(string); ok && s != "" {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// jsonEqual compares two values by their JSON representation to avoid type-mismatch
// false positives (e.g. float64(5) vs int(5) after json.Unmarshal).
func jsonEqual(a, b interface{}) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

// countLeaves counts the number of leaf (non-object) values in a nested map.
// Used for logging how many keys were sent to AI.
func countLeaves(m map[string]interface{}) int {
	n := 0
	for _, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			n += countLeaves(nested)
		} else {
			n++
		}
	}
	return n
}

// resolveOpenAIService returns an OpenAIService using the app's key or the
// environment fallback. Returns nil if no key is available at all.
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
