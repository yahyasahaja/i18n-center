# i18n-center

Centralized i18n management service for managing translations across multiple applications.

## Architecture

- **Backend**: Go application with RESTful API
- **Frontend**: Next.js admin backoffice
- **Database**: CloudSQL (PostgreSQL)
- **Cache**: Redis
- **Deployment**: GKE

## Features

- Structured JSON translation files per component
- Multi-language support with auto-translation via OpenAI
- Deployment stages: draft, staging, production
- Versioning system (before/after save)
- Export/Import functionality
- Role-based access control (Super Admin, Operator, User Manager)
- Template value support (preserves values in brackets)
- **SDKs**: JavaScript/TypeScript SDK for Next.js and Go SDK for backend services
- **Headless CMS** with templates, versioned localizations, and draft→staging→production workflow
- **AI translation for CMS content** (async jobs, field-type-aware)
- **Image upload to GCS** served via PixelShift CDN (optional)

## Project Structure

```
.
├── backend/          # Go backend service
├── frontend/         # Next.js admin UI
│   └── app/
│       └── cms/      # CMS pages (templates, items, localization editor)
├── i18ncenter-js/    # JavaScript/TypeScript SDK for Next.js apps
├── i18ncenter-go/    # Go SDK for backend services
├── e2e/              # End-to-end tests (Playwright)
├── docker-compose.yml
└── README.md
```

## Getting Started

### Backend

```bash
cd backend
go mod download

# Install air for hot reload (optional but recommended)
go install github.com/air-verse/air@latest

# Run with hot reload (auto-restarts on code changes)
air

# Or run without hot reload
go run main.go
```

### Frontend

```bash
cd frontend
yarn install
yarn dev
```

## Environment Variables

See `.env.example` files in backend and frontend directories.

## API Documentation

The API is fully documented with Swagger/OpenAPI:

- **Swagger UI**: http://localhost:8080/api/docs/index.html
- **OpenAPI JSON**: http://localhost:8080/api/docs/doc.json
- **OpenAPI YAML**: http://localhost:8080/api/docs/doc.yaml

You can:
- Browse and test endpoints in Swagger UI
- Export to Postman collection
- Generate client SDKs from OpenAPI spec

See [API_DOCUMENTATION.md](./API_DOCUMENTATION.md) for details.

## Documentation

- **[SETUP.md](./SETUP.md)** — Local setup, testing, deployment.
- **[API_DOCUMENTATION.md](./API_DOCUMENTATION.md)** — API overview and Swagger.
- **[contexts/](./contexts/README.md)** — Live docs: architecture, API design, DB schema, auth, translation system, frontend, deployment, patterns, troubleshooting. Update these when making significant changes.

