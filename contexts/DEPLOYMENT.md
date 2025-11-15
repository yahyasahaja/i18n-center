# Deployment Guide

## Overview

Deployment to Google Kubernetes Engine (GKE) with CloudSQL PostgreSQL and Redis.

## Prerequisites

- Google Cloud Platform account
- `gcloud` CLI installed
- `kubectl` installed
- Docker installed
- Access to GKE cluster

## Environment Setup

### Backend Environment Variables

```env
# Database
DB_HOST=cloudsql-proxy
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your-password
DB_NAME=i18n_center
DB_SSLMODE=require

# Redis
REDIS_HOST=redis-service
REDIS_PORT=6379
REDIS_PASSWORD=your-redis-password

# JWT
JWT_SECRET=your-jwt-secret

# Server
PORT=8080
GIN_MODE=release

# CORS
CORS_ORIGIN=https://your-frontend-domain.com

# OpenAI (optional)
OPENAI_API_KEY=sk-...
```

### Frontend Environment Variables

```env
NEXT_PUBLIC_API_URL=https://api.your-domain.com/api
```

## Database Setup

### CloudSQL PostgreSQL

1. **Create CloudSQL Instance:**
```bash
gcloud sql instances create i18n-center-db \
  --database-version=POSTGRES_15 \
  --tier=db-f1-micro \
  --region=us-central1
```

2. **Create Database:**
```bash
gcloud sql databases create i18n_center \
  --instance=i18n-center-db
```

3. **Create User:**
```bash
gcloud sql users create i18n_user \
  --instance=i18n-center-db \
  --password=your-password
```

4. **Enable CloudSQL Proxy:**
- Use CloudSQL Proxy sidecar in Kubernetes
- Or use Private IP with VPC connector

## Redis Setup

### Option 1: Managed Redis (Memorystore)

```bash
gcloud redis instances create i18n-center-redis \
  --size=1 \
  --region=us-central1 \
  --redis-version=redis_6_x
```

### Option 2: Redis in Kubernetes

Deploy Redis as a StatefulSet in the cluster.

## Docker Images

### Build Backend Image

```bash
cd backend
docker build -t gcr.io/PROJECT_ID/i18n-center-backend:latest .
docker push gcr.io/PROJECT_ID/i18n-center-backend:latest
```

### Build Frontend Image

```bash
cd frontend
docker build -t gcr.io/PROJECT_ID/i18n-center-frontend:latest .
docker push gcr.io/PROJECT_ID/i18n-center-frontend:latest
```

## Kubernetes Deployment

### Backend Deployment

**File:** `k8s/backend-deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: i18n-center-backend
spec:
  replicas: 3
  selector:
    matchLabels:
      app: i18n-center-backend
  template:
    metadata:
      labels:
        app: i18n-center-backend
    spec:
      containers:
      - name: backend
        image: gcr.io/PROJECT_ID/i18n-center-backend:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          value: "127.0.0.1"
        - name: DB_PORT
          value: "5432"
        # ... other env vars
        volumeMounts:
        - name: cloudsql
          mountPath: /cloudsql
      - name: cloudsql-proxy
        image: gcr.io/cloudsql-docker/gce-proxy:1.33.2
        command:
        - "/cloud_sql_proxy"
        - "-instances=PROJECT_ID:REGION:INSTANCE_NAME=tcp:5432"
        volumeMounts:
        - name: cloudsql
          mountPath: /cloudsql
      volumes:
      - name: cloudsql
        emptyDir: {}
```

### Frontend Deployment

**File:** `k8s/frontend-deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: i18n-center-frontend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: i18n-center-frontend
  template:
    metadata:
      labels:
        app: i18n-center-frontend
    spec:
      containers:
      - name: frontend
        image: gcr.io/PROJECT_ID/i18n-center-frontend:latest
        ports:
        - containerPort: 3000
        env:
        - name: NEXT_PUBLIC_API_URL
          value: "https://api.your-domain.com/api"
```

