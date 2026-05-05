# Translation System

## Overview

The translation system manages structured JSON translations with versioning, deployment stages, and AI-powered auto-translation. AI operations (auto-translate, backfill) are **async** — they return a `job_id` immediately and are processed by a background worker, so the HTTP request never blocks on OpenAI.

---

## Translation Workflow

### 1. Component Structure

Each component has a JSON structure defining translation keys (stored in `Component.Structure` JSONB):

```json
{
  "form": {
    "name": { "label": "Name", "placeholder": "Enter your name" },
    "email": { "label": "Email", "placeholder": "Enter your email" }
  }
}
```

### 2. Translation Creation

When a translation is saved, a new `TranslationVersion` row is created (always insert, never overwrite). Key: `(component_id, locale, stage, version)`. Old versions beyond the last 50 are pruned automatically.

### 3. Deployment Stages

```
draft → (Deploy) → staging → (Deploy) → production
```

| Stage | Purpose | Can edit? |
|-------|---------|-----------|
| `draft` | Work in progress | ✅ |
| `staging` | Testing environment | ❌ (must edit draft first) |
| `production` | Live data | ❌ |

---

## AI Auto-Translation

### Architecture

AI translation is fully **async**. Both endpoints return **202 Accepted** with a `job_id`. Clients poll `GET /translate-jobs/:job_id` for status.

```
POST /components/:id/translations/auto-translate
    → 202 { job_id, status: "pending", message }
    → worker picks up job from DB
    → worker calls OpenAI (batch JSON, 1 call per component)
    → worker saves TranslationVersion to DB
    → GET /translate-jobs/:job_id → { status: "completed" | "failed" }
```

### Worker Design

- `TranslateJob` is DB-backed (`translate_jobs` table) — no in-memory state
- Multiple K8s replicas safe via `FOR UPDATE SKIP LOCKED`
- Stuck jobs (running > 15 min) are automatically reset to `pending`
- Failed jobs include `error_message` and `error_detail` for debugging

### OpenAI Batch Strategy

Instead of one API call per string value (which could be 1,000+ calls), the worker sends the **entire component JSON in one call** using `response_format: json_object`:

- **1 call per component** (was: 1 call per string value)
- `gpt-3.5-turbo-0125` with `response_format: json_object` guarantees valid JSON output
- Validates: all keys preserved, no extra keys, all `[bracketed]` placeholders survive
- Retries: up to 3× with validation check; falls back to per-key if all retries fail
- Components > 12,000 chars fall back directly to per-key mode

### Retry & Backoff Policy

| Error type | Action |
|-----------|--------|
| 429 (rate limit) | Exponential backoff: 5s → 10s → 20s |
| 400 / 401 / 403 | Fail immediately, do not retry (permanent) |
| 5xx / network | Exponential backoff: 2s → 4s → 8s |
| JSON validation failure | Retry with 1s pause (model may give better output) |

After 3 failed attempts → fall back to per-key translation (one OpenAI call per string value). This is slower but never loses data.

### Placeholder Preservation (`[bracketed]` tokens)

Template variables like `[name]`, `[count]` must survive translation intact.

Two-layer protection:
1. **Prompt instruction**: explicit rule in system prompt to copy placeholders verbatim
2. **Post-processing**: `PreserveTemplateValues(source, translated)` applies 3-tier recovery:
   - Already present → skip
   - GPT renamed variable but kept brackets → ordinal swap
   - Completely missing → append to end of string (never silently lost)

---

## Job Lifecycle

### `TranslateJob` model

```go
type TranslateJob struct {
    ID            uuid.UUID    // job_id returned by API
    ApplicationID uuid.UUID
    ComponentID   uuid.UUID
    JobType       string       // "auto_translate" | "backfill"
    SourceLocale  string
    TargetLocales StringArray  // always exactly 1 locale per job
    Status        string       // "pending" | "running" | "completed" | "failed"
    ErrorMessage  string       // short summary on failure
    ErrorDetail   string       // full error for debugging
    ClaimedBy     string       // K8s pod hostname
    CreatedBy     uuid.UUID
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### Poll Endpoints

```
GET /translate-jobs/:job_id
    → 200 { ...TranslateJob }

GET /components/:id/translate-jobs?status=pending,running
    → 200 { jobs: [...TranslateJob] }
    (returns last 50, newest first)
```

---

## Auto-Translate Endpoints

### Auto-translate (single locale)

```
POST /components/:id/translations/auto-translate
Authorization: Bearer <JWT>

{
  "source_locale": "en",
  "target_locale": "id",
  "stage": "draft"
}

→ 202 Accepted
{
  "job_id": "uuid",
  "status": "pending",
  "message": "Translation job enqueued. Poll /translate-jobs/{job_id} for status."
}
```

### Backfill (multiple locales)

Each target locale gets its own `TranslateJob` for independent tracking and retry:

```
POST /components/:id/translations/backfill
Authorization: Bearer <JWT>

