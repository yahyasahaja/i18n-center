# System Architecture

## Overview

i18n-center is a centralized internationalization management service built with a microservices-ready architecture.

## Tech Stack

### Backend
- **Language**: Go 1.23
- **Framework**: Gin (HTTP router)
- **ORM**: GORM
- **Database**: PostgreSQL (CloudSQL in production)
- **Cache**: Redis
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **API Documentation**: Swagger/OpenAPI (swaggo)

### Frontend
- **Framework**: Next.js 14+ (App Router)
- **Language**: TypeScript
- **Styling**: TailwindCSS
- **State Management**: Redux Toolkit
- **Code Editor**: Monaco Editor (@monaco-editor/react)
- **Diff Viewer**: react-diff-viewer-continued

### Infrastructure
- **Containerization**: Docker
- **Orchestration**: GKE (Google Kubernetes Engine)
- **Database**: CloudSQL (PostgreSQL)
- **Cache**: Redis (managed or containerized)

## Architecture Pattern

### Layered Architecture

```
┌─────────────────────────────────────┐
│         Frontend (Next.js)          │
│  - Pages (App Router)               │
│  - Components (UI)                   │
│  - Redux Store                      │
│  - API Client                       │
└──────────────┬──────────────────────┘
                │ REST API
┌───────────────▼──────────────────────┐
│      Backend (Go/Gin)                  │
│  ┌──────────────────────────────┐    │
│  │  Routes Layer                 │    │
│  │  - Route definitions          │    │
│  │  - Middleware                 │    │
│  └───────────┬────────────────────┘    │
│  ┌───────────▼────────────────────┐    │
│  │  Handlers Layer                │    │
│  │  - Request/Response handling  │    │
│  │  - Validation                  │    │
│  └───────────┬────────────────────┘    │
│  ┌───────────▼────────────────────┐    │
│  │  Services Layer                │    │
│  │  - Business logic              │    │
│  │  - External API integration    │    │
│  └───────────┬────────────────────┘    │
│  ┌───────────┬────────────────────┐    │
│  │  Models    │  Cache (Redis)     │    │
│  │  - GORM    │  - Invalidation    │    │
│  └───────────┴────────────────────┘    │
│              │                           │
└──────────────┼───────────────────────────┘
               │
┌──────────────▼───────────────────────────┐
│      PostgreSQL (CloudSQL)                │
│  - Applications                           │
│  - Components                             │
│  - Translations                           │
│  - Users                                  │
└───────────────────────────────────────────┘
```

## Key Design Decisions

### 1. Versioning Strategy
- **Current**: 2 versions per translation (before save, after save)
- **Future-proof**: Database supports unlimited versions
- **Cleanup**: Automatically removes versions > 2

### 2. Deployment Stages
- **Draft**: Work in progress, can revert
- **Staging**: Deployed to staging environment
- **Production**: Live production data

### 3. Caching Strategy
- **Cache Keys**: Structured by resource type and ID
- **TTL**: 1 hour for most resources
- **Invalidation**: On update/delete operations
- **Cache Layer**: Redis (optional, graceful degradation)

### 4. Authentication & Authorization
- **JWT**: Stateless authentication
- **RBAC**: Role-based access control
- **Roles**: super_admin, operator, user_manager
- **Middleware**: Auth + Role checking

### 5. Translation Workflow
1. Admin creates/edits component structure (JSON)
2. Translation saved to Draft stage
3. Can revert to previous version
4. Deploy to Staging
5. Deploy to Production
6. Auto-translation via OpenAI API