### Services

**Backend Service:**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: i18n-center-backend
spec:
  selector:
    app: i18n-center-backend
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

**Frontend Service:**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: i18n-center-frontend
spec:
  selector:
    app: i18n-center-frontend
  ports:
  - port: 80
    targetPort: 3000
  type: LoadBalancer
```

### Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: i18n-center-ingress
  annotations:
    kubernetes.io/ingress.class: "gce"
    kubernetes.io/ingress.global-static-ip-name: "i18n-center-ip"
spec:
  rules:
  - host: api.your-domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: i18n-center-backend
            port:
              number: 80
  - host: your-domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: i18n-center-frontend
            port:
              number: 80
```

## Secrets Management

### Create Secrets

```bash
kubectl create secret generic i18n-center-secrets \
  --from-literal=jwt-secret=your-secret \
  --from-literal=db-password=your-password \
  --from-literal=redis-password=your-redis-password
```

### Use in Deployment

```yaml
env:
- name: JWT_SECRET
  valueFrom:
    secretKeyRef:
      name: i18n-center-secrets
      key: jwt-secret
```

## Database Migrations

### Initial Setup

1. Connect to CloudSQL:
```bash
gcloud sql connect i18n-center-db --user=i18n_user
```

2. Run migrations (auto-migrated on backend startup, or manual):
```bash
# Backend auto-migrates on startup
# Or run manual migrations if needed
```

3. Initialize admin user:
```bash
# Run init script or use API
```

## Monitoring & Logging

### Cloud Logging

Logs automatically sent to Cloud Logging:
```bash
gcloud logging read "resource.type=k8s_container" --limit 50
```

### Health Checks

Add health check endpoint:
```go
r.GET("/health", func(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok"})
})
```

## Scaling

### Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: i18n-center-backend-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: i18n-center-backend
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Backup & Recovery

### Database Backup

```bash
# Automated backups (CloudSQL default)
# Manual backup:
gcloud sql backups create --instance=i18n-center-db

# Restore:
gcloud sql backups restore BACKUP_ID --instance=i18n-center-db
```

### Application Backup

- Export translations regularly
- Backup configuration
- Version control for code

## Rollback Procedure

1. **Rollback Deployment:**
```bash
kubectl rollout undo deployment/i18n-center-backend
```

2. **Rollback Database:**
```bash
# Restore from backup if needed
```

## CI/CD Pipeline

### GitHub Actions Example

```yaml
name: Deploy
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - name: Build and push
      run: |
        docker build -t gcr.io/$PROJECT_ID/i18n-center-backend:$GITHUB_SHA ./backend
        docker push gcr.io/$PROJECT_ID/i18n-center-backend:$GITHUB_SHA
    - name: Deploy to GKE
      run: |
        kubectl set image deployment/i18n-center-backend \
          backend=gcr.io/$PROJECT_ID/i18n-center-backend:$GITHUB_SHA
```

## Security Checklist

- [ ] Use HTTPS (Ingress with SSL)
- [ ] Secure JWT secret
- [ ] Use CloudSQL Private IP
- [ ] Enable network policies
- [ ] Use service accounts with minimal permissions
- [ ] Enable audit logging
- [ ] Regular security updates
- [ ] Secrets in Kubernetes Secrets (not env vars)
- [ ] Enable CloudSQL SSL
- [ ] Redis authentication enabled

## Troubleshooting

### Common Issues

1. **Database Connection:**
   - Check CloudSQL Proxy
   - Verify network connectivity
   - Verify credentials

2. **High Memory Usage:**
   - Check Redis connection
   - Review cache TTL
   - Check for memory leaks

3. **Slow Performance:**
   - Check database indexes
   - Review query performance
   - Check Redis cache hit rate

## Local Development with Docker

See `docker-compose.yml` for local PostgreSQL and Redis setup.

