/**
 * TC-TMS-030 → TC-TMS-044  Translations API (write path — JWT auth)
 */

import { test, expect } from '@playwright/test'
import { loadState }    from '../helpers/state'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

const SOURCE_DATA   = { greeting: 'Hello', farewell: 'Goodbye', url: 'https://example.com' }
const UPDATED_DATA  = { greeting: 'Hi there', farewell: 'Goodbye', url: 'https://example.com' }

/** Poll a translate-job until completed/failed or timeout (ms). Returns final status body. */
async function pollTranslateJob(
  request: Parameters<Parameters<typeof test>[1]>[0]['request'],
  jobId: string,
  headers: Record<string, string>,
  timeoutMs = 30_000,
): Promise<{ status: string; error_message?: string; error_detail?: string }> {
  const API_URL = process.env.API_URL ?? 'http://localhost:8080'
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const res  = await request.get(`${API_URL}/api/translate-jobs/${jobId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    if (body.status === 'completed' || body.status === 'failed') return body
    await new Promise((r) => setTimeout(r, 1000))
  }
  return { status: 'timeout' }
}

test.describe('Translations API', () => {
  let headers: Record<string, string>
  let componentId: string
  let secondLocale: string

  test.beforeAll(() => {
    const state  = loadState()
    headers      = { Authorization: `Bearer ${state.token}` }
    componentId  = state.componentId
    secondLocale = state.secondLocale
  })

  test('TC-TMS-030 — POST /components/:id/translations saves draft translation', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/components/${componentId}/translations`, {
      headers,
      data: { locale: 'en', stage: 'draft', data: SOURCE_DATA },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('id')
  })

  test('TC-TMS-031 — GET /components/:id/translations returns draft data', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/components/${componentId}/translations?locale=en&stage=draft`,
      { headers }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('data')
    expect(body.data).toHaveProperty('greeting')
  })

  test('TC-TMS-032 — POST /translations/deploy promotes draft → staging', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/components/${componentId}/translations/deploy`, {
      headers,
      data: { locale: 'en', from_stage: 'draft', to_stage: 'staging' },
    })
    expect(res.status()).toBe(200)
  })

  test('TC-TMS-033 — GET staging translation matches what was deployed', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/components/${componentId}/translations?locale=en&stage=staging`,
      { headers }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.data).toHaveProperty('greeting')
  })

  test('TC-TMS-034 — POST /translations/deploy promotes staging → production', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/components/${componentId}/translations/deploy`, {
      headers,
      data: { locale: 'en', from_stage: 'staging', to_stage: 'production' },
    })
    expect(res.status()).toBe(200)
  })

  test('TC-TMS-035 — POST /translations saves new draft (edit cycle)', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/components/${componentId}/translations`, {
      headers,
      data: { locale: 'en', stage: 'draft', data: UPDATED_DATA },
    })
    expect(res.status()).toBe(200)
  })

  test('TC-TMS-036 — GET /translations/compare returns version1 and version2', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/components/${componentId}/translations/compare?locale=en&stage=draft`,
      { headers }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('version1')
    expect(body).toHaveProperty('version2')
  })

  test('TC-TMS-037 — GET /translations/versions returns version list', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/components/${componentId}/translations/versions?locale=en&stage=draft`,
      { headers }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('versions')
    expect(Array.isArray(body.versions)).toBe(true)
    expect(body.versions.length).toBeGreaterThanOrEqual(1)
  })

  test('TC-TMS-038 — POST /translations/revert restores previous version', async ({ request }) => {
    const res = await request.post(
      `${API_URL}/api/components/${componentId}/translations/revert?locale=en&stage=draft`,
      { headers }
    )
    expect(res.status()).toBe(200)
  })

  test('TC-TMS-039 — GET /components/:id/export returns JSON blob', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/components/${componentId}/export?locale=en&stage=production`,
      { headers }
    )
    expect(res.status()).toBe(200)
    const contentType = res.headers()['content-type'] ?? ''
    expect(contentType).toContain('json')
    const body = await res.json()
    expect(typeof body).toBe('object')
  })

  // ── Auto-translate (single locale) ──────────────────────────────────────────

  test('TC-TMS-040 — POST /auto-translate enqueues job, completes, saves translation + source_data snapshot', async ({ request }) => {
    test.setTimeout(60_000)

    // Ensure a fresh EN draft exists (may have been reverted by TC-TMS-038)
    const saveRes = await request.post(`${API_URL}/api/components/${componentId}/translations`, {
      headers,
      data: { locale: 'en', stage: 'draft', data: SOURCE_DATA },
    })
    expect(saveRes.status()).toBe(200)

    const res = await request.post(
      `${API_URL}/api/components/${componentId}/translations/auto-translate`,
      { headers, data: { source_locale: 'en', target_locale: secondLocale, stage: 'draft' } },
    )
    expect(res.status()).toBe(202)
    const body = await res.json()
    expect(body).toHaveProperty('job_id')

    const job = await pollTranslateJob(request, body.job_id, headers)
    expect(job.status, job.error_detail ?? job.error_message ?? 'job failed').toBe('completed')

    // Translation must exist for the target locale
    const trRes = await request.get(
      `${API_URL}/api/components/${componentId}/translations?locale=${secondLocale}&stage=draft`,
      { headers },
    )
    expect(trRes.status()).toBe(200)
    const tr = await trRes.json()
    expect(tr).toHaveProperty('data')
    expect(tr.data).toHaveProperty('greeting')
    expect(tr.data).toHaveProperty('farewell')

    // source_data snapshot must be stored (enables future incremental diff)
    expect(tr).toHaveProperty('source_data')
    expect(tr.source_data).toHaveProperty('greeting')
    expect(tr.source_locale).toBe('en')
  })

  test('TC-TMS-041 — Incremental re-translate: only changed key is sent to AI, removed key is dropped', async ({ request }) => {
    test.setTimeout(60_000)

    // Step 1: save initial source with 3 keys (TC-TMS-040 already translated, snapshot stored)
    const step1Source = { greeting: 'Hello', farewell: 'Goodbye', url: 'https://example.com' }
    await request.post(`${API_URL}/api/components/${componentId}/translations`, {
      headers,
      data: { locale: 'en', stage: 'draft', data: step1Source },
    })
    // Translate to get a baseline snapshot
    const firstJobRes = await request.post(
      `${API_URL}/api/components/${componentId}/translations/auto-translate`,
      { headers, data: { source_locale: 'en', target_locale: secondLocale, stage: 'draft' } },
    )
    expect(firstJobRes.status()).toBe(202)
    const firstJob = await pollTranslateJob(request, (await firstJobRes.json()).job_id, headers)
    expect(firstJob.status).toBe('completed')

    // Step 2: update source — change `greeting`, remove `farewell`, add `new_key`
    const step2Source = { greeting: 'Hi there', url: 'https://example.com', new_key: 'Brand new' }
    await request.post(`${API_URL}/api/components/${componentId}/translations`, {
      headers,
      data: { locale: 'en', stage: 'draft', data: step2Source },
    })

    // Trigger incremental re-translate
    const incrJobRes = await request.post(
      `${API_URL}/api/components/${componentId}/translations/auto-translate`,
      { headers, data: { source_locale: 'en', target_locale: secondLocale, stage: 'draft' } },
    )
    expect(incrJobRes.status()).toBe(202)
    const incrJob = await pollTranslateJob(request, (await incrJobRes.json()).job_id, headers)
    expect(incrJob.status, incrJob.error_detail ?? incrJob.error_message ?? 'incremental job failed').toBe('completed')

    // Verify result structure matches the new source exactly
    const trRes = await request.get(
      `${API_URL}/api/components/${componentId}/translations?locale=${secondLocale}&stage=draft`,
      { headers },
    )
    expect(trRes.status()).toBe(200)
    const tr = await trRes.json()
    expect(tr.data).toHaveProperty('greeting')   // changed — re-translated
    expect(tr.data).toHaveProperty('url')         // unchanged — preserved from previous
    expect(tr.data).toHaveProperty('new_key')     // new — translated
    expect(tr.data).not.toHaveProperty('farewell') // removed from source — pruned from target

    // Snapshot updated to current source
    expect(tr.source_data).toMatchObject(step2Source)
  })

  test('TC-TMS-042 — Incremental re-translate: no-op when source is unchanged', async ({ request }) => {
    test.setTimeout(60_000)

    // Ensure we have a translation with a snapshot (from TC-TMS-041)
    const trBefore = await request.get(
      `${API_URL}/api/components/${componentId}/translations?locale=${secondLocale}&stage=draft`,
      { headers },
    )
    expect(trBefore.status()).toBe(200)
    const versionBefore: number = (await trBefore.json()).version

    // Re-translate without changing the source — should be a no-op (same job completes, no new version)
    const nopJobRes = await request.post(
      `${API_URL}/api/components/${componentId}/translations/auto-translate`,
      { headers, data: { source_locale: 'en', target_locale: secondLocale, stage: 'draft' } },
    )
    expect(nopJobRes.status()).toBe(202)
    const nopJob = await pollTranslateJob(request, (await nopJobRes.json()).job_id, headers)
    expect(nopJob.status).toBe('completed')

    // Version must not have advanced — no new translation version was saved
    const trAfter = await request.get(
      `${API_URL}/api/components/${componentId}/translations?locale=${secondLocale}&stage=draft`,
      { headers },
    )
    expect(trAfter.status()).toBe(200)
    expect((await trAfter.json()).version).toBe(versionBefore)
  })

  // ── Backfill (all locales) ───────────────────────────────────────────────────

  test('TC-TMS-043 — POST /backfill enqueues one job per target locale, all complete, translations saved', async ({ request }) => {
    test.setTimeout(60_000)

    // Restore a clean EN draft so backfill has a known source
    const source = { title: 'Welcome', body: 'Hello [name]', link: 'https://example.com' }
    await request.post(`${API_URL}/api/components/${componentId}/translations`, {
      headers,
      data: { locale: 'en', stage: 'draft', data: source },
    })

    const res = await request.post(
      `${API_URL}/api/components/${componentId}/translations/backfill`,
      { headers, data: { source_locale: 'en', target_locales: [secondLocale], stage: 'draft' } },
    )
    expect(res.status()).toBe(202)
    const body = await res.json()
    expect(body).toHaveProperty('job_ids')
    expect(Array.isArray(body.job_ids)).toBe(true)
    expect(body.job_ids).toHaveLength(1)

    // Poll every job to completion
    for (const jobId of body.job_ids as string[]) {
      const job = await pollTranslateJob(request, jobId, headers)
      expect(job.status, job.error_detail ?? job.error_message ?? `backfill job ${jobId} failed`).toBe('completed')
    }

    // Target locale must have the backfilled translation with correct keys
    const trRes = await request.get(
      `${API_URL}/api/components/${componentId}/translations?locale=${secondLocale}&stage=draft`,
      { headers },
    )
    expect(trRes.status()).toBe(200)
    const tr = await trRes.json()
    expect(tr.data).toHaveProperty('title')
    expect(tr.data).toHaveProperty('body')
    expect(tr.data).toHaveProperty('link')
    expect(tr.source_locale).toBe('en')
    expect(tr.source_data).toMatchObject(source)
  })

  test('TC-TMS-044 — POST /backfill with empty target_locales returns 400', async ({ request }) => {
    const res = await request.post(
      `${API_URL}/api/components/${componentId}/translations/backfill`,
      { headers, data: { source_locale: 'en', target_locales: [], stage: 'draft' } },
    )
    expect(res.status()).toBe(400)
  })
})
