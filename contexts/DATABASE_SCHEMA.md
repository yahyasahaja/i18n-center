# Database Schema

## Overview

PostgreSQL database with GORM as ORM. Auto-migration on startup.

### Deployment notes (production)

- **Shared Cloud SQL with Hydra.** i18n-center runs in the same Postgres instance as the Hydra OAuth2 server (B2C login hot path). i18n-center's table names don't collide with Hydra's (`hydra_oauth2_*` vs unprefixed), but a dedicated schema (e.g. `i18n_center.*`) is on the roadmap for blast-radius isolation.
- **Connection pool ceilings** (set in `database.InitDatabase`):
  - `DB_MAX_OPEN_CONNS` = 20 per pod (default)
  - `DB_MAX_IDLE_CONNS` = 5
  - `DB_CONN_MAX_LIFETIME_MIN` = 30
  Sized so 3 pods × 20 = 60 connections leaves headroom in Cloud SQL's default `max_connections = 100` for Hydra.
- **Migrations are operator-driven via the `i18n-center-migrate` CLI.** The server binary never touches the schema at boot. `database.InitDatabase` only opens the connection and sizes the pool. Schema is bootstrapped once via `kubectl exec deploy/i18n-center-backend -- i18n-center-migrate up` against a fresh DB. Subsequent changes ship as new files in `backend/migrations/` (numbered, `goose`-formatted) and are applied the same way before each deploy that needs the new schema.

See [`backend/migrations/README.md`](../backend/migrations/README.md) for the Postgres safe-pattern playbook (`CREATE INDEX CONCURRENTLY`, `ADD CONSTRAINT ... NOT VALID`, expand-contract for renames).

## Models

### Application

Central entity representing an application that uses i18n.

```go
type Application struct {
    ID               uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    Name             string         `gorm:"not null"`
    Code             string         `gorm:"uniqueIndex;not null"`        // Unique identifier for API access
    Description      string
    OpenAIKey        string         `gorm:"type:text;column:openai_key"` // Encrypted in production
    HasOpenAIKey     bool           `gorm:"-" json:"has_openai_key"`     // Computed field
    EnabledLanguages StringArray    `gorm:"type:text[]"`                 // PostgreSQL array
    CreatedBy        uuid.UUID      `gorm:"type:uuid;index"`            // Audit: who created
    UpdatedBy        uuid.UUID      `gorm:"type:uuid;index"`            // Audit: who last updated
    CreatedAt        time.Time
    UpdatedAt        time.Time
    DeletedAt        gorm.DeletedAt `gorm:"index"`                      // Soft delete
}
```

**Relationships:**
- Has many Components

**Special Fields:**
- `Code`: Unique, human-readable identifier (e.g., "whatsapp", "mobile_app"). Used for API lookups instead of UUID.
- `EnabledLanguages`: PostgreSQL `text[]` array type
- `OpenAIKey`: Stored but not returned in JSON (use `HasOpenAIKey`)
- `CreatedBy`/`UpdatedBy`: Audit fields tracking who made changes

### Component

Represents a UI component that needs translations.

```go
type Component struct {
    ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    ApplicationID uuid.UUID      `gorm:"type:uuid;not null;index"`
    Name          string         `gorm:"not null"`
    Code          string         `gorm:"uniqueIndex;not null"`        // Unique identifier for API access
    Description   string
    Structure     JSONB          `gorm:"type:jsonb"`                 // JSON structure template
    DefaultLocale string         `gorm:"not null"`                  // First language used
    CreatedBy     uuid.UUID      `gorm:"type:uuid;index"`          // Audit: who created
    UpdatedBy     uuid.UUID      `gorm:"type:uuid;index"`          // Audit: who last updated
    CreatedAt     time.Time
    UpdatedAt     time.Time
    DeletedAt     gorm.DeletedAt `gorm:"index"`
}
```

**Relationships:**
- Belongs to Application
- Has many TranslationVersions

