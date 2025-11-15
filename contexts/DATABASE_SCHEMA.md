# Database Schema

## Overview

PostgreSQL database with GORM as ORM. Auto-migration on startup.

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
- `(component_id, locale, stage, version)` - Composite for lookups

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

