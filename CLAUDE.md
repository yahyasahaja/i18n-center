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
  models/        # Domain models (being migrated off GORM tags — see Commit I)
  services/      # Business logic (openai, audit, gcs, translation)
  repository/    # Data access layer (raw SQL via sqlx; one subpackage per resource)
  routes/        # Route registration (single routes.go)
  middleware/    # Auth, CORS, observability, rate limiting
  database/      # DB init — connection + pool only, NO schema mutation
  migrations/    # SQL migration files (goose), embedded into the binary
  cmd/migrate/   # Migration CLI — `i18n-center-migrate up/down/status/...`
  cache/         # Redis helpers
  jobs/          # Async worker + retention ticker
  observability/ # Zap logger + Datadog
  docs/          # Swagger auto-generated (never edit by hand)
```

### Adding a new endpoint
1. Define model in `models/` if needed (plain struct, `db:"col"` tags only — no `gorm:""`).
2. Add a migration: `go run ./cmd/migrate create add_<table> sql`, write `CREATE TABLE`/etc.
3. Add a repository under `repository/<resource>/`: const queries at top, `Repository` interface, `*sqlx.DB`-backed impl. Take `context.Context`; return `repository.ErrNotFound` etc.
4. Create or extend handler in `handlers/`; depend on the repository (or a service that wraps it). Add Swagger annotations.
5. Register route in `routes/routes.go`.
6. Run `swag init --generalInfo main.go --output docs` to regenerate docs.
7. Add handler test in `handlers/*_test.go` (sqlmock).

### Schema changes (post-init)
**Never** mutate the schema from inside the server binary. Add a new migration file under `backend/migrations/`, apply with `i18n-center-migrate up` in the pod. See `backend/migrations/README.md` for the Postgres safe-pattern playbook (CONCURRENTLY, NOT VALID, expand-contract, etc.). The server itself only opens a connection — no AutoMigrate, no `ensure*Indexes`, no boot-time DDL.

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
DB_MAX_OPEN_CONNS=20     # per-pod; shared Cloud SQL with Hydra, tune to avoid starvation
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME_MIN=30
REDIS_HOST, REDIS_PORT, REDIS_PASSWORD, REDIS_DB
JWT_SECRET, JWT_EXPIRY
OPENAI_API_KEY           # per-application key overrides this global default
CORS_ORIGIN              # MUST be an explicit origin when ENV=production (boot fails on '*' or empty)
LOG_SQL=false            # set true to log GORM queries
DD_ENABLED, DD_AGENT_HOST, DD_DOGSTATSD_PORT   # Datadog (production only)
# GCS — optional, required only for CMS image upload:
GCS_BUCKET, GCS_CREDENTIALS_BASE64, GCS_CMS_IMAGE_PREFIX, PIXELSHIFT_BASE_URL
```

### Deployment topology
- **GKE** — stateless pods, ≥2 replicas. The in-process worker (`jobs/worker.go`) is K8s-safe via `FOR UPDATE SKIP LOCKED` on all three job tables (`add_language_jobs`, `translate_jobs`, `cms_translate_jobs`).
- **Cloud SQL Postgres — SHARED with Hydra** (OAuth2 in B2C login hot path). Pool ceilings above keep i18n-center from starving Hydra. Table namespaces don't collide today (i18n-center uses unprefixed names; Hydra uses `hydra_oauth2_*`) but a dedicated schema is on the roadmap.
- **Redis on a single VM** (not Memorystore, per SoW). Single point of failure — code degrades gracefully (cache errors are logged, never block requests) but Redis-down means every request hits Postgres. Provisioning checklist: `maxmemory <size>`, `maxmemory-policy allkeys-lru`, `tcp-keepalive 60`.
- **Cloudflare** in front of public read endpoints, `Cache-Control: public, max-age=60, s-maxage=300, stale-while-revalidate=600` for production responses.

### Stateless-K8s patterns

Any background task that must run on **exactly one pod** uses a Postgres session-level advisory lock as a leader-election. Example: `jobs/cleanup.go` calls `pg_try_advisory_lock(<fixed-int-key>)` at each tick; losers no-op. Auto-release on session close means a crashed leader doesn't block the next tick.

Things that already use this pattern:
- `RunCleanupTicker` — `translation_versions` retention sweep (key `0x6931386e63746e6d`)

If you add another singleton background job, pick a new int64 key and document it next to `cleanupAdvisoryLockKey`.

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

## Repository pattern (raw SQL via sqlx)

The data layer is being moved off GORM onto raw SQL. New code MUST follow this pattern; old GORM call sites are being converted incrementally (Commits D through I).

**Rules:**
1. **Raw SQL only.** No ORM-driven query builders. Query strings live as package-level `const`s at the top of `repository_impl.go`.
2. **Conditional clauses inline.** For search/filter that varies per call, append the conditional fragment inside the function with numbered placeholders — never `fmt.Sprintf` user input into the SQL.
3. **Methods take `context.Context`** as first arg.
4. **Repositories accept a `repository.Queryer`** (the subset of sqlx satisfied by both `*sqlx.DB` and `*sqlx.Tx`) so the same method body works inside or outside a transaction. Multi-statement consistency uses `repository.WithTx(ctx, db, func(tx Queryer) error { ... })`.
5. **Soft deletes are explicit.** Every read includes `WHERE deleted_at IS NULL`. Every write that "deletes" sets `deleted_at = NOW()` — hard deletes are reserved for the retention job (Commit J).
6. **Domain sentinels** for not-found / conflict: `errors.Is(err, repository.ErrNotFound)`. Never let `sql.ErrNoRows` leak into handlers.
7. **`repository.JSONB`** for jsonb columns — implements `driver.Valuer` and `sql.Scanner`. Use this on any jsonb field (no GORM-style auto-wrapping).
8. **`repository.IsUniqueViolation(err)`** for SQLSTATE 23505 detection (race retry on version inserts, idempotency dedupe).

See `backend/repository/types.go` for the base abstractions and `backend/repository/<resource>/` for example impls.

### Migration status (Commit E in flight)
| Resource | Repository | Handlers wired |
|---|---|---|
| User | `repository/user` | ✅ |
| Application | `repository/application` | ✅ (CRUD + AddLanguage + DeleteLanguage; jobs path still on GORM) |
| ApplicationAPIKey | `repository/apikey` | ✅ (also powers `auth.ValidateAPIKey`) |
| Tag | `repository/tag` | ✅ |
| Page | `repository/page` | ✅ |
| Component | `repository/component` | ✅ (CRUD + tag/page attach via transaction) |
| TranslationVersion | — | Commit F |
| CmsTemplate, CmsItem, CmsLocalization, CmsTemplateField | — | Commit G |
| AuditLog, AddLanguageJob, TranslateJob, CmsTranslateJob, ApplicationLocaleDeploy | — | Commit H |

GORM stays loaded in `database.DB` until Commit I — handlers that haven't been ported yet still use it.

## Patterns to reuse (NOT re-implement)

When you find yourself writing one of these, use the existing helper instead:

- **Insert next version with retry on race** — `services.SaveCmsLocalizationVersion` for CMS, `TranslationService.saveVersion`/`saveVersionTx` for translations. Both rely on partial unique indexes (`idx_cms_loc_unique_version`, `idx_tv_unique_version`) and retry up to 5 times on duplicate-key error. **Do NOT** roll your own MAX(version)+1 read-then-insert — concurrent writers will collide and silently produce duplicate version rows.
- **Detect unique-key violation** — `services.IsUniqueViolation(err)`. Message-based heuristic (matches `SQLSTATE 23505`); no dependency on `jackc/pgconn`.
- **Cache invalidation after a translation write** — `services.InvalidateAfterTranslationWrite(componentID, locale, stage)`. Scopes the by-page/by-tag pattern delete to the affected (appID, locale, stage) cell so draft writes don't touch production cache.
- **Cache invalidation for app-wide changes** — `services.InvalidateApplicationReadCache(appID)`. Used by `DeleteLanguage` and the post-commit hook in `DeployLocale`.
- **Translate-job idempotency lookup** — `findActiveTranslateJob` (in `translation_handler.go`) for translations and `findActiveCmsTranslateJob` (in `cms_item_handler.go`) for CMS. Both back the dedupe partial unique index. Always lookup-then-insert, with a second lookup on the unique-violation race for the catch-up path.
- **Application API-key scoping on public endpoints** — `middleware.GetAPIKeyApplicationID(c)` returns `uuid.Nil` for JWT requests; non-nil means an API key authenticated this request and the handler MUST verify it matches the URL's application_id (see `GetTranslationsByPage`, `GetTranslationsByTag`, `GetCmsItemByIdentifier`). Missing this check is a cross-tenant leak.
- **CMS identifier normalization** — `normalizeIdentifier(s)` in `cms_item_handler.go` lowercases + trims. The SDK (`i18ncenter-js`) lowercases before sending; the server must do the same on create and read or you'll silently 404.
- **Public read endpoint cache headers** — `setPublicCacheHeaders(c, stage, sMaxage)` in `translation_handler.go`. Production stages emit `public, max-age=60, s-maxage=300, stale-while-revalidate=600` + `Vary: X-API-Key, Authorization, Accept-Encoding`; draft/staging get `private, no-store`.
