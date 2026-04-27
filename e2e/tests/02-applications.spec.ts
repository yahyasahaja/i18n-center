/**
 * TC-TMS-010 → TC-TMS-019  Applications API
 */

import { test, expect } from '@playwright/test'
import { loadState }    from '../helpers/state'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

test.describe('Applications API', () => {
  let headers: Record<string, string>
  let applicationId: string

  test.beforeAll(() => {
    const state = loadState()
    headers       = { Authorization: `Bearer ${state.token}` }
    applicationId = state.applicationId
  })

  test('TC-TMS-010 — GET /applications returns array containing the e2e app', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    const found = body.find((a: any) => a.id === applicationId)
    expect(found).toBeDefined()
  })

  test('TC-TMS-011 — GET /applications/:id returns application object', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/${applicationId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.id).toBe(applicationId)
    expect(body).toHaveProperty('enabled_languages')
    expect(body.enabled_languages).toContain('en')
  })

  test('TC-TMS-012 — PUT /applications/:id updates name', async ({ request }) => {
    const res = await request.put(`${API_URL}/api/applications/${applicationId}`, {
      headers,
      data: { name: 'E2E Updated Name' },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.name).toBe('E2E Updated Name')
  })

  test('TC-TMS-013 — GET /applications/:id/pending-deploys returns array', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/${applicationId}/pending-deploys`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('pending_deploys')
    expect(Array.isArray(body.pending_deploys)).toBe(true)
  })

  test('TC-TMS-014 — GET /applications/:id/active-jobs returns job structure', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/${applicationId}/active-jobs`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('add_language_jobs')
    expect(body).toHaveProperty('translate_jobs')
    expect(Array.isArray(body.add_language_jobs)).toBe(true)
    expect(Array.isArray(body.translate_jobs)).toBe(true)
  })

  test('TC-TMS-015 — POST /applications/:id/bootstrap seeds components from JSON', async ({ request }) => {
    const res = await request.post(
      `${API_URL}/api/applications/${applicationId}/bootstrap?locale=en&stage=draft`,
      {
        headers,
        data: {
          data: {
            bootstrap_section: { title: 'Bootstrap Title', subtitle: 'Bootstrap Subtitle' },
            flat_key_one: 'Flat value one',
          },
        },
      }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.components_created).toBeGreaterThanOrEqual(1)
    expect(body.keys_imported).toBeGreaterThan(0)
    expect(Array.isArray(body.components)).toBe(true)
  })

  test('TC-TMS-016 — GET /applications/:id/api-keys returns array with e2e key', async ({ request }) => {
    const { apiKeyId } = loadState()
    const res = await request.get(`${API_URL}/api/applications/${applicationId}/api-keys`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    const found = body.find((k: any) => k.id === apiKeyId)
    expect(found).toBeDefined()
  })

  test('TC-TMS-017 — POST /applications/:id/languages adds new locale', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/applications/${applicationId}/languages`, {
      headers,
      data: { locale: 'id', auto_translate: false },
    })
    // 200 success or 409 if already added — both are acceptable in repeated runs
    expect([200, 201, 409]).toContain(res.status())
  })

  test('TC-TMS-018 — DELETE /applications/:id/languages/:locale removes locale', async ({ request }) => {
    // First add a throwaway locale, then delete it
    await request.post(`${API_URL}/api/applications/${applicationId}/languages`, {
      headers,
      data: { locale: 'fr', auto_translate: false },
    })
    const res = await request.delete(`${API_URL}/api/applications/${applicationId}/languages/fr`, { headers })
    expect([200, 204]).toContain(res.status())
  })

  test('TC-TMS-018A — POST /applications/:id/languages with auto_translate=true completes add-language job and creates draft translations', async ({ request }) => {
    test.setTimeout(120_000)
    const ts = Date.now()
    const appCode = `e2e-auto-${ts}`
    let tempAppId = ''
    let tempComponentId = ''
    const locale = 'es'

    const appRes = await request.post(`${API_URL}/api/applications`, {
      headers,
      data: {
        name: `E2E Auto Translate ${ts}`,
        code: appCode,
        description: 'Temp application for auto-translate e2e verification',
        openai_key: 'mock',
      },
    })
    expect(appRes.status()).toBe(201)
    tempAppId = (await appRes.json()).id

    try {
      const addEnRes = await request.post(`${API_URL}/api/applications/${tempAppId}/languages`, {
        headers,
        data: { locale: 'en', auto_translate: false },
      })
      expect([200, 201, 409]).toContain(addEnRes.status())

      const compRes = await request.post(`${API_URL}/api/components`, {
        headers,
        data: {
          application_id: tempAppId,
          code: `e2e-auto-component-${ts}`,
          name: 'E2E Auto Component',
          default_locale: 'en',
        },
      })
      expect(compRes.status()).toBe(201)
      tempComponentId = (await compRes.json()).id

      const saveDraftRes = await request.post(`${API_URL}/api/components/${tempComponentId}/translations`, {
        headers,
        data: {
          locale: 'en',
          stage: 'draft',
          data: {
            hello: 'Hello',
            welcome_message: 'Welcome [name]',
            url: 'https://example.com',
          },
        },
      })
      expect(saveDraftRes.status()).toBe(200)

      const createRes = await request.post(`${API_URL}/api/applications/${tempAppId}/languages`, {
        headers,
        data: { locale, auto_translate: true },
      })
      expect(createRes.status()).toBe(202)
      const createBody = await createRes.json()
      expect(createBody).toHaveProperty('job_id')
      const jobId = createBody.job_id as string
      expect(jobId).toBeTruthy()

      let finalStatus = 'pending'
      let statusBody: any = null
      for (let i = 0; i < 60; i++) {
        const statusRes = await request.get(`${API_URL}/api/applications/${tempAppId}/jobs/${jobId}`, { headers })
        expect(statusRes.status()).toBe(200)
        statusBody = await statusRes.json()
        finalStatus = statusBody.status
        if (finalStatus === 'completed' || finalStatus === 'failed') break
        await new Promise((resolve) => setTimeout(resolve, 1000))
      }
      expect(finalStatus, statusBody?.error_detail || statusBody?.error_message || 'auto-translate job failed').toBe('completed')

      const trRes = await request.get(
        `${API_URL}/api/components/${tempComponentId}/translations?locale=${encodeURIComponent(locale)}&stage=draft`,
        { headers }
      )
      expect(trRes.status()).toBe(200)
      const trBody = await trRes.json()
      expect(trBody).toHaveProperty('data')
      expect(typeof trBody.data).toBe('object')
      expect(Object.keys(trBody.data)).toContain('hello')
      expect(String(trBody.data.hello)).toContain(`[${locale.toLowerCase()}-mock]`)
    } finally {
      if (tempAppId) {
        await request.delete(`${API_URL}/api/applications/${tempAppId}`, { headers })
      }
    }
  })

  test('TC-TMS-019 — GET /applications/:id with invalid ID returns 400 or 404', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/not-a-uuid`, { headers })
    expect([400, 404]).toContain(res.status())
  })
})
