package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONB type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// StringArray type for PostgreSQL text[] columns
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	if len(a) == 0 {
		return "{}", nil
	}
	// Format as PostgreSQL array: {"value1","value2"}
	values := make([]string, len(a))
	for i, v := range a {
		// Escape quotes and backslashes
		v = strings.ReplaceAll(v, `\`, `\\`)
		v = strings.ReplaceAll(v, `"`, `\"`)
		values[i] = `"` + v + `"`
	}
	return "{" + strings.Join(values, ",") + "}", nil
}

func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return fmt.Errorf("cannot scan %T into StringArray", value)
	}

	// Parse PostgreSQL array format: {value1,value2} or {"value1","value2"}
	str = strings.TrimSpace(str)
	if str == "{}" {
		*a = []string{}
		return nil
	}

	// Remove curly braces
	str = strings.TrimPrefix(str, "{")
	str = strings.TrimSuffix(str, "}")

	if str == "" {
		*a = []string{}
		return nil
	}

	// Split by comma and trim quotes
	parts := strings.Split(str, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Remove surrounding quotes if present
		if len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"' {
			part = part[1 : len(part)-1]
			// Unescape
			part = strings.ReplaceAll(part, `\"`, `"`)
			part = strings.ReplaceAll(part, `\\`, `\`)
		}
		if part != "" {
			result = append(result, part)
		}
	}

	*a = result
	return nil
}

// UserRole represents user roles
type UserRole string

const (
	RoleSuperAdmin  UserRole = "super_admin"
	RoleOperator    UserRole = "operator"
	RoleUserManager UserRole = "user_manager"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"not null" json:"-"`
	Role         UserRole       `gorm:"type:varchar(50);not null" json:"role"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Application represents an application (e.g., whatsapp)
type Application struct {
	ID               uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name             string         `gorm:"not null" json:"name"`
	Code             string         `gorm:"uniqueIndex;not null" json:"code"` // Unique identifier for API access
	Description      string         `json:"description"`
	OpenAIKey        string         `gorm:"type:text;column:openai_key" json:"-"` // Encrypted in production
	HasOpenAIKey     bool           `gorm:"-" json:"has_openai_key"`              // Computed field
	EnabledLanguages StringArray    `gorm:"type:text[]" json:"enabled_languages"`
	CreatedBy        uuid.UUID      `gorm:"type:uuid;index" json:"created_by"`
	UpdatedBy        uuid.UUID      `gorm:"type:uuid;index" json:"updated_by"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApplicationAPIKey is a secret key for an application (used by client apps to access translations API).
// Only the key prefix is stored for display; the full key is shown once on create.
const APIKeyPrefix = "sk_"

type ApplicationAPIKey struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"application_id"`
	Application   Application    `gorm:"foreignKey:ApplicationID" json:"-"`
	KeyHash       string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`   // SHA-256 hex of the full key
	KeyPrefix     string         `gorm:"type:varchar(20);not null;index" json:"key_prefix"` // First 12 chars for display (e.g. sk_abc12345)
	Name          string         `gorm:"type:varchar(255)" json:"name"`                     // Optional label
	CreatedAt     time.Time      `json:"created_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApplicationLocaleDeploy tracks a locale added to an application and its deploy progress (draft -> staging -> production)
type ApplicationLocaleDeploy struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID  uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_app_locale" json:"application_id"`
	Locale         string         `gorm:"type:varchar(20);not null;uniqueIndex:idx_app_locale" json:"locale"`
	StageCompleted string         `gorm:"type:varchar(50);not null;default:draft" json:"stage_completed"` // draft, staging, production
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// AddLanguageJob is a DB-backed job for "add language + auto-translate". No in-memory state; safe for K8s scaling.
const (
	JobStatusPending   = "pending"
	JobStatusRunning  = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed   = "failed"
)

type AddLanguageJob struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"application_id"`
	Locale        string         `gorm:"type:varchar(20);not null" json:"locale"`
	AutoTranslate bool           `gorm:"not null" json:"auto_translate"`
	Status        string         `gorm:"type:varchar(50);not null;default:pending;index" json:"status"` // pending, running, completed, failed
	ErrorMessage  string         `gorm:"type:text" json:"error_message,omitempty"`
	ErrorDetail   string         `gorm:"type:text" json:"error_detail,omitempty"`
	ClaimedBy     string         `gorm:"type:varchar(255)" json:"claimed_by,omitempty"` // pod/instance id for debugging (K8s HOSTNAME)
	CreatedBy     uuid.UUID      `gorm:"type:uuid;index" json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (AddLanguageJob) TableName() string {
	return "add_language_jobs"
}

// Tag is a label that can be attached to components (scoped per application). Identified by code only.
type Tag struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"application_id"`
	Application   Application    `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Code          string         `gorm:"type:varchar(100);not null;uniqueIndex:idx_tag_app_code" json:"code"` // Unique per application
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// Page is a grouping that can be attached to components (scoped per application). Identified by code only.
type Page struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"application_id"`
	Application   Application    `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Code          string         `gorm:"type:varchar(100);not null;uniqueIndex:idx_page_app_code" json:"code"` // Unique per application
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// Component represents a component within an application (e.g., pdp_form)
type Component struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_component_app_code" json:"application_id"`
	Application   Application    `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Name          string         `gorm:"not null" json:"name"`
	Code          string         `gorm:"uniqueIndex:idx_component_app_code;not null" json:"code"` // Unique per application (composite with application_id)
	Description   string         `json:"description"`
	Structure     JSONB          `gorm:"type:jsonb" json:"structure"` // The JSON structure template
	DefaultLocale string         `gorm:"not null" json:"default_locale"`
	Tags          []Tag          `gorm:"many2many:component_tags;" json:"tags,omitempty"`
	Pages         []Page         `gorm:"many2many:component_pages;" json:"pages,omitempty"`
	CreatedBy     uuid.UUID      `gorm:"type:uuid;index" json:"created_by"`
	UpdatedBy     uuid.UUID      `gorm:"type:uuid;index" json:"updated_by"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// DeploymentStage represents deployment stages
type DeploymentStage string

const (
	StageDraft      DeploymentStage = "draft"
	StageStaging    DeploymentStage = "staging"
	StageProduction DeploymentStage = "production"
)

// TranslationVersion represents a version of translations
type TranslationVersion struct {
	ID          uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ComponentID uuid.UUID       `gorm:"type:uuid;not null;index" json:"component_id"`
	Component   Component       `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	Locale      string          `gorm:"not null;index" json:"locale"`
	Stage       DeploymentStage `gorm:"type:varchar(50);not null;index" json:"stage"`
	Version     int             `gorm:"not null;default:1" json:"version"` // 1, 2, 3, ... current = latest
	Data        JSONB           `gorm:"type:jsonb;not null" json:"data"`   // The translation data
	IsActive    bool            `gorm:"default:true" json:"is_active"`
	CreatedBy   uuid.UUID       `gorm:"type:uuid;index" json:"created_by"`
	UpdatedBy   uuid.UUID       `gorm:"type:uuid;index" json:"updated_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
}

// BeforeCreate hooks
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (a *Application) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (c *Component) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (t *Tag) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (p *Page) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (tv *TranslationVersion) BeforeCreate(tx *gorm.DB) error {
	if tv.ID == uuid.Nil {
		tv.ID = uuid.New()
	}
	return nil
}

func (a *ApplicationLocaleDeploy) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (j *AddLanguageJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return nil
}
