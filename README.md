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

## Project Structure

```
.
├── backend/          # Go backend service
├── frontend/         # Next.js admin UI
├── i18ncenter-js/    # JavaScript/TypeScript SDK for Next.js apps
├── i18ncenter-go/    # Go SDK for backend services
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