**Special Fields:**
- `Code`: Human-readable identifier (e.g., "pdp_form", "checkout"). **Unique per application** (composite unique index with `application_id`). Used for API lookups instead of UUID.
- `Structure`: JSONB type for flexible JSON storage
- `DefaultLocale`: First language used when creating component
- `CreatedBy`/`UpdatedBy`: Audit fields tracking who made changes

**Unique Constraint:**
- Component codes are unique **per application**, not globally unique
- Same component code can exist in different applications
- Composite unique index: `(application_id, code)`

### TranslationVersion

Stores translation data with versioning support.

```go
type TranslationVersion struct {
    ID          uuid.UUID         `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    ComponentID uuid.UUID         `gorm:"type:uuid;not null;index"`
    Locale      string            `gorm:"not null;index"`
    Stage       DeploymentStage   `gorm:"type:varchar(50);not null;index"`
    Version     int               `gorm:"not null;default:1"`        // 1 = before save, 2 = after save
    Data        JSONB             `gorm:"type:jsonb;not null"`       // The translation data
    IsActive    bool              `gorm:"default:true"`
    CreatedBy   uuid.UUID         `gorm:"type:uuid;index"`          // Audit: who created
    UpdatedBy   uuid.UUID         `gorm:"type:uuid;index"`          // Audit: who last updated
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt    `gorm:"index"`
}
```

**Relationships:**
- Belongs to Component

**Indexes:**
- `(component_id, locale, stage, version)` - For fast lookups
- Individual indexes on `component_id`, `locale`, `stage`

**Versioning:**
- Version 1: Before save (original)
- Version 2: After save (current)
- Versions > 2: Automatically cleaned up

### DeploymentStage

Enum type for deployment stages.

```go
type DeploymentStage string

