# Authentication & Authorization

## Overview

JWT-based stateless authentication with role-based access control (RBAC).

## Authentication Flow

### 1. Login

```http
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "password"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "...",
    "username": "admin",
    "role": "super_admin",
    "is_active": true
  }
}
```

### 2. Using Token

Include token in Authorization header for all protected endpoints:

```http
GET /api/applications
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## JWT Implementation

### Token Structure

```go
type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}
```

**Token Payload:**
- `user_id`: UUID of user
- `username`: Username
- `role`: User role (super_admin, operator, user_manager)
- `exp`: Expiration time (24 hours)
- `iat`: Issued at time

### Token Generation

**Location:** `backend/auth/auth.go`

```go
func GenerateToken(userID uuid.UUID, username string, role string) (string, error) {
    claims := Claims{
        UserID:   userID.String(),
        Username: username,
        Role:     role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(jwtSecret))
}
```

**Secret:** Stored in environment variable `JWT_SECRET`

### Token Validation

**Location:** `backend/middleware/auth.go`

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token from header
        // Validate token
        // Set user context
        c.Set("user_id", claims.UserID)
        c.Set("username", claims.Username)
        c.Set("role", claims.Role)
    }
}
```

## Password Security

### Hashing

**Algorithm:** bcrypt

**Location:** `backend/auth/auth.go`

```go
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

**Cost:** Default (10 rounds)

### Password Requirements

Currently no validation, but can be added:
- Minimum length: 8 characters
- Complexity requirements
- Password history

## Role-Based Access Control (RBAC)

### Roles

1. **super_admin**
   - Full access to all endpoints
   - Can delete applications
   - Can manage users

2. **operator**
   - Can manage applications
   - Can manage components
   - Can manage translations
   - Cannot delete applications
   - Cannot manage users

3. **user_manager**
   - Can manage users
   - Can view applications/components
   - Cannot modify i18n data

### Role Middleware

**Location:** `backend/middleware/auth.go`

```go
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRole, exists := c.Get("role")
        if !exists {
            c.JSON(401, gin.H{"error": "Unauthorized"})
            c.Abort()
            return
        }

        for _, role := range allowedRoles {
            if userRole == role {
                c.Next()
                return
            }
        }

        c.JSON(403, gin.H{"error": "Insufficient permissions"})
        c.Abort()
    }
}
```

### Usage in Routes

```go
api.GET("/applications", appHandler.GetApplications,
    middleware.RequireRole("super_admin", "operator"))

api.DELETE("/applications/:id", appHandler.DeleteApplication,
    middleware.RequireRole("super_admin"))
```

## Frontend Authentication

### Token Storage

**Location:** `frontend/services/api.ts`

- Stored in `localStorage` as `token`
- Included in all API requests
- Removed on logout

### Auth State Management

**Location:** `frontend/store/slices/authSlice.ts`

**State:**
- `token`: JWT token
- `user`: Current user object
- `isAuthenticated`: Boolean flag

**Actions:**
- `login`: Store token and user
- `logout`: Clear token and user
- `getCurrentUser`: Fetch user from `/api/auth/me`

### Protected Routes

**Implementation:** Client-side checks in page components

```typescript
useEffect(() => {
  const token = localStorage.getItem('token')
  if (!token) {
    router.replace('/login')
    return
  }
  // Load data...
}, [])
```

## User Management

### Creating Users

**Endpoint:** `POST /api/auth/users`

**Required Role:** `super_admin` or `user_manager`

**Request:**
```json
{
  "username": "newuser",
  "password": "password123",
  "role": "operator"
}
```

### Updating Users

**Endpoint:** `PUT /api/auth/users/:id`

**Fields:**
- `is_active`: Boolean (activate/deactivate)
- `role`: UserRole (change role)
- `password`: String (change password)

### Listing Users

**Endpoint:** `GET /api/auth/users`

**Response:** Array of users (password hashes excluded)

## Security Best Practices

### Implemented
- ✅ Password hashing (bcrypt)
- ✅ JWT with expiration
- ✅ Role-based access control
- ✅ Secure password comparison
- ✅ Token in Authorization header

### Recommendations for Production
- [ ] HTTPS only
- [ ] Rate limiting on login endpoint
- [ ] Account lockout after failed attempts
- [ ] Password complexity requirements
- [ ] Token refresh mechanism
- [ ] Audit logging
- [ ] 2FA support (future)

## Environment Variables

```env
JWT_SECRET=your-secret-key-here  # Use strong random secret
```

**Generate Secret:**
```bash
openssl rand -base64 32
```

## Initial Admin User

**Script:** `backend/scripts/init_admin.go`

Creates initial admin user:
- Username: `admin`
- Password: Set via environment or default
- Role: `super_admin`

**Run:**
```bash
go run scripts/init_admin.go
```

