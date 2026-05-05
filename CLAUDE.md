# CLAUDE.md — i18n-center

> Read this file at the start of every session. It captures the full architecture, conventions, and current feature state so you can work without re-reading the entire codebase.

---

## What this repo is

**i18n-center** is a centralized Translation Management System with a Headless CMS, built for LapakGaming / Joytify. It consists of:

| Directory | Stack | Purpose |
|-----------|-------|---------|
| `backend/` | Go 1.23, Gin, GORM, PostgreSQL, Redis | REST API + async job worker |
| `frontend/` | Next.js 14, TypeScript, Redux Toolkit, Tailwind | Admin UI |
| `i18ncenter-js/` | TypeScript (no framework) | JS/TS SDK for Next.js apps |
| `i18ncenter-go/` | Go | Go SDK for backend services |
| `e2e/` | Playwright | End-to-end API + UI tests |

---

## Backend conventions

### Framework & patterns
- **Router**: Gin (not Echo, not chi — only Gin in this repo)
- **ORM**: GORM with PostgreSQL (CloudSQL in production)
- **Auth**: JWT (`Authorization: Bearer <token>`) + application-scoped API keys (`X-API-Key`)
- **Money**: not applicable here (no monetary values)
- **UUIDs**: `github.com/google/uuid`, all primary keys are UUID v4 (`gen_random_uuid()`)
- **Soft deletes**: all main models use `gorm.DeletedAt`

### Module layout
```
backend/
  handlers/      # HTTP handlers (one file per resource group)
  models/        # GORM models
  services/      # Business logic (openai, audit, gcs)
  routes/        # Route registration (single routes.go)
  middleware/    # Auth, CORS, observability, rate limiting
  database/      # DB init + AutoMigrate
  cache/         # Redis helpers
  jobs/          # Async worker (translate jobs)
  observability/ # Zap logger + Datadog
  docs/          # Swagger auto-generated (never edit by hand)
```

### Adding a new endpoint
1. Define model in `models/` if needed; add to `AutoMigrate` in `database/database.go`
2. Create or extend handler in `handlers/`; add Swagger annotations
3. Register route in `routes/routes.go`
4. Run `swag init --generalInfo main.go --output docs` to regenerate docs
5. Add handler test in `handlers/*_test.go` following existing patterns

### Swagger annotations (mandatory for all endpoints)
```go
// @Summary      Short description
// @Description  Longer description
// @Tags         cms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Resource ID (UUID)"
// @Success      200  {object}  models.SomeModel
// @Failure      404  {object}  map[string]string
// @Router       /cms/items/{id} [get]
func (h *Handler) Method(c *gin.Context) {
```

Run `swag init --generalInfo main.go --output docs` after adding or changing annotations.

### Testing
```bash
cd backend
go test ./...          # all tests
go test ./handlers/... # handler tests only
```
All tests pass without a live DB (sqlmock). `observability.Logger` defaults to `zap.NewNop()` so tests don't panic.

### Environment variables
```
PORT, ENV, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
REDIS_HOST, REDIS_PORT, REDIS_PASSWORD, REDIS_DB
JWT_SECRET, JWT_EXPIRY
OPENAI_API_KEY           # per-application key overrides this global default
CORS_ORIGIN
LOG_SQL=false            # set true to log GORM queries
DD_ENABLED, DD_AGENT_HOST, DD_DOGSTATSD_PORT   # Datadog (production only)
# GCS — optional, required only for CMS image upload:
GCS_BUCKET, GCS_CREDENTIALS_BASE64, GCS_CMS_IMAGE_PREFIX, PIXELSHIFT_BASE_URL
```

---

## Frontend conventions

### Stack
- **Next.js 14 App Router**, TypeScript, Tailwind CSS
- **State**: Redux Toolkit (`store/slices/`) for auth, applications, components, translations
- **API client**: Axios instance in `services/api.ts` with request interceptor (JWT) and response interceptor (401 → redirect)
- **UI components**: `components/ui/` (Button, Card, Modal, Table, Badge, Input, Select, Textarea)
- **Code editor**: Monaco (`components/CodeEditor.tsx`, dynamic import, SSR disabled)
- **Rich text editor**: TipTap (`components/RichTextEditor.tsx`, SSR disabled)

