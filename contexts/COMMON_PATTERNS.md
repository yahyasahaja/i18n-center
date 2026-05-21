# Common Patterns & Code Snippets

## Repository pattern (raw SQL via sqlx)

The data access layer is being moved off GORM onto raw SQL. New work follows this pattern; old GORM call sites are being converted in Commits D–I.

### File layout per resource

```
repository/<resource>/
    repository.go         # Repository interface + domain types + sentinel errors
    repository_impl.go    # All queries as const at top; struct + methods
    repository_impl_test.go
```

### Skeleton

```go
// repository/component/repository.go
package component

import (
    "context"

    "github.com/google/uuid"
    "github.com/your-org/i18n-center/repository"
)

type Component struct {
    ID            uuid.UUID         `db:"id"`
    ApplicationID uuid.UUID         `db:"application_id"`
    Name          string            `db:"name"`
    Code          string            `db:"code"`
    Description   string            `db:"description"`
    KeyContexts   repository.JSONB  `db:"key_contexts"`
    DefaultLocale string            `db:"default_locale"`
    CreatedBy     uuid.UUID         `db:"created_by"`
    UpdatedBy     uuid.UUID         `db:"updated_by"`
    CreatedAt     time.Time         `db:"created_at"`
    UpdatedAt     time.Time         `db:"updated_at"`
}

type Repository interface {
    GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Component, error)
    ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID, limit, offset int) ([]Component, int, error)
    Create(ctx context.Context, q repository.Queryer, c *Component) error
    Update(ctx context.Context, q repository.Queryer, c *Component) error
    SoftDelete(ctx context.Context, q repository.Queryer, id, userID uuid.UUID) error
}
```