{
  "source_locale": "en",
  "target_locales": ["id", "es", "fr"],
  "stage": "draft"
}

→ 202 Accepted
{
  "job_ids": ["uuid1", "uuid2", "uuid3"],
  "count": 3,
  "message": "3 translation jobs enqueued..."
}
```

---

## Frontend Integration

### Dashboard Polling Pattern

The `TranslationEditor` component in `frontend/components/TranslationEditor.tsx`:

1. On button click → calls auto-translate or backfill → gets `job_id(s)`
2. Shows `toast.loading('Translating...')` — editor **stays interactive** (no full-page spinner)
3. Polls `GET /translate-jobs/:job_id` every 2 seconds
4. On `completed` → dismisses toast, shows success, reloads translation if locale matches
5. On `failed` → shows error toast with `error_message` from job

```typescript
// Polling helper
const pollTranslateJob = async (jobId: string) => {
  for (let i = 0; i < 120; i++) {  // 4 min max
    const res = await translationApi.getTranslateJobStatus(jobId)
    if (res.status === 'completed') return { status: 'completed' }
    if (res.status === 'failed') return { status: 'failed', error: res.error_message }
    await new Promise(r => setTimeout(r, 2000))
  }
  return { status: 'failed', error: 'Timed out' }
}
```

State distinction:
- `loading` (boolean) — blocks editor while **fetching** translation data from server
- `translating` (boolean) — disables translate buttons while **async job is polling** (editor stays usable)

---

## Manual Translation Endpoints

```
GET    /components/:id/translations?locale=en&stage=production
POST   /components/:id/translations                             (save)
POST   /components/:id/translations/revert?locale=en&stage=draft
POST   /components/:id/translations/deploy
GET    /components/:id/translations/compare?locale=en&stage=draft
GET    /components/:id/translations/versions?locale=en&stage=draft
```

---

## Export / Import

```
GET  /applications/:id/export?stage=production     (full app export)
GET  /components/:id/export?locale=en&stage=draft  (component export)
POST /components/:id/import?locale=en&stage=draft  (component import)
POST /applications/:id/bootstrap                   (bulk seed from locale JSON)
```

Bootstrap is idempotent — safe to re-run. Splits top-level object keys into separate components; flat primitive keys go into a `common` component.

---

## Best Practices

1. Always edit **draft** first, then deploy draft → staging → production
2. Use **version comparison** before deploying
3. After adding a new component, **backfill** missing locales immediately
4. AI translate runs on draft — review before promoting to staging
5. Export regularly as backup
6. `[bracketed]` placeholders in translation strings are **never translated** — treat them as opaque tokens

---

## CMS Content Translation

### Overview

CMS items have their own AI translation flow that mirrors the component translation async job pattern but is field-type-aware. The endpoint is:

```
POST /api/cms/items/:id/localizations/translate
Authorization: Bearer <JWT>

{
  "source_locale": "en",
  "target_locale": "id",
  "stage": "draft"
}

→ 202 Accepted
{
  "job_id": "uuid",
  "status": "pending",
  "message": "CMS translation job enqueued. Poll /cms/translate-jobs/{job_id} for status."
}
```

Poll for completion:

```
GET /api/cms/translate-jobs/:job_id
→ 200 { id, status: "completed" | "failed", error_message, ... }
```

### Field-Type-Aware Translation

Each field in a CMS template has a `value_type` that controls how the AI translate worker handles it:

| `value_type` | Translation behaviour |
|---|---|
| `text` | Standard string translate via OpenAI (same as component key translation) |
| `textarea` | Standard string translate via OpenAI |
| `rich_text` | HTML-aware translate — preserves HTML tags, translates only text nodes |
| `json` | Copied verbatim from source locale (not translated) |

The worker reads the source `CmsLocalization.Data` (a JSONB map of field_key → value), processes each field according to its `value_type` from the template, then saves a new `CmsLocalization` row in the target locale.

### Job Lifecycle

`CmsTranslateJob` follows the same state machine as `TranslateJob`:

```
pending → running → completed
                 ↘ failed (error_message set)
```

- DB-backed — safe across multiple K8s replicas (`FOR UPDATE SKIP LOCKED`)
- Stuck jobs (running > 15 min) are automatically reset to `pending`
- The frontend CMS editor polls every 2 seconds after triggering a translate, using the same pattern as the component translation editor

### Translate-from-Other-Language (TranslationEditor)

In the component translation editor, a **"Translate from:"** row shows all other enabled locales as buttons. Clicking one enqueues an async `auto-translate` job from the selected source locale into the currently viewed locale. The same polling pattern applies — a loading toast appears while the job runs, and the editor stays interactive throughout.
