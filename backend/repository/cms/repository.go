// Package cms is the data access layer for the Headless CMS feature.
//
// Three tables, three repos in one package:
//   - cms_templates       — schema definitions, with cms_template_fields children
//   - cms_items           — instances of a template (identified by `identifier`)
//   - cms_localizations   — versioned, per-locale, per-stage content for items
//
// Identifier normalisation: cms_items.identifier is case-folded to lowercase
// at the handler layer (handlers/cms_item_handler.go:normalizeIdentifier)
// because the SDK lowercases before sending. The repo trusts that — it doesn't
// re-normalise — but every read path uses an exact-match WHERE.
//
// Localization versioning follows the same pattern as translation_versions:
// every save is an INSERT, MAX(version)+1 with race-retry via the partial
// unique index idx_cms_loc_unique_version. SaveLocalizationVersion encapsulates
// the retry loop.
package cms

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
	"github.com/lapakgaming/i18n-center/repository/translation"
)

// ─── Template + Field ───────────────────────────────────────────────────────

// Template is a CMS schema definition. Fields holds the ordered field
// definitions; populated only by repository methods that explicitly preload.
type Template struct {
	ID            uuid.UUID       `db:"id"             json:"id"`
	ApplicationID uuid.UUID       `db:"application_id" json:"application_id"`
	Name          string          `db:"name"           json:"name"`
	Code          string          `db:"code"           json:"code"`
	Description   string          `db:"description"    json:"description"`
	CreatedBy     uuid.UUID       `db:"created_by"     json:"created_by"`
	UpdatedBy     uuid.UUID       `db:"updated_by"     json:"updated_by"`
	CreatedAt     time.Time       `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"     json:"updated_at"`
	Fields        []TemplateField `db:"-"              json:"fields,omitempty"`
}

// TemplateField is one field in a Template. ValueType is one of:
// text | textarea | rich_text | json (see ValueType* constants).
type TemplateField struct {
	ID         uuid.UUID `db:"id"          json:"id"`
	TemplateID uuid.UUID `db:"template_id" json:"template_id"`
	Key        string    `db:"key"         json:"key"`
	Label      string    `db:"label"       json:"label"`
	ValueType  string    `db:"value_type"  json:"value_type"`
	Required   bool      `db:"required"    json:"required"`
	SortOrder  int       `db:"sort_order"  json:"sort_order"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}

const (
	ValueTypeText     = "text"
	ValueTypeTextarea = "textarea"
	ValueTypeRichText = "rich_text"
	ValueTypeJSON     = "json"
)

// IsValidValueType reports whether vt is one of the four supported values.
func IsValidValueType(vt string) bool {
	switch vt {
	case ValueTypeText, ValueTypeTextarea, ValueTypeRichText, ValueTypeJSON:
		return true
	}
	return false
}

// TemplateRepository is the contract for template + field persistence.
type TemplateRepository interface {
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Template, error)
	// GetByIDWithFields populates Fields in the returned Template (sorted by SortOrder).
	GetByIDWithFields(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Template, error)
	ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Template, error)
	Create(ctx context.Context, q repository.Queryer, t *Template) error
	Update(ctx context.Context, q repository.Queryer, t *Template) error
	SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error

	// LoadFields fetches a template's fields sorted by SortOrder. Exposed so
	// callers that have a Template in hand can populate Fields without a re-fetch.
	LoadFields(ctx context.Context, q repository.Queryer, templateID uuid.UUID) ([]TemplateField, error)
	// ReplaceFields deletes all existing fields for templateID and inserts the
	// provided set in one transaction. Used by UpdateTemplate which takes the
	// fields list as a complete replacement (not a patch).
	ReplaceFields(ctx context.Context, q repository.Queryer, templateID uuid.UUID, fields []TemplateField) error

	// CountItemsForTemplate returns how many CmsItems reference a given template.
	// Used by DeleteTemplate to refuse deletion when items still depend on it.
	CountItemsForTemplate(ctx context.Context, q repository.Queryer, templateID uuid.UUID) (int, error)
}

// ─── Item ───────────────────────────────────────────────────────────────────

// Item is a CMS content instance. Template is populated only by the
// With-template variants of the read methods.
type Item struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	TemplateID    uuid.UUID `db:"template_id"    json:"template_id"`
	Identifier    string    `db:"identifier"     json:"identifier"`
	Name          string    `db:"name"           json:"name"`
	Description   string    `db:"description"    json:"description"`
	CreatedBy     uuid.UUID `db:"created_by"     json:"created_by"`
	UpdatedBy     uuid.UUID `db:"updated_by"     json:"updated_by"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
	Template      *Template `db:"-"              json:"template,omitempty"`
}

type ItemRepository interface {
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Item, error)
	// GetByIDWithTemplate populates Item.Template (with Fields).
	GetByIDWithTemplate(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Item, error)
	// GetByAppIdentifier looks up by (application_id, identifier). ErrNotFound on miss.
	GetByAppIdentifier(ctx context.Context, q repository.Queryer, appID uuid.UUID, identifier string) (*Item, error)
	ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Item, error)
	Create(ctx context.Context, q repository.Queryer, i *Item) error
	Update(ctx context.Context, q repository.Queryer, i *Item) error
	SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error
}

// ─── Localization ────────────────────────────────────────────────────────────

// Localization is the per-locale, per-stage, versioned content for a CmsItem.
type Localization struct {
	ID           uuid.UUID         `db:"id"            json:"id"`
	CmsItemID    uuid.UUID         `db:"cms_item_id"   json:"cms_item_id"`
	Locale       string            `db:"locale"        json:"locale"`
	Stage        translation.Stage `db:"stage"         json:"stage"`
	Version      int               `db:"version"       json:"version"`
	Data         repository.JSONB  `db:"data"          json:"data"`
	SourceLocale string            `db:"source_locale" json:"source_locale,omitempty"`
	IsActive     bool              `db:"is_active"     json:"is_active"`
	CreatedBy    uuid.UUID         `db:"created_by"    json:"created_by"`
	UpdatedBy    uuid.UUID         `db:"updated_by"    json:"updated_by"`
	CreatedAt    time.Time         `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time         `db:"updated_at"    json:"updated_at"`
}

type LocalizationRepository interface {
	// GetLatest fetches the highest-versioned active row for (cmsItemID, locale, stage).
	GetLatest(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, locale string, stage translation.Stage) (*Localization, error)
	// GetByVersion fetches a specific historical version.
	GetByVersion(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, locale string, stage translation.Stage, version int) (*Localization, error)
	// ListAll returns every (non-deleted) localization for a CmsItem.
	ListAll(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID) ([]Localization, error)
	// ListVersions returns the version history for (cmsItemID, locale, stage), newest first.
	ListVersions(ctx context.Context, q repository.Queryer, cmsItemID uuid.UUID, locale string, stage translation.Stage) ([]Localization, error)
	// SaveLocalizationVersion inserts a new row with version = MAX(version)+1,
	// retrying up to 5× on the unique-constraint race. Preserves the
	// SaveCmsLocalizationVersion semantics from the GORM-era helper.
	SaveLocalizationVersion(ctx context.Context, q repository.Queryer, l *Localization) error
}
