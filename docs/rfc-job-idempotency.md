# RFC: Job Idempotency & Stuck-Job Recovery for Async Translation Jobs

**Status:** Implemented  
**Date:** 2026-04-26  
**Scope:** `AddLanguageJob`, `TranslateJob`

---

## Problem

The "Add Language" feature triggers an async OpenAI translation job that can run for
several minutes (one API call per component × N components). Without idempotency
controls, two failure modes exist:

1. **Double submission** — An operator clicks "Add language" twice, or a network retry
   causes the HTTP request to be sent twice. Two jobs are created for the same
   `(application_id, locale)`. Both workers pick up their job and write conflicting
   `translation_versions` rows, wasting OpenAI quota and producing a race condition
   on which version survives.

2. **Stuck running** — A worker pod crashes mid-job (OOM kill, node eviction, SIGKILL
   with no time to drain). The job remains in `status = 'running'` forever and no other
   worker picks it up because the claim query filters for `status = 'pending'`.

---

## Design

### 1. API-level idempotency (AddLanguage handler)

`POST /api/applications/:id/languages` with `auto_translate: true` now checks for an
existing job before creating a new one:

```sql
SELECT id, status FROM add_language_jobs
WHERE application_id = $1
  AND locale         = $2
  AND status         IN ('pending', 'running')
  AND deleted_at     IS NULL
LIMIT 1;
```

If a row is found → **HTTP 409 Conflict** with the existing `job_id` and `status_url`
so the caller can poll that job instead of creating a duplicate.

The frontend handles 409 by transparently attaching to the existing job and showing
progress as if it had created the job itself.

```
POST /api/applications/:id/languages  →  409
{
  "error":      "A translation job for this locale is already in progress",
  "job_id":     "<uuid>",
  "status":     "running",
  "status_url": "/api/applications/:id/jobs/<uuid>"
}
```

### 2. Worker-level claim locking (TranslateJob & AddLanguageJob)

The worker uses PostgreSQL `SELECT ... FOR UPDATE SKIP LOCKED` to atomically claim a
job row. At most one worker instance (across all K8s replicas) can hold the lock on a
given row at a time. This is the second line of defence against double-processing
**after** the API check.

```sql
UPDATE add_language_jobs
SET status = 'running', claimed_by = $pod_hostname, updated_at = NOW()
WHERE id = (
    SELECT id FROM add_language_jobs
    WHERE status = 'pending'
    ORDER BY created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id;
```

### 3. Stuck-job recovery (heartbeat reset)

Every worker poll cycle (every 5 seconds), before claiming a new job, the worker
resets any job that has been in `running` state for more than **15 minutes** back to
`pending`:

```sql
UPDATE add_language_jobs
SET status = 'pending', claimed_by = '', updated_at = NOW()
WHERE status = 'running'
  AND updated_at < NOW() - INTERVAL '15 minutes';
```

The 15-minute threshold is intentionally conservative: even with 34 components and
slow OpenAI responses (≈10 s each with concurrency=5), a full run takes under 2
minutes. A job stuck beyond 15 minutes has definitively lost its worker.

**Why not shorter?** The worker updates `updated_at` at the start of the job but does
**not** heartbeat during processing. Increasing `componentConcurrency` or very slow
OpenAI responses could push wall-clock time above a shorter threshold and cause
spurious resets. If a heartbeat column is added in the future, the threshold can be
tightened to ~2 minutes.

### 4. Graceful shutdown cleanup

On `SIGTERM` / `SIGINT`, the worker's context is cancelled. Each in-flight goroutine
checks `ctx.Err()` before calling OpenAI and exits early. The job orchestrator then:

1. Drains all goroutine results from the buffered channel.
2. Calls `DeleteTranslationVersionByID` for every `translation_version` row that was
   already written (full rollback).
3. Marks the job `status = 'failed'` with `error_message = "Worker cancelled"`.

This means a graceful restart (K8s rolling update) never leaves partial translations
in draft. The job can be retried from scratch.

---

## Failure mode matrix

| Scenario | Outcome |
|---|---|
| Operator submits Add Language twice (fast) | Second request → 409; frontend attaches to first job |
| Operator submits Add Language twice (slow, first job completed) | Second request creates a new job normally |
| Worker pod OOM-killed mid-job | Job reset to `pending` after 15 min; picked up by another pod |
| Worker pod graceful shutdown (SIGTERM) | Job marked `failed`; partial translations rolled back |
| OpenAI rate-limit error on one component | Whole job marked `failed`; all created versions rolled back |
| Two worker pods running (K8s scale-up) | `FOR UPDATE SKIP LOCKED` ensures only one claims each job |

---

## Files changed

| File | Change |
|---|---|
| `backend/handlers/application_handler.go` | 409 idempotency check before creating `AddLanguageJob` |
| `backend/jobs/worker.go` | `claimAddLanguageJob` / `claimTranslateJob`: stuck-job reset query + `FOR UPDATE SKIP LOCKED` claim |
| `backend/models/models.go` | `AddLanguageJob.TotalComponents` + `AddLanguageJob.CompletedComponents` fields |
| `frontend/app/applications/[id]/page.tsx` | Handle 409 response; poll existing job; show progress bar |