```go
// repository/component/repository_impl.go
package component

import (
    "context"
    "database/sql"
    "errors"

    "github.com/google/uuid"
    "github.com/your-org/i18n-center/repository"
)

// ── Queries (top of file, const, fully-formed) ──────────────────────────────
const (
    queryGetByID = `
        SELECT id, application_id, name, code, description,
               key_contexts, default_locale, created_by, updated_by,
               created_at, updated_at
        FROM components
        WHERE id = $1
          AND deleted_at IS NULL
    `

    queryListByApp = `
        SELECT id, application_id, name, code, description,
               key_contexts, default_locale, created_by, updated_by,
               created_at, updated_at
        FROM components
        WHERE application_id = $1
          AND deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `

    queryCountByApp = `
        SELECT COUNT(*) FROM components
        WHERE application_id = $1 AND deleted_at IS NULL
    `

    queryInsert = `
        INSERT INTO components (
            id, application_id, name, code, description,
            key_contexts, default_locale, created_by, updated_by,
            created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
    `

    queryUpdate = `
        UPDATE components
        SET name = $2, description = $3, key_contexts = $4,
            default_locale = $5, updated_by = $6, updated_at = NOW()
        WHERE id = $1 AND deleted_at IS NULL
    `

    querySoftDelete = `
        UPDATE components
        SET deleted_at = NOW(), updated_by = $2, updated_at = NOW()
        WHERE id = $1 AND deleted_at IS NULL
    `
)

// ── Implementation ──────────────────────────────────────────────────────────
type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Component, error) {
    var c Component
    if err := q.GetContext(ctx, &c, queryGetByID, id); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, repository.ErrNotFound
        }
        return nil, err
    }
    return &c, nil
}

// ... etc
```

### Conditional queries (search / filter)

Static parts as const; conditional parts appended in the function with PG numbered placeholders:

```go
const queryListBase = `
    SELECT id, application_id, name, code, description,
           key_contexts, default_locale, created_by, updated_by,
           created_at, updated_at
    FROM components
    WHERE deleted_at IS NULL
`

func (r *Impl) Search(ctx context.Context, q repository.Queryer, f SearchFilter) ([]Component, error) {
    sb := strings.Builder{}
    sb.WriteString(queryListBase)
    args := []any{}
    i := 1
    if f.ApplicationID != uuid.Nil {
        fmt.Fprintf(&sb, " AND application_id = $%d", i)
        args = append(args, f.ApplicationID)
        i++
    }
    if f.SearchTerm != "" {
        fmt.Fprintf(&sb, " AND (name ILIKE $%d OR code ILIKE $%d)", i, i+1)
        like := "%" + f.SearchTerm + "%"
        args = append(args, like, like)
        i += 2
    }
    fmt.Fprintf(&sb, " ORDER BY created_at DESC LIMIT $%d OFFSET $%d", i, i+1)
    args = append(args, f.Limit, f.Offset)

    var out []Component
    return out, q.SelectContext(ctx, &out, sb.String(), args...)
}
```

### IN clause expansion (variable-length lists)

```go
const queryFindByCodes = `
    SELECT id, code, ...
    FROM components
    WHERE application_id = ? AND code IN (?) AND deleted_at IS NULL
`

func (r *Impl) FindByCodes(ctx context.Context, q repository.Queryer, appID uuid.UUID, codes []string) ([]Component, error) {
    expanded, args, err := sqlx.In(queryFindByCodes, appID, codes)
    if err != nil { return nil, err }
    expanded = q.Rebind(expanded)   // ? → $1, $2, ... for Postgres

    var out []Component
    return out, q.SelectContext(ctx, &out, expanded, args...)
}
```

### Transactions

```go
// In a usecase / handler
err := repository.WithTx(ctx, database.SQLX, func(tx repository.Queryer) error {
    if err := componentRepo.Update(ctx, tx, c); err != nil { return err }
    if err := tagRepo.AttachToComponent(ctx, tx, c.ID, tagIDs); err != nil { return err }
    return nil
})
```

Both `*sqlx.DB` (autocommit) and `*sqlx.Tx` satisfy `repository.Queryer`, so the same repository methods participate in either context.

---

## Critical patterns (USE THESE — do not re-implement)

### Versioned-insert with race retry (translation_versions, cms_localizations)

Every write to a versioned table needs to compute `next_version = MAX(version) + 1` and insert atomically. Done naively, two concurrent writers compute the same number and silently create duplicate rows.

**Always use the helper. Never write `MAX(version)+1` directly.**

```go
// CMS — services.SaveCmsLocalizationVersion
loc, err := services.SaveCmsLocalizationVersion(
    nil,                          // or *gorm.DB if inside an outer transaction
    cmsItemID,
    locale,
    stage,
    data,
    sourceLocale,                 // "" if manual edit (no AI source)
    userID,
)

// Translation — TranslationService.saveVersion / saveVersionTx
v, err := translationService.SaveTranslation(componentID, locale, stage, data, userID)
```

Both retry up to 5 times against the partial unique index (`idx_cms_loc_unique_version`, `idx_tv_unique_version`). The retry loop re-reads `MAX(version)` and inserts again — bounded retry budget is deliberate because high collision count means application-level pathology (one component being hammered), not normal load.

### Detect unique-key violation

```go
import "github.com/your-org/i18n-center/services"

if err := tx.Create(&row).Error; err != nil {
    if services.IsUniqueViolation(err) {
        // expected race — re-read and retry, or return existing row
    }
    return err
}
```

Message-based; no dependency on `jackc/pgconn`. Matches `SQLSTATE 23505`, `duplicate key value`, or `unique constraint`.

### Cache invalidation after a translation write

```go
// Per-write invalidation (saveVersion already calls this for you on non-tx path)
services.InvalidateAfterTranslationWrite(componentID, locale, stage)

// App-wide invalidation (locale deleted, bulk deploy)
services.InvalidateApplicationReadCache(applicationID)
```

`InvalidateAfterTranslationWrite` busts:
- `translation:{componentID}:{locale}:{stage}`
- `component:{componentID}`
- `translations:bypage:{appID}:*:{locale}:{stage}` (scoped — draft writes don't touch production cache)
- `translations:bytag:{appID}:*:{locale}:{stage}`

If you add a NEW write path for translations, call this helper. Without it, FE2 (next-intl) will serve stale strings for up to 1 hour after a production deploy.

### Translate-job idempotency

Lookup-then-insert, with a second lookup on the unique-violation race:

```go
// In the handler:
if existing := findActiveTranslateJob(componentID, sourceLocale, targetLocale, jobType); existing != nil {
    c.JSON(http.StatusAccepted, gin.H{
        "job_id":  existing.ID.String(),
        "status":  existing.Status,
        "deduped": true,
    })
    return
}

job := models.TranslateJob{ /* ... */ }
if err := database.DB.Create(&job).Error; err != nil {
    // DB unique constraint may have caught a race — try the lookup again
    if existing := findActiveTranslateJob(...); existing != nil {
        // return existing
    }
    return // ... error
}
```

Equivalent helper for CMS: `findActiveCmsTranslateJob`. Both back the partial unique indexes `idx_translate_jobs_dedupe` / `idx_cms_translate_jobs_dedupe`.

### Application API-key scoping on public endpoints

```go
applicationID, err := uuid.Parse(c.Param("id"))
// ... 400 on parse failure ...
if apiKeyAppID := middleware.GetAPIKeyApplicationID(c); apiKeyAppID != uuid.Nil && apiKeyAppID != applicationID {
    c.JSON(http.StatusForbidden, gin.H{"error": "API key does not have access to this application"})
    return
}
```

JWT-authenticated requests return `uuid.Nil` from `GetAPIKeyApplicationID`, so the check passes for operators. API-key requests MUST be scope-checked, otherwise an API key issued for app A can fetch app B's data. Apply on every public read endpoint.

### CMS identifier case-folding

```go
// On both create AND read paths
identifier := normalizeIdentifier(input)   // lowercase + trim
```

The SDK lowercases identifiers before sending. If the server doesn't normalize on create, an item created as `Flash_Banner` will 404 on SDK calls.

### Public cache headers for Cloudflare

```go
setPublicCacheHeaders(c, string(stage), 300)
c.JSON(http.StatusOK, response)
```

Production stage → `public, max-age=60, s-maxage=300, stale-while-revalidate=600` + `Vary: X-API-Key, Authorization, Accept-Encoding`. Draft/staging → `private, no-store`. Apply on every PUBLIC read endpoint (translation by-page/by-tag/bulk, CMS by-identifier).

### Singleton background tasks across K8s replicas (advisory lock)

When a background goroutine must run on **exactly one pod** (e.g. retention sweep, audit archive), use a Postgres advisory lock:

```go
var got bool
database.DB.WithContext(ctx).Raw(
    "SELECT pg_try_advisory_lock(?)", yourFixedKey,
).Scan(&got)
if !got { return }    // another pod holds it; skip this tick
defer database.DB.WithContext(ctx).Exec(
    "SELECT pg_advisory_unlock(?)", yourFixedKey,
)
// ... do the singleton work ...
```

Use a fixed int64 per task (declare next to the function). Auto-release on session close means a crashed leader doesn't block the next tick. Currently used by `jobs.RunCleanupTicker` (key `0x6931386e63746e6d`).

---

## Backend Patterns

### Handler with Service Pattern

```go
type Handler struct {
    service ServiceInterface
}

func (h *Handler) Create(c *gin.Context) {
    var req Request
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    result, err := h.service.Create(req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, result)
}
```

### Cache Pattern

```go
// Get from cache
cacheKey := cache.ResourceKey(id)
var cached Resource
if err := cache.Get(cacheKey, &cached); err == nil {
    return cached, nil
}

// Get from database
var resource Resource
if err := database.DB.First(&resource, "id = ?", id).Error; err != nil {
    return nil, err
}

// Set cache
cache.Set(cacheKey, resource, 3600*1000000000) // 1 hour
return resource, nil
```

### Pagination Pattern

```go
type PaginationParams struct {
    Page  int `form:"page" binding:"min=1"`
    Limit int `form:"limit" binding:"min=1,max=100"`
}

func (h *Handler) List(c *gin.Context) {
    var params PaginationParams
    if err := c.ShouldBindQuery(&params); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    offset := (params.Page - 1) * params.Limit
    var items []Item
    var total int64

    database.DB.Model(&Item{}).Count(&total)
    database.DB.Offset(offset).Limit(params.Limit).Find(&items)

    c.JSON(http.StatusOK, gin.H{
        "data": items,
        "total": total,
        "page": params.Page,
        "limit": params.Limit,
    })
}
```

### Error Handling Pattern

```go
func (s *Service) DoSomething() error {
    if err := s.validate(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    if err := s.process(); err != nil {
        return fmt.Errorf("processing failed: %w", err)
    }

    return nil
}
```

### Swagger Annotation Pattern

```go
// @Summary      Get resource
// @Description  Get resource by ID
// @Tags         resources
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Resource ID"
// @Success      200  {object}  Resource
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /resources/{id} [get]
func (h *Handler) Get(c *gin.Context) {
    // ...
}
```

## Frontend Patterns

### API Call with Loading State

```typescript
const [loading, setLoading] = useState(false)
const [error, setError] = useState<string | null>(null)

const handleSubmit = async (data: FormData) => {
  setLoading(true)
  setError(null)
  try {
    const response = await api.create(data)
    toast.success('Created successfully')
    // Handle success
  } catch (err: any) {
    setError(err.response?.data?.error || 'Failed to create')
    toast.error(error)
  } finally {
    setLoading(false)
  }
}
```

### Redux Async Thunk Pattern

```typescript
export const fetchItems = createAsyncThunk(
  'items/fetch',
  async (params: FetchParams) => {
    const response = await api.getItems(params)
    return response.data
  }
)

const itemsSlice = createSlice({
  name: 'items',
  initialState: { items: [], loading: false, error: null },
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchItems.pending, (state) => {
        state.loading = true
        state.error = null
      })
      .addCase(fetchItems.fulfilled, (state, action) => {
        state.items = action.payload
        state.loading = false
      })
      .addCase(fetchItems.rejected, (state, action) => {
        state.loading = false
        state.error = action.error.message || 'Failed to fetch'
      })
  }
})
```

### Protected Route Pattern

```typescript
export default function ProtectedPage() {
  const router = useRouter()
  const { user, isAuthenticated } = useAppSelector((state) => state.auth)

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      router.replace('/login')
      return
    }

    if (!isAuthenticated) {
      // Wait for auth to initialize
      return
    }

    // Check role if needed
    if (user && user.role !== 'super_admin') {
      router.replace('/dashboard')
      return
    }

    // Load data
    loadData()
  }, [router, isAuthenticated, user])

  // ...
}
```

### Form Handling Pattern

```typescript
const [formData, setFormData] = useState<FormData>({
  name: '',
  email: '',
})

const [errors, setErrors] = useState<Record<string, string>>({})

const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
  const { name, value } = e.target
  setFormData((prev) => ({ ...prev, [name]: value }))
  // Clear error for this field
  if (errors[name]) {
    setErrors((prev) => ({ ...prev, [name]: '' }))
  }
}

const handleSubmit = async (e: React.FormEvent) => {
  e.preventDefault()

  // Validate
  const newErrors: Record<string, string> = {}
  if (!formData.name) newErrors.name = 'Name is required'
  if (!formData.email) newErrors.email = 'Email is required'

  if (Object.keys(newErrors).length > 0) {
    setErrors(newErrors)
    return
  }

  // Submit
  await handleSave(formData)
}
```

### Modal Pattern

```typescript
const [isOpen, setIsOpen] = useState(false)

const handleOpen = () => setIsOpen(true)
const handleClose = () => setIsOpen(false)

return (
  <>
    <Button onClick={handleOpen}>Open Modal</Button>
    <Modal isOpen={isOpen} onClose={handleClose} title="Modal Title">
      {/* Modal content */}
    </Modal>
  </>
)
```

### Unsaved Changes Warning

```typescript
const [originalData, setOriginalData] = useState('')
const [currentData, setCurrentData] = useState('')

const hasUnsavedChanges = () => {
  return currentData !== originalData
}

const handleNavigation = () => {
  if (hasUnsavedChanges()) {
    if (!window.confirm('You have unsaved changes. Continue?')) {
      return
    }
  }
  // Navigate
}

// Update original after save
const handleSave = async () => {
  await api.save(currentData)
  setOriginalData(currentData)
}
```

## Database Patterns

### Soft Delete Query

```go
// GORM automatically excludes soft-deleted records
var items []Item
database.DB.Find(&items) // Only active items

// Include deleted
database.DB.Unscoped().Find(&items)

// Only deleted
database.DB.Unscoped().Where("deleted_at IS NOT NULL").Find(&items)
```

### Transaction Pattern

```go
tx := database.DB.Begin()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
    }
}()

if err := tx.Create(&item1).Error; err != nil {
    tx.Rollback()
    return err
}

if err := tx.Create(&item2).Error; err != nil {
    tx.Rollback()
    return err
}

return tx.Commit().Error
```

### JSONB Query Pattern

```go
// Query JSONB field
database.DB.Where("data->>'key' = ?", "value").Find(&items)

// Update JSONB field
database.DB.Model(&item).Update("data", gorm.Expr("jsonb_set(data, '{key}', ?)", value))
```

## Utility Patterns

### UUID Generation

```go
import "github.com/google/uuid"

id := uuid.New()
idString := uuid.New().String()
```

### Password Hashing

```go
import "golang.org/x/crypto/bcrypt"

hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
```

### String Array (PostgreSQL)

```go
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
    // Convert to PostgreSQL array format
}

func (a *StringArray) Scan(value interface{}) error {
    // Parse PostgreSQL array format
}
```

## Testing Patterns

### Handler Test

```go
func TestHandler_Create(t *testing.T) {
    // Setup
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest("POST", "/", bytes.NewBuffer(jsonData))
    c.Request.Header.Set("Content-Type", "application/json")

    // Execute
    handler.Create(c)

    // Assert
    assert.Equal(t, http.StatusCreated, w.Code)
    // Check response body
}
```

### Service Test

```go
func TestService_Create(t *testing.T) {
    // Setup mock
    mockDB := setupTestDB(t)
    service := NewService(mockDB)

    // Execute
    result, err := service.Create(testData)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

## Environment Configuration

### Backend Config

```go
type Config struct {
    DBHost     string
    DBPort     string
    DBUser     string
    DBPassword string
    DBName     string
    JWTSecret  string
}

func LoadConfig() Config {
    return Config{
        DBHost:     getEnv("DB_HOST", "localhost"),
        DBPort:     getEnv("DB_PORT", "5432"),
        // ...
    }
}
```

### Frontend Config

```typescript
const config = {
  apiUrl: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api',
  // ...
}
```

## Error Messages

### Standard Error Messages

```go
const (
    ErrNotFound      = "Resource not found"
    ErrUnauthorized  = "Unauthorized"
    ErrForbidden     = "Insufficient permissions"
    ErrValidation    = "Validation failed"
    ErrInternal      = "Internal server error"
)
```

### Frontend Error Handling

```typescript
const getErrorMessage = (error: any): string => {
  if (error.response?.data?.error) {
    return error.response.data.error
  }
  if (error.message) {
    return error.message
  }
  return 'An unexpected error occurred'
}
```

