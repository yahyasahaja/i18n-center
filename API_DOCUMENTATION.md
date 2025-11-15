# API Documentation

The i18n Center API is fully documented using Swagger/OpenAPI.

## Accessing the Documentation

### Swagger UI (Interactive)
Visit: **http://localhost:8080/api/docs/index.html**

(Also available at: http://localhost:8080/swagger/index.html)

This provides an interactive interface where you can:
- Browse all API endpoints
- See request/response schemas
- Test endpoints directly from the browser
- View authentication requirements

### OpenAPI Specifications

- **JSON**: http://localhost:8080/api/docs/doc.json
- **YAML**: http://localhost:8080/api/docs/doc.yaml

(Also available at: http://localhost:8080/swagger/doc.json)

These can be used to:
- Generate client SDKs
- Import into Postman
- Import into other API tools
- Generate API documentation sites

## Generating Postman Collection

### Option 1: From Swagger UI
1. Open http://localhost:8080/api/docs/index.html
2. Click "Download" button
3. Select "Postman Collection" format

### Option 2: Using Script
```bash
cd backend
./scripts/generate_postman.sh
```

This will generate `i18n-center-api.postman_collection.json` that you can import into Postman.

### Option 3: Manual Import
1. Copy the OpenAPI JSON from http://localhost:8080/api/docs/doc.json
2. Open Postman
3. Click "Import" → "Raw text"
4. Paste the JSON
5. Postman will automatically convert it to a collection

## API Endpoints Overview

### Authentication
- `POST /api/auth/login` - Login and get JWT token

### Applications
- `GET /api/applications` - List all applications
- `GET /api/applications/:id` - Get application details
- `POST /api/applications` - Create application
- `PUT /api/applications/:id` - Update application
- `DELETE /api/applications/:id` - Delete application

### Components
- `GET /api/components` - List components (filter by application_id)
- `GET /api/components/:id` - Get component details
- `POST /api/components` - Create component
- `PUT /api/components/:id` - Update component
- `DELETE /api/components/:id` - Delete component

### Translations
- `GET /api/components/:id/translations` - Get translation
- `POST /api/components/:id/translations` - Save translation
- `POST /api/components/:id/translations/revert` - Revert to previous version
- `POST /api/components/:id/translations/deploy` - Deploy to stage
- `POST /api/components/:id/translations/auto-translate` - Auto-translate
- `POST /api/components/:id/translations/backfill` - Backfill all locales
- `GET /api/components/:id/translations/compare` - Compare versions

### Export/Import
- `GET /api/applications/:id/export` - Export application translations
- `GET /api/components/:id/export` - Export component translations
- `POST /api/components/:id/import` - Import translations

## Authentication

All endpoints (except `/api/auth/login`) require JWT authentication.

**Header Format:**
```
Authorization: Bearer <your-jwt-token>
```

**Getting a Token:**
1. POST to `/api/auth/login` with username and password
2. Response includes a `token` field
3. Use this token in the Authorization header for subsequent requests

## Role-Based Access Control

- **Super Admin**: Full access to all endpoints
- **Operator**: Can manage applications, components, and translations
- **User Manager**: Can manage users and view applications/components

## Updating Documentation

After adding new endpoints or modifying existing ones:

1. Add Swagger annotations to handler functions
2. Run: `swag init --parseDependency --parseInternal`
3. Restart the backend server
4. Documentation will be automatically updated

## Example Swagger Annotation

```go
// @Summary      Get translation
// @Description  Get translation data for a component
// @Tags         translations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string  true   "Component ID"
// @Param        locale   query     string  false  "Locale"
// @Success      200      {object}  models.TranslationVersion
// @Failure      400      {object}  map[string]string
// @Router       /components/{id}/translations [get]
func (h *TranslationHandler) GetTranslation(c *gin.Context) {
    // Handler implementation
}
```

