package models

import (
	"time"

	"github.com/google/uuid"
)

// CMS value types for template fields. See `repository/cms.IsValidValueType`
// for validation.
const (
	CmsValueTypeText     = "text"
	CmsValueTypeTextarea = "textarea"
	CmsValueTypeRichText = "rich_text"
	CmsValueTypeJSON     = "json"
)

// CmsTemplate defines the schema for CMS items in an application. See
// `repository/cms.Template` for the canonical type.
type CmsTemplate struct {
	ID            uuid.UUID          `db:"id"             json:"id"`
	ApplicationID uuid.UUID          `db:"application_id" json:"application_id"`
	Name          string             `db:"name"           json:"name"`
	Code          string             `db:"code"           json:"code"`
	Description   string             `db:"description"    json:"description"`
	Fields        []CmsTemplateField `db:"-"              json:"fields,omitempty"`
	CreatedBy     uuid.UUID          `db:"created_by"     json:"created_by"`
	UpdatedBy     uuid.UUID          `db:"updated_by"     json:"updated_by"`
	CreatedAt     time.Time          `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time          `db:"updated_at"     json:"updated_at"`
}

// CmsTemplateField is a single field definition within a CmsTemplate.
type CmsTemplateField struct {
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

// CmsItem is a named CMS content object in an application that uses a
// CmsTemplate. Example: identifier="flash_sale_banner",
// template="banner_template". See `repository/cms.Item`.
type CmsItem struct {
	ID            uuid.UUID    `db:"id"             json:"id"`
	ApplicationID uuid.UUID    `db:"application_id" json:"application_id"`
	TemplateID    uuid.UUID    `db:"template_id"    json:"template_id"`
	Template      *CmsTemplate `db:"-"              json:"template,omitempty"`
	Identifier    string       `db:"identifier"     json:"identifier"`
	Name          string       `db:"name"           json:"name"`
	Description   string       `db:"description"    json:"description"`
	CreatedBy     uuid.UUID    `db:"created_by"     json:"created_by"`
	UpdatedBy     uuid.UUID    `db:"updated_by"     json:"updated_by"`
	CreatedAt    time.Time     `db:"created_at"     json:"created_at"`
	UpdatedAt    time.Time     `db:"updated_at"     json:"updated_at"`
}

// CmsLocalization is the per-locale content for a CmsItem. Data contains
// field_key → value pairs matching the CmsTemplate's fields. See
// `repository/cms.Localization`.
type CmsLocalization struct {
	ID           uuid.UUID       `db:"id"            json:"id"`
	CmsItemID    uuid.UUID       `db:"cms_item_id"   json:"cms_item_id"`
	Locale       string          `db:"locale"        json:"locale"`
	Stage        DeploymentStage `db:"stage"         json:"stage"`
	Version      int             `db:"version"       json:"version"`
	Data         JSONB           `db:"data"          json:"data"`
	SourceLocale string          `db:"source_locale" json:"source_locale,omitempty"`
	IsActive     bool            `db:"is_active"     json:"is_active"`
	CreatedBy    uuid.UUID       `db:"created_by"    json:"created_by"`
	UpdatedBy    uuid.UUID       `db:"updated_by"    json:"updated_by"`
	CreatedAt    time.Time       `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"    json:"updated_at"`
}

// CmsTranslateJob tracks async AI translation for CMS localizations. See
// `repository/job.CmsTranslateJob`.
const CmsTranslateJobTypeSingle = "cms_translate"

type CmsTranslateJob struct {
	ID            uuid.UUID       `db:"id"             json:"id"`
	ApplicationID uuid.UUID       `db:"application_id" json:"application_id"`
	CmsItemID     uuid.UUID       `db:"cms_item_id"    json:"cms_item_id"`
	SourceLocale  string          `db:"source_locale"  json:"source_locale"`
	TargetLocale  string          `db:"target_locale"  json:"target_locale"`
	Stage         DeploymentStage `db:"stage"          json:"stage"`
	Status        string          `db:"status"         json:"status"`
	ErrorMessage  string          `db:"error_message"  json:"error_message,omitempty"`
	ErrorDetail   string          `db:"error_detail"   json:"error_detail,omitempty"`
	ClaimedBy     string          `db:"claimed_by"     json:"claimed_by,omitempty"`
	CreatedBy     uuid.UUID       `db:"created_by"     json:"created_by"`
	CreatedAt     time.Time       `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"     json:"updated_at"`
}
