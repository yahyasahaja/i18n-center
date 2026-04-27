.PHONY: help build-backend build-frontend test-backend docker-build docker-push deploy \
        dev run run-deps run-backend run-frontend stop logs \
        e2e e2e-install e2e-api e2e-ui e2e-report

help:
	@echo "Available targets:"
	@echo "  dev              - Start everything (DB, Redis, backend, frontend) — main dev command"
	@echo "  stop             - Stop all Docker containers (Postgres, Redis)"
	@echo "  logs             - Tail logs of all Docker containers"
	@echo "  run              - Start deps (Postgres, Redis) and run backend locally"
	@echo "  run-deps         - Start Postgres and Redis via docker-compose"
	@echo "  run-backend      - Run Go backend (requires run-deps or existing DB/Redis)"
	@echo "  run-frontend     - Run Next.js frontend (yarn dev)"
	@echo "  build-backend    - Build Go backend"
	@echo "  build-frontend   - Build Next.js frontend"
	@echo "  test-backend     - Run backend tests"
	@echo "  e2e-install      - Install Playwright + Chromium (one-time)"
	@echo "  e2e              - Run full E2E test suite (API + UI)"
	@echo "  e2e-api          - Run API tests only (no browser)"
	@echo "  e2e-ui           - Run UI/browser tests only"
	@echo "  e2e-report       - Open Playwright HTML report"
	@echo "  docker-build     - Build Docker images"
	@echo "  docker-push      - Push Docker images to GCR"
	@echo "  deploy           - Deploy to GKE"

# ── Main dev command ─────────────────────────────────────────────────────────
# Starts Postgres + Redis, waits until healthy, then runs backend & frontend
# concurrently. Ctrl-C cleanly kills all three.

dev: run-deps wait-deps
	@echo "==> Starting backend and frontend..."
	@trap 'echo "\n==> Shutting down..."; kill 0' INT; \
	  (cd backend && go run main.go 2>&1 | sed "s/^/[backend] /") & \
	  (cd frontend && yarn install --frozen-lockfile --silent 2>/dev/null; yarn dev 2>&1 | sed "s/^/[frontend] /") & \
	  wait

wait-deps:
	@echo "==> Waiting for Postgres to be ready..."
	@until docker exec i18n-center-postgres pg_isready -U i18n_user -q 2>/dev/null; do \
	  printf '.'; sleep 1; \
	done; echo " ready."
	@echo "==> Waiting for Redis to be ready..."
	@until docker exec i18n-center-redis redis-cli ping 2>/dev/null | grep -q PONG; do \
	  printf '.'; sleep 1; \
	done; echo " ready."

stop:
	docker-compose down

logs:
	docker-compose logs -f

# ── Individual targets ────────────────────────────────────────────────────────

build-backend:
	cd backend && go build -o bin/main ./main.go

build-frontend:
	cd frontend && yarn install && yarn build

run-deps:
	docker-compose up -d

run-backend: run-deps
	cd backend && go run main.go

run-frontend:
	cd frontend && yarn install && yarn dev

run: run-backend

test-backend:
	cd backend && go test -v -coverprofile=coverage.out ./...
	cd backend && go tool cover -html=coverage.out -o coverage.html

# ── E2E Tests (Playwright) ────────────────────────────────────────────────────

e2e-install:
	cd e2e && npm install
	cd e2e && npx playwright install chromium

e2e:
	cd e2e && npx playwright test

e2e-api:
	cd e2e && npx playwright test tests/0[1-5]-*.spec.ts

e2e-ui:
	cd e2e && npx playwright test tests/06-ui.spec.ts

e2e-report:
	cd e2e && npx playwright show-report

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

