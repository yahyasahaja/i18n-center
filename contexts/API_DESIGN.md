# API Design Patterns

## Base URL

- **Local**: `http://localhost:8080/api`
- **Production**: Configured via environment

## Authentication

All endpoints (except `/api/auth/login`) require JWT authentication.

### Header Format
```
Authorization: Bearer <jwt-token>
```

### Getting a Token
```http
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "password"
}
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": { ... }
}
```

## API Conventions

### HTTP Methods
- `GET`: Retrieve resources
- `POST`: Create resources or actions
- `PUT`: Update resources (full update)
- `DELETE`: Delete resources

### URL Patterns
- **Resources**: `/api/{resource}` (plural)
- **Single Resource**: `/api/{resource}/:id`
- **Nested Resources**: `/api/{resource}/:id/{subresource}`
- **Actions**: `/api/{resource}/:id/{action}`

### Response Format

#### Success Response
```json
{
  "id": "...",
  "name": "...",
  ...
}
```

#### Error Response
```json
{
  "error": "Error message here"
}
```

### Status Codes
- `200 OK`: Success
- `201 Created`: Resource created
- `400 Bad Request`: Validation error
- `401 Unauthorized`: Missing/invalid token
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

## Endpoint Structure

### Authentication
```
POST   /api/auth/login              # Public
GET    /api/auth/me                 # Protected
GET    /api/auth/users               # super_admin, user_manager
POST   /api/auth/users               # super_admin, user_manager
PUT    /api/auth/users/:id           # super_admin, user_manager
```

### Applications
```
GET    /api/applications            # List all
GET    /api/applications/:id        # Get one
POST   /api/applications            # Create
PUT    /api/applications/:id        # Update
DELETE /api/applications/:id        # Delete (super_admin only)
```

### Components
```
GET    /api/components              # List (filter: ?application_id=...)
GET    /api/components/:id          # Get one
POST   /api/components               # Create
PUT    /api/components/:id          # Update
DELETE /api/components/:id          # Delete
```

### Translations
```
GET    /api/components/:id/translations              # Get translation
POST   /api/components/:id/translations              # Save translation
POST   /api/components/:id/translations/revert      # Revert to previous
POST   /api/components/:id/translations/deploy      # Deploy to stage
POST   /api/components/:id/translations/auto-translate # Translate one locale
POST   /api/components/:id/translations/backfill    # Backfill all locales
GET    /api/components/:id/translations/compare      # Compare versions
```

### Export/Import
```
GET    /api/applications/:id/export  # Export application
GET    /api/components/:id/export    # Export component
POST   /api/components/:id/import     # Import component
```

## Query Parameters

### Common Parameters
- `locale`: Language code (e.g., "en", "id", "es")
- `stage`: Deployment stage ("draft", "staging", "production")
- `application_id`: Filter components by application

### Examples
```
GET /api/components?application_id=123e4567-e89b-12d3-a456-426614174000
GET /api/components/:id/translations?locale=en&stage=production
```

## Request/Response Examples

### Create Application
```http
POST /api/applications
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "MyApp",
  "description": "My Application",
  "enabled_languages": ["en", "id", "es"],
  "openai_key": "sk-..."
}
```

### Save Translation
```http
POST /api/components/:id/translations
Authorization: Bearer <token>
Content-Type: application/json

{
  "locale": "en",
  "stage": "draft",
  "data": {
    "form": {
      "name": "Name",
      "email": "Email"
    }
  }
}
```

### Backfill Translations
```http
POST /api/components/:id/translations/backfill
Authorization: Bearer <token>
Content-Type: application/json

{
  "source_locale": "en",
  "target_locales": ["id", "es"],
  "stage": "draft"
}
```

## Error Handling Patterns

### Validation Errors
```json
{
  "error": "Validation failed: name is required"
}
```

### Not Found
```json
{
  "error": "Application not found"
}
```

### Permission Denied
```json
{
  "error": "Insufficient permissions"
}
```

## Caching Strategy

### Cache Keys
- Applications: `application:{id}`
- Components: `component:{id}`
- Translations: Not cached (frequently updated)

### Cache Invalidation
- On update: Delete cache key
- On delete: Delete cache key
- TTL: 1 hour (3600 seconds)

## Rate Limiting

Currently not implemented, but can be added via middleware.

## API Versioning

Currently v1 (no version prefix). Future versions can use:
- `/api/v1/...`
- `/api/v2/...`

## Swagger Documentation

All endpoints are documented in Swagger:
- UI: `http://localhost:8080/swagger/index.html`
- JSON: `http://localhost:8080/swagger/doc.json`
- YAML: `http://localhost:8080/swagger/doc.yaml`

