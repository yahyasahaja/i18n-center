# Common Patterns & Code Snippets

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

