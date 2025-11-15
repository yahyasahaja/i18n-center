# Translation System

## Overview

The translation system manages structured JSON translations with versioning, deployment stages, and AI-powered auto-translation.

## Translation Workflow

### 1. Component Structure

Each component has a JSON structure that defines the translation keys:

```json
{
  "form": {
    "name": {
      "label": "Name",
      "placeholder": "Enter your name"
    },
    "email": {
      "label": "Email",
      "placeholder": "Enter your email"
    }
  }
}
```

This structure is stored in `Component.Structure` (JSONB).

### 2. Translation Creation

When a translation is saved:

1. **Version Management:**
   - Current version (version 2) becomes version 1
   - New data becomes version 2
   - Old versions (> 2) are deleted

2. **Stage Management:**
   - `draft`: Work in progress
   - `staging`: Deployed to staging
   - `production`: Live production

3. **Storage:**
   - Stored in `TranslationVersion` table
   - Key: `(component_id, locale, stage, version)`

### 3. Translation Retrieval

**Default Behavior:**
- Returns version 2 (latest) for specified locale and stage
- Falls back to version 1 if version 2 doesn't exist

**Endpoint:**
```
GET /api/components/:id/translations?locale=en&stage=production
```

## Versioning System

### Version Structure

```
Version 1: Before save (original)
Version 2: After save (current)
```

### Revert Flow

1. User clicks "Revert"
2. System deletes version 2
3. Version 1 becomes the active version
4. User can continue editing

**Implementation:**
```go
func RevertTranslation(componentID, locale, stage) {
    // Delete version 2
    // Version 1 automatically becomes active
}
```

### Future-Proof Design

Database supports unlimited versions, but currently:
- Only 2 versions are kept
- Versions > 2 are automatically cleaned up
- Can be extended to full versioning later

## Deployment Stages

### Draft
- **Purpose:** Work in progress
- **Can:** Edit, revert, save
- **Cannot:** Deploy to staging/production

### Staging
- **Purpose:** Testing environment
- **Source:** Deployed from Draft
- **Can:** View, deploy to production
- **Cannot:** Edit directly (must edit Draft first)

### Production
- **Purpose:** Live production data
- **Source:** Deployed from Staging
- **Can:** View
- **Cannot:** Edit (must go through Draft → Staging → Production)

### Deployment Flow

```
Draft → (Deploy) → Staging → (Deploy) → Production
```

**Implementation:**
```go
func DeployTranslation(componentID, locale, fromStage, toStage) {
    // Copy version 2 from source stage
    // Create new version 2 in target stage
    // Keep version 1 as backup
}
```

## Auto-Translation

### OpenAI Integration

**Service:** `backend/services/openai_service.go`

**Features:**
- Translates JSON structures
- Preserves template values (e.g., `[last_name]`)
- Maintains JSON structure
- Handles nested objects

### Template Value Preservation

Template values in brackets are not translated:

```
Input:  "Hi [last_name]!"
Output: "Halo [last_name]!"  // [last_name] preserved
```

**Implementation:**
- Regex pattern: `\[([^\]]+)\]`
- Extract template values before translation
- Restore after translation

### Auto-Translate Single Locale

**Endpoint:**
```
POST /api/components/:id/translations/auto-translate
```

**Request:**
```json
{
  "source_locale": "en",
  "target_locale": "id",
  "stage": "draft"
}
```

**Process:**
1. Get source translation (version 2)
2. Extract template values
3. Call OpenAI API
4. Restore template values
5. Save as new translation

### Backfill All Locales

**Endpoint:**
```
POST /api/components/:id/translations/backfill
```

**Request:**
```json
{
  "source_locale": "en",
  "target_locales": ["id", "es", "fr"],
  "stage": "draft"
}
```

**Process:**
1. Get source translation
2. For each target locale:
   - Check if translation exists
   - If missing, auto-translate
   - Save translation
3. Return array of created translations

## Translation Data Structure

### JSON Structure

Translations are stored as JSONB with flexible structure:

```json
{
  "form": {
    "name": "Name",
    "email": "Email"
  },
  "button": {
    "submit": "Submit",
    "cancel": "Cancel"
  }
}
```

### Validation

- Must be valid JSON
- No duplicate keys (validated in frontend)
- Structure should match component structure (not enforced, but recommended)

## Comparison Feature

### Version Comparison

**Endpoint:**
```
GET /api/components/:id/translations/compare?locale=en&stage=draft
```

**Response:**
```json
{
  "version1": {
    "data": { ... },
    "created_at": "2024-01-01T00:00:00Z"
  },
  "version2": {
    "data": { ... },
    "created_at": "2024-01-02T00:00:00Z"
  }
}
```

**Frontend:** Uses `react-diff-viewer-continued` for side-by-side diff

## Export/Import

### Export

**Component Export:**
```
GET /api/components/:id/export?locale=en&stage=production
```

**Application Export:**
```
GET /api/applications/:id/export?stage=production
```

**Format:** JSON file download

### Import

**Component Import:**
```
POST /api/components/:id/import?locale=en&stage=draft
Content-Type: application/json

{
  "data": { ... }
}
```

**Process:**
1. Validate JSON
2. Save as new translation (version 2)
3. Previous version becomes version 1

## Service Layer

**Location:** `backend/services/translation_service.go`

**Key Methods:**
- `GetTranslation()`: Retrieve translation
- `SaveTranslation()`: Save with versioning
- `RevertTranslation()`: Revert to previous version
- `DeployTranslation()`: Deploy between stages
- `AutoTranslate()`: Single locale translation
- `BackfillTranslations()`: Multiple locale translation

## Frontend Integration

### Translation Editor

**Component:** `frontend/components/TranslationEditor.tsx`

**Features:**
- Monaco Editor for JSON editing
- Real-time validation
- Duplicate key detection
- Unsaved changes warning
- Version comparison
- Diff viewer

### State Management

- Current translation data
- Selected locale
- Selected stage
- Original JSON (for change detection)
- Loading/saving states

## Best Practices

1. **Always edit Draft first**, then deploy
2. **Test in Staging** before Production
3. **Use version comparison** before deploying
4. **Backfill missing locales** after structure changes
5. **Export regularly** for backup
6. **Validate JSON** before saving
7. **Preserve template values** in translations

## Future Enhancements

- [ ] Translation memory
- [ ] Translation suggestions
- [ ] Bulk operations
- [ ] Translation history/audit
- [ ] Translation quality checks
- [ ] Multi-user collaboration
- [ ] Translation comments/notes