### CMS pages use local React state (not Redux)
Only `auth`, `applications`, `components`, and `translations` have Redux slices. CMS pages (`/cms/templates`, `/cms/items`, `/cms/items/[id]`) manage state locally with `useState`/`useCallback`.

### Testing
```bash
cd frontend
npm test               # jest + coverage (threshold: 80% lines)
npx tsc --noEmit       # type check only
```

---

## Implemented features (authoritative list)

### Translation system
- **Application / Component / Tag / Page** models with full CRUD
- **Draft → Staging → Production** deployment workflow (per component, per locale)
- **Versioning** — every save creates a new version; revert creates a new version from old data
- **AI auto-translate** (async, 202 → poll `/translate-jobs/:id`): single locale and backfill all locales
- **"Translate from other language"** — in the component translation editor, pull content into the current locale from any other enabled locale (async job, same poll pattern)
- **Export / Import** — application-level JSON export, component-level import
- **Bootstrap Import** (`POST /applications/:id/bootstrap`) — bulk-creates components from a locale JSON
- **Feature tagging** (many-to-many component↔tag) — `by-tag` read API
- **Page grouping** (many-to-many component↔page) — `by-page` read API

### Component list
- Paginated response: `{ data, total, page, page_size, total_pages }`
- Query params: `application_id`, `search` (ILIKE, pg_trgm GIN indexed), `page`, `page_size`

### Headless CMS
Five new models in `backend/models/cms.go`:
- **CmsTemplate** — content schema; belongs to Application; has many CmsTemplateFields
- **CmsTemplateField** — `value_type`: `text` | `textarea` | `rich_text` | `json`
- **CmsItem** — content item; belongs to Application + CmsTemplate; identified by `identifier` string
- **CmsLocalization** — versioned content per item+locale+stage; `data` is JSONB
- **CmsTranslateJob** — async AI translate job; statuses: `pending` → `running` → `completed` / `failed`

CMS API endpoints (all require BearerAuth unless marked public):
```
# Templates
GET    /api/applications/:id/cms/templates
POST   /api/applications/:id/cms/templates
GET    /api/cms/templates/:id
PUT    /api/cms/templates/:id          # replaces all fields
DELETE /api/cms/templates/:id          # fails if items reference it

# Items
GET    /api/applications/:id/cms/items
POST   /api/applications/:id/cms/items
GET    /api/cms/items/:id              # preloads Template.Fields
PUT    /api/cms/items/:id
DELETE /api/cms/items/:id              # cascades localizations

# Localizations
GET    /api/cms/items/:id/localizations
GET    /api/cms/items/:id/localizations/detail?locale=en&stage=draft
POST   /api/cms/items/:id/localizations                  # {locale, stage, data} — always appends
POST   /api/cms/items/:id/localizations/translate        # async → 202 {job_id}
POST   /api/cms/items/:id/localizations/deploy           # {locale, from_stage, to_stage}
POST   /api/cms/items/:id/localizations/revert           # {locale, stage, version}
GET    /api/cms/items/:id/localizations/versions?locale=en&stage=draft

# Jobs
GET    /api/cms/translate-jobs/:job_id

# Public (API key auth, no role required)
GET    /api/applications/:id/cms/:identifier?locale=en&stage=production
# Returns: { identifier, locale, stage, data: { field_key: value } }

# Image upload (only registered when GCS env vars present)
POST   /api/cms/upload-image           # multipart/form-data; field: file (JPEG/PNG/GIF/WebP ≤10MB)
# Returns: { url: "https://img.lapakgaming.com/s/cms/{uuid}.ext" }
```

**GCS image upload**: uses stdlib JWT auth (no `cloud.google.com/go` dependency). Key: `GCS_CREDENTIALS_BASE64` (base64 service account JSON from Vault/external-secrets). Route is not registered if env vars are absent — server starts cleanly without GCS.

**CMS AI translation**: worker in `backend/jobs/worker.go` polls `cms_translate_jobs` with `FOR UPDATE SKIP LOCKED`. Translation is field-type-aware:
- `text` / `textarea` → standard OpenAI translate
- `rich_text` → HTML-aware translate (model instructed to preserve tags)
- `json` → copied as-is (not translated)

### RBAC
Three roles: `super_admin`, `operator`, `user_manager`. Translation read endpoints accept either JWT (any role) or API key (application-scoped).