const (
    StageDraft      DeploymentStage = "draft"
    StageStaging    DeploymentStage = "staging"
    StageProduction DeploymentStage = "production"
)
```

### User

User accounts with role-based access.

```go
type User struct {
    ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    Username     string     `gorm:"uniqueIndex;not null"`
    PasswordHash string     `gorm:"not null"`
    Role         UserRole   `gorm:"type:varchar(20);not null"`
    IsActive     bool       `gorm:"default:true"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**Relationships:**
- None (standalone)

**Roles:**
- `super_admin`: Full access
- `operator`: Manage i18n data
- `user_manager`: Manage users

## CMS Models

### CmsTemplate

Defines the field structure for a category of CMS content (`cms_templates` table).

```go
type CmsTemplate struct {
    ID            uuid.UUID          `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    ApplicationID uuid.UUID          `gorm:"type:uuid;not null;index"`
    Name          string             `gorm:"not null"`
    Code          string             `gorm:"not null"`            // Unique per application
    Description   string
    Fields        []CmsTemplateField `gorm:"foreignKey:CmsTemplateID"`
    CreatedAt     time.Time
    UpdatedAt     time.Time
    DeletedAt     gorm.DeletedAt     `gorm:"index"`
}
```

### CmsTemplateField

Individual field definition within a CMS template (`cms_template_fields` table).

```go
type CmsTemplateField struct {
    ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    CmsTemplateID uuid.UUID `gorm:"type:uuid;not null;index"`
    Key           string    `gorm:"not null"`
    Label         string    `gorm:"not null"`
    ValueType     string    `gorm:"not null"` // text | textarea | rich_text | json
    Required      bool      `gorm:"default:false"`
    SortOrder     int       `gorm:"default:0"`
}
```

### CmsItem

A content item belonging to an application and template (`cms_items` table).

```go
type CmsItem struct {
    ID            uuid.UUID   `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    ApplicationID uuid.UUID   `gorm:"type:uuid;not null;index"`
    TemplateID    uuid.UUID   `gorm:"type:uuid;not null;index"`
    Identifier    string      `gorm:"not null"`  // Used in public API URL
    Name          string      `gorm:"not null"`
    Description   string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    DeletedAt     gorm.DeletedAt `gorm:"index"`
}
```

### CmsLocalization

Versioned content for a CMS item per locale and stage (`cms_localizations` table).

```go
type CmsLocalization struct {
    ID           uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    CmsItemID    uuid.UUID       `gorm:"type:uuid;not null;index"`
    Locale       string          `gorm:"not null;index"`
    Stage        DeploymentStage `gorm:"type:varchar(50);not null;index"`
    Version      int             `gorm:"not null;default:1"`
    Data         JSONB           `gorm:"type:jsonb;not null"`  // { field_key: value }
    SourceLocale string                                         // Locale used as translation source
    IsActive     bool            `gorm:"default:true"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**Indexes:**
- `(cms_item_id, locale, stage, version)` — composite for fast lookups

### CmsTranslateJob

Async AI translation job for CMS content (`cms_translate_jobs` table).

```go
type CmsTranslateJob struct {
    ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    ApplicationID uuid.UUID `gorm:"type:uuid;not null;index"`
    CmsItemID     uuid.UUID `gorm:"type:uuid;not null;index"`
    SourceLocale  string    `gorm:"not null"`
    TargetLocale  string    `gorm:"not null"`
    Stage         string    `gorm:"not null"`
    Status        string    `gorm:"not null;default:'pending'"` // pending | running | completed | failed
    ErrorMessage  string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

---

## Search Indexes (pg_trgm)

`components.name` and `components.code` have GIN indexes using `pg_trgm` to support the `search` parameter on `GET /api/components` (ILIKE search with index acceleration):

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_components_name_trgm ON components USING GIN (name gin_trgm_ops);
CREATE INDEX idx_components_code_trgm ON components USING GIN (code gin_trgm_ops);
```

---

## Custom Types

### StringArray

Handles PostgreSQL `text[]` array type.

```go
type StringArray []string

func (a StringArray) Value() (driver.Value, error)
func (a *StringArray) Scan(value interface{}) error
```

**Usage:**
- `EnabledLanguages` in Application model
- Converts between Go `[]string` and PostgreSQL `text[]`

### JSONB

Flexible JSON storage type.

```go
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error)
func (j *JSONB) Scan(value interface{}) error
```

**Usage:**
- `Structure` in Component
- `Data` in TranslationVersion

## Relationships

```
Application (1) ──< (N) Component
Component (1) ──< (N) TranslationVersion
User (1) ──< (N) AuditLog (tracks who made changes)
```

## Audit Logging

All changes to resources are tracked in the `AuditLog` table:

- **Who**: `user_id`, `username`
- **What**: `action` (CREATE, UPDATE, DELETE, DEPLOY, AUTO_TRANSLATE, etc.)
- **Which Resource**: `resource_type`, `resource_id`, `resource_code`
- **When**: `created_at`
- **Where**: `ip_address`, `user_agent`
- **Changes**: `before` (JSONB), `after` (JSONB), `details` (JSONB)

**Location:** `backend/models/audit.go`, `backend/services/audit_service.go`

## Indexes

### Application
- `name` (index)
- `code` (unique) - Used for API lookups
- `created_by` (index) - Audit tracking
- `updated_by` (index) - Audit tracking

### Component
- `application_id` (foreign key, part of composite unique index)
- `code` (part of composite unique index with application_id) - Unique per application, used for API lookups
- `(application_id, code)` (composite unique index: `idx_component_app_code`) - Ensures code is unique per application
- `created_by` (index) - Audit tracking
- `updated_by` (index) - Audit tracking

### TranslationVersion
- `component_id`
- `locale`
- `stage`
- `idx_tv_lookup` (`component_id, locale, stage, version DESC` WHERE `deleted_at IS NULL`) — composite covering the hot read query (latest active version per component+locale+stage). Required for p95 <50ms at scale.
- `idx_tv_unique_version` (`component_id, locale, stage, version` WHERE `deleted_at IS NULL`) — **partial unique** index that catches the read-MAX-then-insert race. `services.TranslationService.saveVersion` retries up to 5× on collision.

### TranslateJob
- `idx_translate_jobs_dedupe` (`component_id, source_locale, (target_locales[1]), job_type` WHERE `deleted_at IS NULL AND status IN ('pending','running')`) — partial unique to dedupe double-clicks on AutoTranslate/Backfill. The handler does lookup-then-insert and rescues the unique-violation on race.

### CmsLocalization
- `idx_cms_loc_lookup` (`cms_item_id, locale, stage, version DESC` WHERE `deleted_at IS NULL`) — mirror of `idx_tv_lookup` for the CMS hot read path.
- `idx_cms_loc_unique_version` (`cms_item_id, locale, stage, version` WHERE `deleted_at IS NULL`) — partial unique. `services.SaveCmsLocalizationVersion` retries up to 5× on collision.

### CmsTranslateJob
- `idx_cms_translate_jobs_dedupe` (`cms_item_id, source_locale, target_locale, stage` WHERE `deleted_at IS NULL AND status IN ('pending','running')`) — mirror of `idx_translate_jobs_dedupe` for CMS translate jobs (single-target per row, so simpler index).

### CmsItem
- `idx_cms_item_app_identifier` (`application_id, identifier`) — composite unique, identifiers are case-folded to lowercase on create (`normalizeIdentifier`) so SDK lookups always match.

### User
- `username` (unique)

### AuditLog
- `user_id` (index) - Who made the change
- `resource_type` (index) - Type of resource (application, component, translation, user)
- `resource_id` (index) - ID of the resource
- `resource_code` (index) - Code of the resource (for easier lookup)
- `action` (index) - Action type (CREATE, UPDATE, DELETE, DEPLOY, AUTO_TRANSLATE, etc.)

## Soft Deletes

Applications and Components use soft deletes:
- `DeletedAt` field (GORM)
- Records are not physically deleted
- Queries automatically exclude soft-deleted records

## Migrations

GORM auto-migration runs on startup:
- Creates tables if not exist
- Adds columns if not exist
- Does NOT modify existing columns (manual migration needed)

**Location:** `backend/database/database.go`

### Migration Process

1. **Pre-Migration:** `migrateCodeFields()` handles data migration for new required fields
   - Checks if `code` column exists
   - Adds column as nullable if missing
   - Backfills existing rows with generated codes (from name)
   - Makes column NOT NULL after backfill
   - Handles both `applications` and `components` tables

2. **Auto-Migration:** GORM auto-migrates all models

```go
func InitDatabase() error {
    // ... connection ...

    // Handle migration for code fields (backfill existing data)
    if err := migrateCodeFields(); err != nil {
        return fmt.Errorf("failed to migrate code fields: %w", err)
    }

    // Auto-migrate tables
    return database.AutoMigrate(
        &models.Application{},
        &models.Component{},
        &models.TranslationVersion{},
        &models.User{},
        &models.AuditLog{},
    )
}
```

### Code Field Migration

When adding the `code` field to existing tables:
- **Problem**: Can't add NOT NULL column to table with existing rows
- **Solution**: Add as nullable → backfill → make NOT NULL
- **Code Generation**: `LOWER(REGEXP_REPLACE(name, '[^a-zA-Z0-9]+', '_', 'g'))`
  - Converts name to lowercase
  - Replaces non-alphanumeric with underscores
  - Example: "My App" → "my_app"

## Future Schema Changes

For production schema changes:
1. Create migration script
2. Test on staging
3. Backup production
4. Run migration
5. Update models if needed

## Data Types Reference

| Go Type | PostgreSQL Type | Notes |
|---------|---------------|-------|
| `uuid.UUID` | `uuid` | Primary keys |
| `string` | `varchar` or `text` | Text fields |
| `StringArray` | `text[]` | Array of strings |
| `JSONB` | `jsonb` | JSON data |
| `DeploymentStage` | `varchar(20)` | Enum-like |
| `UserRole` | `varchar(20)` | Enum-like |
| `time.Time` | `timestamp` | Created/Updated |
| `gorm.DeletedAt` | `timestamp` | Soft delete |

