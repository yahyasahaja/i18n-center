// Package models holds the legacy struct definitions that were GORM-tagged.
// As of Commit I they are no longer the persistence layer — the repository
// packages (`repository/...`) own the database I/O. These types stick around
// because:
//
//   - Swagger annotations across the handlers still reference `models.X`.
//   - The legacy AuditServicer interface returns `[]models.AuditLog` for
//     backwards compatibility with the mock implementation. New code should
//     prefer the repository types directly (`audit.Log`, `application.Application`).
//
// GORM imports and hooks have been removed; struct tags now only carry
// `json` and `db` annotations.
package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JSONB type for PostgreSQL JSONB columns.
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

// StringArray type for PostgreSQL text[] columns. lib/pq has its own
// StringArray now — prefer that in new code; this one is kept because
// some Swagger annotations and legacy types still reference it.
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	if len(a) == 0 {
		return "{}", nil
	}
	values := make([]string, len(a))
	for i, v := range a {
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

	str = strings.TrimSpace(str)
	if str == "{}" {
		*a = []string{}
		return nil
	}

	str = strings.TrimPrefix(str, "{")
	str = strings.TrimSuffix(str, "}")
	if str == "" {
		*a = []string{}
		return nil
	}

	parts := strings.Split(str, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"' {
			part = part[1 : len(part)-1]
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

// UserRole represents user roles. Mirrored in `user.Role*` constants — both
// point at the same string values.
type UserRole string

const (
	RoleSuperAdmin  UserRole = "super_admin"
	RoleOperator    UserRole = "operator"
	RoleUserManager UserRole = "user_manager"
)

// User represents a user in the system. See `repository/user.User` for the
// canonical type.
type User struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	Username     string    `db:"username"      json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         UserRole  `db:"role"          json:"role"`
	IsActive     bool      `db:"is_active"     json:"is_active"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// Application — see `repository/application.Application` for the canonical type.
type Application struct {
	ID               uuid.UUID   `db:"id"                json:"id"`
	Name             string      `db:"name"              json:"name"`
	Code             string      `db:"code"              json:"code"`
	Description      string      `db:"description"       json:"description"`
	OpenAIKey        string      `db:"openai_key"        json:"-"`
	HasOpenAIKey     bool        `db:"-"                 json:"has_openai_key"`
	EnabledLanguages StringArray `db:"enabled_languages" json:"enabled_languages"`
	CreatedBy        uuid.UUID   `db:"created_by"        json:"created_by"`
	UpdatedBy        uuid.UUID   `db:"updated_by"        json:"updated_by"`
	CreatedAt        time.Time   `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time   `db:"updated_at"        json:"updated_at"`
}

// ApplicationAPIKey is a secret key for an application (used by client apps
// to access translations API). Only the key prefix is stored for display;
// the full key is shown once on create.
const APIKeyPrefix = "sk_"

type ApplicationAPIKey struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	KeyHash       string    `db:"key_hash"       json:"-"`
	KeyPrefix     string    `db:"key_prefix"     json:"key_prefix"`
	Name          string    `db:"name"           json:"name"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
}

// ApplicationLocaleDeploy tracks a locale added to an application and its
// deploy progress (draft -> staging -> production). See
// `repository/localedeploy.Deploy` for the canonical type.
type ApplicationLocaleDeploy struct {
	ID             uuid.UUID `db:"id"              json:"id"`
	ApplicationID  uuid.UUID `db:"application_id"  json:"application_id"`
	Locale         string    `db:"locale"          json:"locale"`
	StageCompleted string    `db:"stage_completed" json:"stage_completed"`
	CreatedAt      time.Time `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"      json:"updated_at"`
}

// AddLanguageJob status constants. Kept here for backward compatibility;
// new code should use `job.Status*` from `repository/job`.
const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
)

type AddLanguageJob struct {
	ID                  uuid.UUID `db:"id"                   json:"id"`
	ApplicationID       uuid.UUID `db:"application_id"       json:"application_id"`
	Locale              string    `db:"locale"               json:"locale"`
	AutoTranslate       bool      `db:"auto_translate"       json:"auto_translate"`
	Status              string    `db:"status"               json:"status"`
	TotalComponents     int       `db:"total_components"     json:"total_components"`
	CompletedComponents int       `db:"completed_components" json:"completed_components"`
	ErrorMessage        string    `db:"error_message"        json:"error_message,omitempty"`
	ErrorDetail         string    `db:"error_detail"         json:"error_detail,omitempty"`
	ClaimedBy           string    `db:"claimed_by"           json:"claimed_by,omitempty"`
	CreatedBy           uuid.UUID `db:"created_by"           json:"created_by"`
	CreatedAt           time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"           json:"updated_at"`
}

// TranslateJob type constants. See `repository/job.TranslateType*` for the
// canonical home.
const (
	TranslateJobTypeAutoTranslate = "auto_translate"
	TranslateJobTypeBackfill      = "backfill"
)

type TranslateJob struct {
	ID            uuid.UUID   `db:"id"             json:"id"`
	ApplicationID uuid.UUID   `db:"application_id" json:"application_id"`
	ComponentID   uuid.UUID   `db:"component_id"   json:"component_id"`
	JobType       string      `db:"job_type"       json:"job_type"`
	SourceLocale  string      `db:"source_locale"  json:"source_locale"`
	TargetLocales StringArray `db:"target_locales" json:"target_locales"`
	Status        string      `db:"status"         json:"status"`
	ErrorMessage  string      `db:"error_message"  json:"error_message,omitempty"`
	ErrorDetail   string      `db:"error_detail"   json:"error_detail,omitempty"`
	ClaimedBy     string      `db:"claimed_by"     json:"claimed_by,omitempty"`
	CreatedBy     uuid.UUID   `db:"created_by"     json:"created_by"`
	CreatedAt     time.Time   `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time   `db:"updated_at"     json:"updated_at"`
}

// Tag — see `repository/tag.Tag` for the canonical type.
type Tag struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	Code          string    `db:"code"           json:"code"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
}

// Page — see `repository/page.Page` for the canonical type.
type Page struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	Code          string    `db:"code"           json:"code"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
}

// Component represents a component within an application (e.g., pdp_form).
// See `repository/component.Component` for the canonical type.
type Component struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	Name          string    `db:"name"           json:"name"`
	Code          string    `db:"code"           json:"code"`
	Description   string    `db:"description"    json:"description"`
	Structure     JSONB     `db:"structure"      json:"structure"`
	KeyContexts   JSONB     `db:"key_contexts"   json:"key_contexts"`
	DefaultLocale string    `db:"default_locale" json:"default_locale"`
	Tags          []Tag     `db:"-"              json:"tags,omitempty"`
	Pages         []Page    `db:"-"              json:"pages,omitempty"`
	CreatedBy     uuid.UUID `db:"created_by"     json:"created_by"`
	UpdatedBy     uuid.UUID `db:"updated_by"     json:"updated_by"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
}

// DeploymentStage represents deployment stages. Mirrored by
// `translation.Stage` in the repository layer.
type DeploymentStage string

const (
	StageDraft      DeploymentStage = "draft"
	StageStaging    DeploymentStage = "staging"
	StageProduction DeploymentStage = "production"
)

// TranslationVersion — see `repository/translation.Version` for the canonical
// type.
type TranslationVersion struct {
	ID           uuid.UUID       `db:"id"            json:"id"`
	ComponentID  uuid.UUID       `db:"component_id"  json:"component_id"`
	Locale       string          `db:"locale"        json:"locale"`
	Stage        DeploymentStage `db:"stage"         json:"stage"`
	Version      int             `db:"version"       json:"version"`
	Data         JSONB           `db:"data"          json:"data"`
	SourceLocale string          `db:"source_locale" json:"source_locale,omitempty"`
	SourceData   JSONB           `db:"source_data"   json:"source_data,omitempty"`
	IsActive     bool            `db:"is_active"     json:"is_active"`
	CreatedBy    uuid.UUID       `db:"created_by"    json:"created_by"`
	UpdatedBy    uuid.UUID       `db:"updated_by"    json:"updated_by"`
	CreatedAt    time.Time       `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"    json:"updated_at"`
}