### Observability
- Zap logger (JSON in production, dev console otherwise); defaults to `zap.NewNop()` so tests work without calling `InitLogger()`
- Datadog StatsD metrics (gated by `DD_ENABLED`)
- Only 4xx/5xx HTTP responses are logged (2xx suppressed)

---

## SDK — i18ncenter-js

```typescript
import { I18nCenterClient, CmsContent } from 'i18ncenter-js';

const client = new I18nCenterClient({
  apiUrl: 'https://api.example.com/api',
  apiToken: 'sk_...',        // API key or JWT
  defaultLocale: 'en',
  defaultStage: 'production',
  cacheTTL: 3600000,         // 1 hour (default)
  enableCache: true,
});
```

**Methods:**
| Method | Description |
|--------|-------------|
| `getTranslation(appCode, componentCode, locale?, stage?)` | Single component |
| `getMultipleTranslations(appCode, codes[], locale?, stage?)` | Bulk, cache-aware |
| `getTranslationsByTag(appId, tagCode, locale?, stage?)` | By tag |
| `getTranslationsByPage(appId, pageCode, locale?, stage?)` | By page |
| `getCmsContent(appId, identifier, locale?, stage?)` | CMS item by identifier → `CmsContent` |
| `clearCache()` | Clear in-memory cache |

`CmsContent` shape: `{ identifier: string; locale: string; stage: DeploymentStage; data: Record<string, any> }`

---

## E2E tests (Playwright)

Located in `e2e/`. Run sequentially (state-dependent). Global setup seeds all fixtures.

```bash
cd e2e
npx playwright test            # all tests
npx playwright test tests/06-cms.spec.ts  # CMS tests only
```

Test files:
| File | TC range | Coverage |
|------|----------|---------|
| `01-auth.spec.ts` | TC-TMS-001–010 | Login, JWT |
| `02-applications.spec.ts` | — | Application CRUD |
| `03-components.spec.ts` | — | Component CRUD |
| `04-translations.spec.ts` | — | Translation workflow |
| `05-read-api.spec.ts` | TC-TMS-040–049 | Public API (API key) |
| `06-cms.spec.ts` | TC-TMS-063–083 | CMS templates, items, localizations, public read, SDK |
| `07-ui.spec.ts` | TC-TMS-050–062 | Browser / dashboard UI |

State is passed between suites via `e2e/.test-state.json` (written by global-setup, read by each spec).

---

## Key files to know

| File | Why it matters |
|------|----------------|
| `backend/routes/routes.go` | Single source of truth for all routes and middleware chains |
| `backend/database/database.go` | AutoMigrate list — add new models here |
| `backend/jobs/worker.go` | Async job worker — handles TranslateJob and CmsTranslateJob |
| `backend/services/openai_service.go` | `Translate`, `TranslateHTML`, `TranslateCMSFields` |
| `backend/services/gcs_service.go` | GCS upload (stdlib JWT, no extra deps) |
| `backend/observability/logger.go` | Logger defaults to `zap.NewNop()` — safe in tests |
| `frontend/services/api.ts` | Axios client + all typed API helpers (`componentApi`, `cmsApi`, …) |
| `frontend/store/slices/componentSlice.ts` | Paginated component state |
| `e2e/global-setup.ts` | Seeds all test fixtures — edit here when adding new E2E resources |
| `e2e/helpers/state.ts` | `TestState` interface — add fields here when global-setup produces new IDs |

---

## Common pitfalls

- **Never edit `backend/docs/`** — it's generated by `swag init`. Always run swag after changing annotations.
- **Component `getAll` returns paginated shape** (`{ data, total, page, page_size, total_pages }`), not a raw array. Tests that mock it must return that shape.
- **CMS upload route is conditional** — `NewCmsUploadHandler()` returns `nil, err` when GCS env vars are absent; `routes.go` only registers the route if non-nil.
- **`observability.Logger` starts as `zap.NewNop()`** — `InitLogger()` is called by `main.go` at startup, but tests never call it. This is intentional.
- **CMS pages don't use Redux** — they fetch data directly via `cmsApi.*` and hold it in local state.
- **Swagger security tag on public endpoints**: `GetCmsItemByIdentifier` and translation read endpoints intentionally omit `@Security BearerAuth` — they accept API keys via `TranslationAuthMiddleware`.
