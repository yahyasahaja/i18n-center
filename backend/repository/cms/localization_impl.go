package cms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/translation"
)

// maxSaveAttempts mirrors translation.maxSaveAttempts — small budget; a high
// collision count means the same (item, locale, stage) is being hammered,
// which is application-level pathology.
const maxSaveAttempts = 5

const (
	queryGetLatestLocalization = `
		SELECT id, cms_item_id, locale, stage, version,
		       data, source_locale, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM cms_localizations
		WHERE cms_item_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND is_active = TRUE
		  AND deleted_at IS NULL
		ORDER BY version DESC
		LIMIT 1
	`

	queryGetLocalizationByVersion = `
		SELECT id, cms_item_id, locale, stage, version,
		       data, source_locale, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM cms_localizations
		WHERE cms_item_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND version = $4
		  AND deleted_at IS NULL
	`

	queryListAllLocalizations = `
		SELECT id, cms_item_id, locale, stage, version,
		       data, source_locale, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM cms_localizations
		WHERE cms_item_id = $1
		  AND deleted_at IS NULL
		ORDER BY locale, stage, version DESC
	`

	queryListLocalizationVersions = `
		SELECT id, cms_item_id, locale, stage, version,
		       data, source_locale, is_active,
		       created_by, updated_by, created_at, updated_at
		FROM cms_localizations
		WHERE cms_item_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND deleted_at IS NULL
		ORDER BY version DESC
	`

	queryNextLocalizationVersion = `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM cms_localizations
		WHERE cms_item_id = $1
		  AND locale = $2
		  AND stage = $3
		  AND deleted_at IS NULL
	`

	queryInsertLocalization = `
		INSERT INTO cms_localizations (
			id, cms_item_id, locale, stage, version,
			data, source_locale, is_active,
			created_by, updated_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, TRUE, $8, $8, NOW(), NOW()
		)
	`
)

// localizationImpl is the default LocalizationRepository.
type localizationImpl struct{}

// NewLocalizationRepository returns the default Localization repository.
func NewLocalizationRepository() LocalizationRepository { return &localizationImpl{} }

func (r *localizationImpl) GetLatest(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, locale string, stage translation.Stage) (*Localization, error) {
	var l Localization
	if err := q.GetContext(ctx, &l, queryGetLatestLocalization, cmsItemID, locale, stage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (r *localizationImpl) GetByVersion(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, locale string, stage translation.Stage, version int) (*Localization, error) {
	var l Localization
	if err := q.GetContext(ctx, &l, queryGetLocalizationByVersion, cmsItemID, locale, stage, version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (r *localizationImpl) ListAll(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID) ([]Localization, error) {
	out := []Localization{}
	if err := q.SelectContext(ctx, &out, queryListAllLocalizations, cmsItemID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *localizationImpl) ListVersions(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, locale string, stage translation.Stage) ([]Localization, error) {
	out := []Localization{}
	if err := q.SelectContext(ctx, &out, queryListLocalizationVersions, cmsItemID, locale, stage); err != nil {
		return nil, err
	}
	return out, nil
}

// SaveLocalizationVersion replaces the GORM-era services.SaveCmsLocalizationVersion
// helper. Read-MAX-then-insert with retry on the partial-unique-index race —
// same pattern as translation.SaveVersion.
func (r *localizationImpl) SaveLocalizationVersion(ctx context.Context, q repository.Queryer, l *Localization) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	var lastErr error
	for attempt := 1; attempt <= maxSaveAttempts; attempt++ {
		var next int
		if err := q.GetContext(ctx, &next, queryNextLocalizationVersion, l.CmsItemID, l.Locale, l.Stage); err != nil {
			return fmt.Errorf("compute next cms_localization version: %w", err)
		}
		l.Version = next
		_, err := q.ExecContext(ctx, queryInsertLocalization,
			l.ID, l.CmsItemID, l.Locale, l.Stage, l.Version,
			l.Data, l.SourceLocale, l.CreatedBy,
		)
		if err == nil {
			return nil
		}
		lastErr = err
		if !repository.IsUniqueViolation(err) {
			return err
		}
		l.ID = uuid.New()
	}
	return fmt.Errorf("cms.SaveLocalizationVersion: exhausted %d retries on unique-version conflict: %w", maxSaveAttempts, lastErr)
}
