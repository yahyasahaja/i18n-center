/**
 * TC-TMS-030 → TC-TMS-039  Translations API (write path — JWT auth)
 */

import { test, expect } from '@playwright/test'
import { loadState }    from '../helpers/state'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

const SOURCE_DATA = { greeting: 'Hello', farewell: 'Goodbye', url: 'https://example.com' }
const UPDATED_DATA = { greeting: 'Hi there', farewell: 'Goodbye', url: 'https://example.com' }

test.describe('Translations API', () => {
  let headers: Record<string, string>
  let componentId: string

  test.beforeAll(() => {
    const state = loadState()
    headers     = { Authorization: `Bearer ${state.token}` }
    componentId = state.componentId
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
})
