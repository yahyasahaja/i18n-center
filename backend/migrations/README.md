# Database Migrations

Schema changes for i18n-center, applied via [goose](https://github.com/pressly/goose).

## TL;DR

```bash
# In production / staging: exec into a running pod
kubectl exec -it deploy/i18n-center-backend -- i18n-center migrate up

# In dev: run from the backend directory
go run ./cmd/migrate up
```

## Conventions

- **One file per logical change.** Numbered `00001_*.sql`, `00002_*.sql`, …
- **SQL only.** No Go-based migrations. The binary embeds the `.sql` files via `embed.FS`.
- **Both `Up` and `Down`** sections, even if `Down` is `-- intentional no-op`. Forces you to think about rollback.
- **`00001_init.sql` is the canonical schema bootstrap** — squashed from the original GORM `AutoMigrate` + bespoke `migrateCodeFields` / `ensureSearchIndexes` / `ensurePerformanceIndexes` work. It is intended to be run ONCE against a fresh database. Do not edit it — write a new migration instead.

## Commands

| Command | What it does |
|---|---|
| `i18n-center migrate up` | Apply all pending migrations |
| `i18n-center migrate up-by-one` | Apply just the next pending migration |
| `i18n-center migrate down` | Roll back the most recent migration |
| `i18n-center migrate status` | Show what's been applied |
| `i18n-center migrate create <name> sql` | Scaffold a new migration file (dev convenience) |
| `i18n-center migrate version` | Print the current migration version |

The binary never auto-migrates on boot. If a service pod starts before migrations have been applied, the first query will fail with `relation "..." does not exist` — exec in and run `migrate up`.

## Postgres safe-pattern playbook

The shared Cloud SQL instance carries Hydra's auth load. Lock-heavy statements
ripple into B2C login latency. **Default to lock-light patterns:**

### ✅ Safe (metadata-only or short locks)

```sql
-- New column with no default (PG 11+: even with a constant default)
ALTER TABLE components ADD COLUMN new_flag BOOLEAN;
ALTER TABLE components ADD COLUMN new_count INTEGER DEFAULT 0;

-- New index without blocking writes
CREATE INDEX CONCURRENTLY idx_components_new_flag
    ON components (new_flag)
    WHERE deleted_at IS NULL;

-- Drop column (metadata-only)
ALTER TABLE components DROP COLUMN old_flag;

-- Add CHECK constraint without table scan
ALTER TABLE components ADD CONSTRAINT chk_name_len CHECK (length(name) > 0) NOT VALID;
ALTER TABLE components VALIDATE CONSTRAINT chk_name_len;
```

### ⚠️ Requires care

```sql
-- Adding NOT NULL on existing column: expand-contract, not in-place
-- Step 1 (new migration):
ALTER TABLE components ADD CONSTRAINT chk_name_not_null CHECK (name IS NOT NULL) NOT VALID;
-- Backfill any NULLs in a separate migration or one-off
UPDATE components SET name = '' WHERE name IS NULL;
ALTER TABLE components VALIDATE CONSTRAINT chk_name_not_null;
-- Step 2 (later migration, once you're sure):
ALTER TABLE components ALTER COLUMN name SET NOT NULL;
ALTER TABLE components DROP CONSTRAINT chk_name_not_null;

-- Adding foreign key
ALTER TABLE translate_jobs ADD CONSTRAINT fk_translate_jobs_app
    FOREIGN KEY (application_id) REFERENCES applications(id) NOT VALID;
ALTER TABLE translate_jobs VALIDATE CONSTRAINT fk_translate_jobs_app;
```

### 🚫 Avoid in production (acquires ACCESS EXCLUSIVE)

```sql
-- Plain CREATE INDEX (blocks writes for duration). Use CONCURRENTLY.
CREATE INDEX idx_foo ON big_table (foo);

-- ALTER COLUMN TYPE (full table rewrite)
ALTER TABLE components ALTER COLUMN code TYPE VARCHAR(255);
-- Instead: add a new column, dual-write, swap usage, drop the old.

-- Renaming a column you actually use. Even though the ALTER is fast,
-- old pods will 500 on the old name during rolling deploys. Use the
-- expand-contract dance:
--   1. Add new_name as nullable, backfill, dual-write in code
--   2. Cut reads over to new_name
--   3. Drop old_name in a later migration
```

## Goose syntax cheat-sheet

```sql
-- +goose Up
-- +goose StatementBegin
-- (your forward statements; multiple SQL statements OK inside Begin/End)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- (your rollback statements)
-- +goose StatementEnd
```

If a statement needs to run outside a transaction (e.g. `CREATE INDEX CONCURRENTLY`), use the no-transaction marker at the top of the file:

```sql
-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY idx_foo ON big_table (foo);
```

## Workflow for a new schema change

1. **Scaffold**: `go run ./cmd/migrate create add_components_archived_flag sql`
   This writes `migrations/00002_add_components_archived_flag.sql` with empty Up/Down blocks.
2. **Author the SQL** using the safe-pattern playbook above. Include both Up AND Down.
3. **Test against a dev DB**: `go run ./cmd/migrate up` then `go run ./cmd/migrate status`.
4. **Verify rollback works**: `go run ./cmd/migrate down` then re-apply `up`.
5. **Commit the file alongside the Go code** that depends on the new schema. The Go change ships in the same PR; the migration runs in the K8s pod just before the deploy.
6. **In production**: `kubectl exec -it deploy/i18n-center-backend -- i18n-center migrate up`.
   - Best practice: scale down replicas to 1 first, run migrations, then scale back up. Eliminates write contention during the migration window.
   - If the migration is `CONCURRENTLY` or otherwise lock-light, you can keep replicas running.

## Shared Postgres with Hydra

This database is shared with the LapakGaming Hydra OAuth2 service (in the B2C login hot path). Implications:

- Never `DROP DATABASE` or `DROP SCHEMA` in a migration.
- Never `CREATE EXTENSION` for something Hydra also uses without coordinating.
- Long-running migrations on i18n-center tables won't directly affect Hydra tables, but they can saturate the shared connection pool. Coordinate with on-call before a heavy migration.
- The default `search_path` is `public` — both services live in the same schema. A future migration may move i18n-center to its own schema (`i18n_center.*`) for blast-radius isolation; until then, table names must not collide with Hydra's (they don't today — Hydra uses `hydra_oauth2_*`).
