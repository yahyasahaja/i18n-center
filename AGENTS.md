# Instructions for AI Agents

When working in this repository (e.g. Cursor agent, Copilot, or other AI tools), use the following so changes are **correct and consistent** with the project.

## 1. Use project docs as context

Before implementing features or refactors, **refer to these files**:

| Purpose | File |
|--------|------|
| **Setup, run, test** | [SETUP.md](./SETUP.md) |
| **API contracts, auth, Swagger** | [API_DOCUMENTATION.md](./API_DOCUMENTATION.md) |
| **Code style, Git, adding features** | [contexts/DEVELOPMENT_GUIDELINES.md](./contexts/DEVELOPMENT_GUIDELINES.md) |
| **Architecture overview** | [README.md](./README.md) |

### The `contexts/` folder — use it

**contexts/** is the project’s live documentation. Use it for deep context:

- **[contexts/README.md](./contexts/README.md)** — Index of all context docs.
- **Architecture, API design, DB, auth, translation flow, frontend, deployment**: ARCHITECTURE.md, API_DESIGN.md, DATABASE_SCHEMA.md, AUTHENTICATION.md, TRANSLATION_SYSTEM.md, FRONTEND_ARCHITECTURE.md, DEPLOYMENT.md.
- **Patterns and troubleshooting**: COMMON_PATTERNS.md, TROUBLESHOOTING.md.

When you add features or change design, read the relevant context file and **update contexts/ when you make significant changes** (see contexts/README.md).

## 2. Project layout

- **backend/** — Go API (Gin, GORM, Redis). Handlers, services, models, routes, cache.
- **frontend/** — Next.js admin UI (App Router, Redux, Tailwind).
- **i18ncenter-go/** — Go SDK for translations API.
- **i18ncenter-js/** — JS/TS SDK for translations API.

## 3. Conventions to follow

- **New API endpoints**: Add Swagger annotations; consider Redis cache for reads; respect RBAC and application API key scope.
- **Navigation**: Preserve `application_id` and `stage` (use `useAppContext().push()` or `LinkWithContext`).
- **Code review**: Follow the workspace Code Review Standard (small PRs, EXPLAIN for new SQL, cache invalidation, error observability).

## 4. Cursor-specific rules

If using Cursor, project rules in **`.cursor/rules/`** provide extra context:

- **project-context.mdc** — Always applied; points to the docs above.
- **backend-go.mdc** — When editing `backend/**/*.go`.
- **frontend-react.mdc** — When editing `frontend/**/*.ts(x)`.

When in doubt, open the relevant doc and implement accordingly.