### 6. Observability & Monitoring
- **Structured Logging**: Zap logger (always enabled)
- **Metrics**: Datadog StatsD (optional, set `DD_ENABLED=true`)
- **Tracing**: Datadog APM (optional, set `DD_ENABLED=true`)
- **Error Tracking**: Contextual error logging (always enabled)
- **Panic Recovery**: Graceful panic handling (always enabled)
- **Health Checks**: `/health`, `/ready`, `/live` endpoints
- **Cost Optimization**: Sampling rates, tag management, optional Datadog
- **Local Development**: Works perfectly without Datadog (just don't set `DD_ENABLED`)

**Location:** `backend/observability/`, `backend/middleware/observability.go`

## File Structure

```
i18n-center/
├── backend/
│   ├── auth/              # Authentication utilities
│   ├── cache/             # Redis cache layer
│   ├── database/          # DB connection & migrations
│   ├── handlers/          # HTTP handlers
│   ├── middleware/        # Auth, role, observability middleware
│   ├── models/            # GORM models (including AuditLog)
│   ├── observability/     # Logging, metrics, tracing
│   ├── routes/            # Route definitions
│   ├── services/          # Business logic (including AuditService)
│   ├── docs/              # Swagger docs (generated)
│   └── main.go
├── frontend/
│   ├── app/               # Next.js App Router
│   │   ├── (auth)/        # Auth routes
│   │   ├── applications/  # Application pages
│   │   ├── components/    # Component pages
│   │   └── ...
│   ├── components/        # React components
│   │   ├── ui/            # Base UI components
│   │   └── ...
│   ├── store/             # Redux store
│   ├── services/          # API client
│   └── hooks/             # Custom React hooks
└── contexts/              # This documentation
```

## Communication Flow

### Request Flow
1. Frontend makes API call via `services/api.ts`
2. Request includes JWT token in Authorization header
3. Backend middleware validates token
4. Role middleware checks permissions
5. Handler processes request
6. Service layer executes business logic
7. Response returned to frontend

### Error Handling
- **Backend**: Returns structured error JSON
- **Frontend**: Toast notifications for user feedback
- **Status Codes**: Standard HTTP status codes
- **Validation**: GORM binding validation

## Image Serving — PixelShift CDN Pipeline

CMS rich-text content can include images uploaded through the admin UI. Images are stored in GCS
and served via LapakGaming's shared image CDN pipeline. Understanding this pipeline is important
before changing any GCS path or URL construction logic.

### Why not link to GCS directly?

Direct `storage.googleapis.com` URLs would:
- Expose the raw bucket name/path publicly
- Bypass Cloudflare CDN caching (no edge TTL, every request hits GCS)
- Incur full GCS egress costs ($0.12/GB) on every image load
- Miss PixelShift's on-the-fly resize/WebP conversion capability

### The double-CDN pipeline

```
Browser
  → Cloudflare (outer CDN — 7-30 day edge cache)
      ↓ miss
  → PixelShift Cloud Run  (on-the-fly resize / format / quality)
      ↓ fetches source image via
  → HAProxy origin shield  (caches raw source image in memory)
      ↓ miss
  → GCS                    (only on first request for that object)
```

### PixelShift API

PixelShift is a query-parameter proxy, not a path-based CDN:

```
https://img.lapakgaming.com/?src={url-encoded-source-url}&w=720&f=webp&q=75&onerror=redirect
```

The `src` value must be a URL routable through HAProxy (e.g. `https://www.lapakgaming.com/static/cms/...`).
**Never pass a `storage.googleapis.com` URL as `src`** — it bypasses the HAProxy origin shield and
defeats the cost-optimisation design.

### GCS path → public URL mapping

| Layer | Value |
|-------|-------|
| GCS object | `static/cms/{uuid}.jpg` in bucket `lapakgaming-frontend-{env}` |
| HAProxy-fronted URL (src) | `https://www.lapakgaming.com/static/cms/{uuid}.jpg` |
| Stored PixelShift URL | `https://img.lapakgaming.com/?src=https%3A%2F%2Fwww.lapakgaming.com%2Fstatic%2Fcms%2F{uuid}.jpg` |

HAProxy's CDN origin shield routes `/static/*` to the GCS bucket, so the path mapping is
transparent — no extra credentials or bucket config required for reads.

### Transform params

The stored URL contains no transform params. FE code that renders rich-text HTML may append
them at render time (e.g. `&w=720&f=webp&q=75`). Each unique `src + params` combination is
cached separately by Cloudflare.

### Environment variables

| Variable | Purpose | Dev default | Prod default |
|----------|---------|-------------|--------------|
| `GCS_BUCKET` | Target GCS bucket | `lapakgaming-frontend-development` | `lapakgaming-frontend-production` |
| `GCS_CMS_IMAGE_PREFIX` | Object path prefix | `static/cms` | `static/cms` |
| `GCS_CREDENTIALS_BASE64` | Service account JSON (base64) | — | — |
| `CMS_IMAGE_PUBLIC_BASE` | HAProxy base for `src` param | `https://dev.lapakgaming.com` | `https://www.lapakgaming.com` |
| `PIXELSHIFT_BASE_URL` | PixelShift endpoint | `https://dev-img.lapakgaming.com` | `https://img.lapakgaming.com` |

> `dev-img` uses a flat subdomain (`dev-img.lapakgaming.com`) rather than `dev.img.lapakgaming.com`
> because Cloudflare's Business plan does not support subdomain chaining beyond one level.

### Implementation

See `backend/services/gcs_service.go` — the `GCSService` struct and `Upload` method contain
detailed inline documentation explaining each architectural choice.

---

## Scalability Considerations

- **Stateless**: JWT allows horizontal scaling
- **Caching**: Redis reduces DB load
- **Database**: PostgreSQL handles concurrent access
- **Future**: Can split into microservices if needed

## Security

- **Password Hashing**: bcrypt
- **JWT**: Signed tokens with expiration
- **CORS**: Configurable origins
- **Input Validation**: GORM binding + custom validation
- **SQL Injection**: Prevented by GORM parameterized queries
- **XSS**: Frontend sanitization (React default)

