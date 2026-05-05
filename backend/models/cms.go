package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CMS value types for template fields
const (
	CmsValueTypeText      = "text"
	CmsValueTypeTextarea  = "textarea"
	CmsValueTypeRichText  = "rich_text"
	CmsValueTypeJSON      = "json"
)

// CmsTemplate defines the schema for CMS items in an application.
type CmsTemplate struct {
	ID            uuid.UUID        `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID        `gorm:"type:uuid;not null;index" json:"application_id"`
	Application   Application      `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Name          string           `gorm:"not null" json:"name"`
	Code          string           `gorm:"type:varchar(100);not null;uniqueIndex:idx_cms_template_app_code" json:"code"`
	Description   string           `json:"description"`
	Fields        []CmsTemplateField `gorm:"foreignKey:TemplateID;constraint:OnDelete:CASCADE" json:"fields,omitempty"`
	CreatedBy     uuid.UUID        `gorm:"type:uuid;index" json:"created_by"`
	UpdatedBy     uuid.UUID        `gorm:"type:uuid;index" json:"updated_by"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	DeletedAt     gorm.DeletedAt   `gorm:"index" json:"-"`
}

func (t *CmsTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// CmsTemplateField is a single field definition within a CmsTemplate.
type CmsTemplateField struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TemplateID uuid.UUID `gorm:"type:uuid;not null;index" json:"template_id"`
	Key        string    `gorm:"type:varchar(100);not null" json:"key"`
	Label      string    `gorm:"type:varchar(255);not null" json:"label"`
	ValueType  string    `gorm:"type:varchar(50);not null" json:"value_type"` // text | textarea | rich_text | json
	Required   bool      `gorm:"default:false" json:"required"`
	SortOrder  int       `gorm:"default:0" json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (f *CmsTemplateField) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// CmsItem is a named CMS content object in an application that uses a CmsTemplate.
// Example: identifier="flash_sale_banner", template="banner_template".
type CmsItem struct {
	ID            uuid.UUID   `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID   `gorm:"type:uuid;not null;uniqueIndex:idx_cms_item_app_identifier" json:"application_id"`
	Application   Application `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	TemplateID    uuid.UUID   `gorm:"type:uuid;not null;index" json:"template_id"`
	Template      CmsTemplate `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	Identifier    string      `gorm:"type:varchar(100);not null;uniqueIndex:idx_cms_item_app_identifier" json:"identifier"`
	Name          string      `gorm:"not null" json:"name"`
	Description   string      `json:"description"`
	CreatedBy     uuid.UUID   `gorm:"type:uuid;index" json:"created_by"`
	UpdatedBy     uuid.UUID   `gorm:"type:uuid;index" json:"updated_by"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (i *CmsItem) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// CmsLocalization is the per-locale content for a CmsItem.
// Data contains field_key → value pairs matching the CmsTemplate's fields.
type CmsLocalization struct {
	ID           uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CmsItemID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"cms_item_id"`
	CmsItem      CmsItem         `gorm:"foreignKey:CmsItemID" json:"cms_item,omitempty"`
	Locale       string          `gorm:"type:varchar(20);not null;index" json:"locale"`
	Stage        DeploymentStage `gorm:"type:varchar(50);not null;index" json:"stage"`
	Version      int             `gorm:"not null;default:1" json:"version"`
	Data         JSONB           `gorm:"type:jsonb;not null" json:"data"`
	SourceLocale string          `gorm:"type:varchar(20)" json:"source_locale,omitempty"`
	IsActive     bool            `gorm:"default:true" json:"is_active"`
	CreatedBy    uuid.UUID       `gorm:"type:uuid;index" json:"created_by"`
	UpdatedBy    uuid.UUID       `gorm:"type:uuid;index" json:"updated_by"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	DeletedAt    gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (l *CmsLocalization) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// CmsTranslateJob tracks async AI translation for CMS localizations.
const CmsTranslateJobTypeSingle = "cms_translate"

type CmsTranslateJob struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID       `gorm:"type:uuid;not null;index" json:"application_id"`
	CmsItemID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"cms_item_id"`
	SourceLocale  string          `gorm:"type:varchar(20);not null" json:"source_locale"`
	TargetLocale  string          `gorm:"type:varchar(20);not null" json:"target_locale"`
	Stage         DeploymentStage `gorm:"type:varchar(50);not null" json:"stage"`
	Status        string          `gorm:"type:varchar(50);not null;default:pending;index" json:"status"`
	ErrorMessage  string          `gorm:"type:text" json:"error_message,omitempty"`
	ErrorDetail   string          `gorm:"type:text" json:"error_detail,omitempty"`
	ClaimedBy     string          `gorm:"type:varchar(255)" json:"claimed_by,omitempty"`
	CreatedBy     uuid.UUID       `gorm:"type:uuid;index" json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (j *CmsTranslateJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return nil
}

func (CmsTranslateJob) TableName() string {
	return "cms_translate_jobs"
}
