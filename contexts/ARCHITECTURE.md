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

