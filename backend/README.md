# i18n-center Backend

Go backend service for centralized i18n management.

## Table of Contents

- [Setup](#setup)
- [Architecture](#architecture)
- [Database Design](#database-design)
- [Request Flow](#request-flow)
- [Caching Strategy](#caching-strategy)
- [API Endpoints](#api-endpoints)
- [Testing](#testing)
- [Deployment](#deployment)

## Setup

### Prerequisites

- Go 1.23+
- PostgreSQL 12+
- Redis (optional, but recommended)

### Installation

1. **Install dependencies:**
```bash
go mod download
```

2. **Set up environment variables:**
```bash
cp .env.example .env
# Edit .env with your configuration
```

3. **Start PostgreSQL and Redis:**
```bash
# Using Docker Compose (recommended for local development)
docker-compose up -d postgres redis
```

4. **Run the application:**
```bash
# Database migrations run automatically on startup
go run main.go

# Or with hot reload (recommended)
go install github.com/air-verse/air@latest
air
```

5. **Initialize admin user:**
```bash
go run scripts/init_admin.go
```

### Environment Variables

See `.env.example` for all required variables:

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=i18n_center
DB_SSLMODE=disable

# Redis (optional)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT
JWT_SECRET=your-secret-key-here

# Server
PORT=8080
GIN_MODE=debug

# CORS
CORS_ORIGIN=*
```

## Architecture

### System Overview

The backend follows a **layered architecture** pattern:

```
┌─────────────────────────────────────┐
│         HTTP Request                 │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│      Routes Layer                    │
│  - Route definitions                 │
│  - Middleware (Auth, CORS, Roles)   │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│      Handlers Layer                  │
│  - Request validation                │
│  - Response formatting               │
│  - Error handling                    │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│      Services Layer                  │
│  - Business logic                    │
│  - External API integration          │
│  - Cache management                  │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│      Data Layer                      │
│  - GORM Models                       │
│  - Redis Cache                       │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│      PostgreSQL Database             │
└─────────────────────────────────────┘
```

### Directory Structure

```
backend/
├── auth/              # Authentication utilities (JWT, password hashing)
├── cache/             # Redis cache layer
├── database/          # Database connection & migrations
├── handlers/          # HTTP request handlers
├── middleware/        # Auth & role-based middleware
├── models/            # GORM models (database schema)
├── routes/            # Route definitions
├── services/          # Business logic services
├── docs/             # Swagger documentation (generated)
├── scripts/           # Utility scripts (init_admin, etc.)
└── main.go            # Application entry point
```

### Key Components

1. **Routes** (`routes/routes.go`)
   - Defines all API endpoints
   - Applies middleware (auth, CORS, role checking)
   - Groups related endpoints

2. **Handlers** (`handlers/`)
   - Process HTTP requests
   - Validate input
   - Call services
   - Format responses

3. **Services** (`services/`)
   - Business logic
   - Database operations
   - Cache management
   - External API calls (OpenAI)

4. **Models** (`models/models.go`)
   - GORM models
   - Database schema definition
   - Relationships

5. **Middleware** (`middleware/`)
   - JWT authentication
   - Role-based access control
   - CORS handling

## Database Design

### Entity Relationship Diagram

```
┌─────────────────┐
│   Application   │
│─────────────────│
│ id (PK)         │
│ name (unique)   │
│ description     │
│ enabled_langs[] │
│ openai_key      │
└────────┬────────┘
         │ 1
         │
         │ N
┌────────▼────────┐
│   Component     │
│─────────────────│
│ id (PK)         │
│ application_id  │
│ name            │
│ structure (JSON)│
│ default_locale  │
└────────┬────────┘
         │ 1
         │
         │ N
┌────────▼──────────────┐
│ TranslationVersion    │
│───────────────────────│
│ id (PK)               │
│ component_id (FK)     │
│ locale                │
│ stage                 │
│ version (1 or 2)      │
│ data (JSONB)          │
└───────────────────────┘

┌─────────────────┐
│      User       │
│─────────────────│
│ id (PK)         │
│ username        │
│ password_hash   │
│ role            │
│ is_active       │
└─────────────────┘
```

### Tables

#### 1. Applications

Stores application configurations.

| Column | Type | Description |
|--------|------|-------------|
| id | uuid | Primary key |
| name | varchar | Unique application name |
| description | text | Application description |
| enabled_languages | text[] | Array of supported locales |
| openai_key | text | OpenAI API key (encrypted) |
| created_at | timestamp | Creation timestamp |
| updated_at | timestamp | Last update timestamp |
| deleted_at | timestamp | Soft delete timestamp |

**Indexes:**
- `name` (unique)

#### 2. Components

Stores UI component definitions.

| Column | Type | Description |
|--------|------|-------------|
| id | uuid | Primary key |
| application_id | uuid | Foreign key to applications |
| name | varchar | Component name |
| description | text | Component description |
| structure | jsonb | JSON structure template |
| default_locale | varchar | Default language |
| created_at | timestamp | Creation timestamp |
| updated_at | timestamp | Last update timestamp |
| deleted_at | timestamp | Soft delete timestamp |

**Indexes:**
- `application_id` (foreign key)
- `(application_id, name)` (composite)

#### 3. TranslationVersions

Stores translation data with versioning.

| Column | Type | Description |
|--------|------|-------------|
| id | uuid | Primary key |
| component_id | uuid | Foreign key to components |
| locale | varchar | Language code (en, id, es, etc.) |
| stage | varchar | Deployment stage (draft, staging, production) |
| version | int | Version number (1 or 2) |
| data | jsonb | Translation JSON data |
| is_active | boolean | Active flag |
| created_at | timestamp | Creation timestamp |
| updated_at | timestamp | Last update timestamp |

**Indexes:**
- `component_id`
- `locale`
- `stage`
- `(component_id, locale, stage, version)` (composite)

**Versioning:**
- Version 1: Before save (can revert to)
- Version 2: After save (current version)
- Versions > 2: Automatically cleaned up

#### 4. Users

Stores user accounts.

| Column | Type | Description |
|--------|------|-------------|
| id | uuid | Primary key |
| username | varchar | Unique username |
| password_hash | varchar | Bcrypt hashed password |
| role | varchar | User role (super_admin, operator, user_manager) |
| is_active | boolean | Account active status |
| created_at | timestamp | Creation timestamp |
| updated_at | timestamp | Last update timestamp |

**Indexes:**
- `username` (unique)

### Database Migrations

**Automatic Migrations:** Database schema is automatically created/updated on application startup using GORM's `AutoMigrate()`.

**Location:** `database/database.go`

```go
func InitDatabase() error {
    // ... connection setup ...

    // Auto-migrate tables
    err = DB.AutoMigrate(
        &models.User{},
        &models.Application{},
        &models.Component{},
        &models.TranslationVersion{},
    )

    // ...
}
```

**What AutoMigrate does:**
- Creates tables if they don't exist
- Adds new columns if models are updated
- Creates indexes
- **Does NOT** modify existing columns or delete columns

**For new developers:**
1. Start PostgreSQL: `docker-compose up -d postgres`
2. Run application: `go run main.go`
3. Tables are automatically created!

**Note:** For production schema changes, manual migrations may be needed.

## Request Flow

### Authentication Flow

```
1. Client → POST /api/auth/login
   └─> AuthHandler.Login()
       ├─> Validate credentials
       ├─> Check password hash
       └─> Generate JWT token
           └─> Return token + user info

2. Client → Any protected endpoint
   └─> AuthMiddleware()
       ├─> Extract token from header
       ├─> Validate JWT
       ├─> Set user context
       └─> Continue to handler
```

### Translation Retrieval Flow

```
1. Client → GET /api/components/:id/translations?locale=en&stage=production
   └─> TranslationHandler.GetTranslation()
       └─> TranslationService.GetTranslation()
           ├─> Check Redis cache
           │   └─> Cache hit? Return cached data
           ├─> Query database (if cache miss)
           │   ├─> Try version 2
           │   └─> Fallback to version 1
           ├─> Cache result (1 hour TTL)
           └─> Return translation data
```

### Translation Save Flow

```
1. Client → POST /api/components/:id/translations
   └─> TranslationHandler.SaveTranslation()
       └─> TranslationService.SaveTranslation()
           ├─> Get existing version 2 (if exists)
           ├─> Create/update version 2 with new data
           ├─> Ensure version 1 exists (backup)
           ├─> Invalidate cache
           ├─> Cleanup old versions (> 2)
           └─> Return saved translation
```

### Bulk Translation Flow (Aggregator)

```
1. Client → GET /api/translations/bulk?component_ids=id1,id2,id3
   └─> TranslationHandler.GetMultipleTranslations()
       └─> TranslationService.GetMultipleTranslations()
           ├─> First pass: Check Redis cache for each component
           │   └─> Collect cache hits
           ├─> Second pass: Query DB for missing translations
           │   ├─> Single query for all missing (efficient!)
           │   ├─> Try version 2, fallback to version 1
           │   └─> Cache newly fetched translations
           └─> Return aggregated map
```

## Caching Strategy

### Cache Keys

| Resource | Key Format | TTL |
|----------|-----------|-----|
| Application | `application:{id}` | 1 hour |
| Component | `component:{id}` | 1 hour |
| Translation | `translation:{componentID}:{locale}:{stage}` | 1 hour |

### Cache Operations

**Read:**
1. Check cache first
2. If miss, query database
3. Store in cache for future use

**Write:**
1. Update database
2. Invalidate cache (delete key)
3. Next read will fetch fresh data and cache it

**Invalidation:**
- On update: Delete specific cache key
- On delete: Delete cache key
- On save translation: Delete translation cache + component cache

### Cache Implementation

**Location:** `cache/cache.go`

```go
// Get from cache
cacheKey := cache.TranslationKey(componentID, locale, stage)
var cached TranslationVersion
if err := cache.Get(cacheKey, &cached); err == nil {
    return &cached, nil
}

// Cache miss - get from DB and cache
translation := getFromDatabase()
cache.Set(cacheKey, translation, 3600*1000000000) // 1 hour
```

**Graceful Degradation:**
- If Redis is unavailable, application continues without cache
- All operations fall back to database queries

## API Endpoints

### Authentication
- `POST /api/auth/login` - Login and get JWT token
- `GET /api/auth/me` - Get current user
- `GET /api/auth/users` - List users (Super Admin, User Manager)
- `POST /api/auth/users` - Create user (Super Admin, User Manager)
- `PUT /api/auth/users/:id` - Update user (Super Admin, User Manager)

### Applications
- `GET /api/applications` - List applications
- `GET /api/applications/:id` - Get application
- `POST /api/applications` - Create application
- `PUT /api/applications/:id` - Update application
- `DELETE /api/applications/:id` - Delete application (Super Admin only)

### Components
- `GET /api/components` - List components (filter: `?application_id=...`)
- `GET /api/components/:id` - Get component
- `POST /api/components` - Create component
- `PUT /api/components/:id` - Update component
- `DELETE /api/components/:id` - Delete component

### Translations
- `GET /api/components/:id/translations` - Get translation
- `GET /api/translations/bulk` - Get multiple translations (aggregator)
- `POST /api/components/:id/translations` - Save translation
- `POST /api/components/:id/translations/revert` - Revert to previous version
- `POST /api/components/:id/translations/deploy` - Deploy to stage
- `POST /api/components/:id/translations/auto-translate` - Auto-translate one locale
- `POST /api/components/:id/translations/backfill` - Backfill all locales
- `GET /api/components/:id/translations/compare` - Compare versions

### Export/Import
- `GET /api/applications/:id/export` - Export application translations
- `GET /api/components/:id/export` - Export component translations
- `POST /api/components/:id/import` - Import translations

### API Documentation

- **Swagger UI**: http://localhost:8080/api/docs/index.html
- **OpenAPI JSON**: http://localhost:8080/api/docs/doc.json
- **OpenAPI YAML**: http://localhost:8080/api/docs/doc.yaml

## Testing

### Run Tests

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run specific package
go test -v ./handlers
```

### Test Coverage Target

**Target:** 80% coverage

**Current coverage:**
- Auth: ✅
- Cache: ✅
- Services: ✅
- Handlers: Partial

### Writing Tests

**Example test structure:**

```go
func TestHandler_Create(t *testing.T) {
    // Setup
    // Execute
    // Assert
}
```

## Deployment

### Local Development

```bash
# Start dependencies
docker-compose up -d postgres redis

# Run with hot reload
air

# Or run directly
go run main.go
```

### Production (GKE)

See `../contexts/DEPLOYMENT.md` for detailed deployment guide.

**Key points:**
- Database: CloudSQL PostgreSQL
- Cache: Redis (Memorystore or containerized)
- Container: Docker image
- Orchestration: Kubernetes

### Environment Variables (Production)

```env
DB_HOST=cloudsql-proxy
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=<secure-password>
DB_NAME=i18n_center
DB_SSLMODE=require

REDIS_HOST=redis-service
REDIS_PORT=6379
REDIS_PASSWORD=<secure-password>

JWT_SECRET=<strong-random-secret>
GIN_MODE=release
PORT=8080
```

## Development Guidelines

### Code Style

- Use `gofmt` or `goimports`
- Follow standard Go conventions
- Document exported functions
- Write tests for new features

### Adding New Features

1. **Create/Update Model** (`models/models.go`)
2. **Create Service** (`services/`)
3. **Create Handler** (`handlers/`)
4. **Add Route** (`routes/routes.go`)
5. **Add Swagger Annotations**
6. **Write Tests**
7. **Update Documentation**

### Database Changes

1. Update model in `models/models.go`
2. Run application (AutoMigrate will update schema)
3. Test thoroughly
4. For production: Create manual migration if needed

## Troubleshooting

### Database Connection Issues

- Check PostgreSQL is running: `docker ps | grep postgres`
- Verify connection string in `.env`
- Check database exists: `psql -l`

### Redis Connection Issues

- Application continues without Redis (graceful degradation)
- Check Redis is running: `docker ps | grep redis`
- Verify connection in `.env`

### Migration Issues

- AutoMigrate only adds, doesn't modify/delete columns
- For schema changes: Manual migration may be needed
- Reset database (dev only): `docker-compose down -v && docker-compose up -d`

## Additional Resources

- [API Documentation](../API_DOCUMENTATION.md)
- [Architecture Details](../contexts/ARCHITECTURE.md)
- [Database Schema](../contexts/DATABASE_SCHEMA.md)
- [Development Guidelines](../contexts/DEVELOPMENT_GUIDELINES.md)
