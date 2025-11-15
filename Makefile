.PHONY: help build-backend build-frontend test-backend docker-build docker-push deploy

help:
	@echo "Available targets:"
	@echo "  build-backend    - Build Go backend"
	@echo "  build-frontend   - Build Next.js frontend"
	@echo "  test-backend     - Run backend tests"
	@echo "  docker-build     - Build Docker images"
	@echo "  docker-push      - Push Docker images to GCR"
	@echo "  deploy           - Deploy to GKE"

build-backend:
	cd backend && go build -o bin/main ./main.go

build-frontend:
	cd frontend && yarn install && yarn build

test-backend:
	cd backend && go test -v -coverprofile=coverage.out ./...
	cd backend && go tool cover -html=coverage.out -o coverage.html

docker-build:
	docker build -t i18n-center-backend:latest ./backend
	docker build -t i18n-center-frontend:latest ./frontend

docker-push:
	@echo "Set PROJECT_ID environment variable"
	docker tag i18n-center-backend:latest gcr.io/$(PROJECT_ID)/i18n-center-backend:latest
	docker tag i18n-center-frontend:latest gcr.io/$(PROJECT_ID)/i18n-center-frontend:latest
	docker push gcr.io/$(PROJECT_ID)/i18n-center-backend:latest
	docker push gcr.io/$(PROJECT_ID)/i18n-center-frontend:latest

deploy:
	kubectl apply -f k8s/backend-deployment.yaml
	kubectl apply -f k8s/frontend-deployment.yaml

