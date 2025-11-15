# i18n-center Setup Guide

Complete setup guide for the i18n-center service.

## Prerequisites

- Go 1.21+
- Node.js 20+
- PostgreSQL 15+ (or CloudSQL)
- Redis 7+
- Docker (for containerization)
- kubectl and gcloud (for GKE deployment)

## Local Development Setup

### 1. Database Setup

Start PostgreSQL and Redis using Docker Compose:

```bash
docker-compose up -d
```

### 2. Backend Setup

```bash
cd backend

# Install dependencies
go mod download

# Copy environment file
cp .env.example .env

# Edit .env with your configuration
# Update database credentials, Redis, JWT secret, etc.

# Initialize admin user
go run scripts/init_admin.go

# Install air for hot reload (optional but recommended)
go install github.com/air-verse/air@latest

# Run the server with hot reload (auto-restarts on code changes)
air

# Or run without hot reload
go run main.go
```

The backend will be available at `http://localhost:8080`

### 3. Frontend Setup

```bash
cd frontend

# Install dependencies
yarn install

# Copy environment file
cp .env.example .env.local

# Edit .env.local
# Set NEXT_PUBLIC_API_URL=http://localhost:8080

# Run development server
yarn dev
```

The frontend will be available at `http://localhost:3000`

## Testing

### Backend Tests

```bash
cd backend
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

Target: 80% coverage

## Deployment to GKE

### 1. Build Docker Images

```bash
# Set your GCP project ID
export PROJECT_ID=your-project-id

# Build images
make docker-build

# Tag and push to GCR
make docker-push PROJECT_ID=$PROJECT_ID
```

### 2. Create Kubernetes Secrets

```bash
kubectl create secret generic i18n-center-secrets \
  --from-literal=db-host=YOUR_CLOUDSQL_HOST \
  --from-literal=db-user=YOUR_DB_USER \
  --from-literal=db-password=YOUR_DB_PASSWORD \
  --from-literal=db-name=i18n_center \
  --from-literal=jwt-secret=YOUR_JWT_SECRET \
  --from-literal=openai-api-key=YOUR_OPENAI_KEY
```

### 3. Update Deployment Files

Edit `k8s/backend-deployment.yaml` and `k8s/frontend-deployment.yaml`:
- Replace `PROJECT_ID` with your GCP project ID
- Update CloudSQL connection details
- Configure Redis service name

### 4. Deploy

```bash
kubectl apply -f k8s/backend-deployment.yaml
kubectl apply -f k8s/frontend-deployment.yaml
```

### 5. Initialize Admin User

After deployment, initialize the admin user:

```bash
kubectl exec -it <backend-pod-name> -- go run scripts/init_admin.go
```

Or create a Kubernetes Job for this.

## Default Admin Credentials

- Username: `admin` (or set via `ADMIN_USERNAME` env var)
- Password: `admin123` (or set via `ADMIN_PASSWORD` env var)

**IMPORTANT**: Change the default password immediately after first login!

## Features Overview

### Applications
- Create and manage applications (e.g., whatsapp, web-app)
- Configure enabled languages per application
- Set OpenAI API key per application for auto-translation

### Components
- Create components within applications (e.g., pdp_form)
- Define JSON structure templates
- Set default locale for new components

### Translations
- Manage translations per component, locale, and deployment stage
- Three stages: draft, staging, production
- Versioning: before save (v1) and after save (v2)
- Revert to previous version
- Deploy from draft → staging → production

### Auto-Translation
- Translate using OpenAI API
- Preserves template values in brackets (e.g., `[last_name]`)
- Backfill multiple languages at once

### Export/Import
- Export translations per application, component, or locale
- Import translations from JSON files

### User Management
- Three roles:
  - **Super Admin**: Full access
  - **Operator**: Manage i18n data
  - **User Manager**: Manage users and privileges

## API Documentation

See `backend/README.md` for detailed API endpoint documentation.

## Troubleshooting

### Database Connection Issues
- Verify CloudSQL connection string
- Check firewall rules allow connections
- Ensure SSL mode is correctly configured

### Redis Connection Issues
- Verify Redis service is running
- Check Redis host and port configuration
- Backend will continue without cache if Redis is unavailable

### OpenAI Translation Issues
- Verify API key is set correctly
- Check API quota and limits
- Ensure template values are preserved (values in brackets)

## Support

For issues or questions, please refer to the project documentation or contact the development team.

