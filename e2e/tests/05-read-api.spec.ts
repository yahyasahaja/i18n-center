/**
 * TC-TMS-040 → TC-TMS-048  Tags, Pages, and Read API (API Key auth)
 *
 * These tests cover the public translation endpoints that client apps
 * (FE1, FE2, Go services) call with an API Key instead of a JWT.
 */

import { test, expect } from '@playwright/test'
import { loadState }    from '../helpers/state'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

test.describe('Tags API', () => {
  let headers: Record<string, string>
  let applicationId: string
  let tagId: string

  test.beforeAll(() => {
    const state = loadState()
    headers       = { Authorization: `Bearer ${state.token}` }
    applicationId = state.applicationId
    tagId         = state.tagId
  })

  test('TC-TMS-040 — GET /applications/:id/tags returns tag list', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/${applicationId}/tags`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    expect(body.some((t: any) => t.id === tagId)).toBe(true)
  })

  test('TC-TMS-041 — GET /tags/:id returns tag', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/tags/${tagId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.id).toBe(tagId)
  })

  test('TC-TMS-042 — GET /tags/:id/components returns component linked during setup', async ({ request }) => {
    const { componentId } = loadState()
    const res = await request.get(`${API_URL}/api/tags/${tagId}/components`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    expect(body.some((c: any) => c.id === componentId)).toBe(true)
  })
})

test.describe('Pages API', () => {
  let headers: Record<string, string>
  let applicationId: string
  let pageId: string

  test.beforeAll(() => {
    const state = loadState()
    headers       = { Authorization: `Bearer ${state.token}` }
    applicationId = state.applicationId
    pageId        = state.pageId
  })

  test('TC-TMS-043 — GET /applications/:id/pages returns page list', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/${applicationId}/pages`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    expect(body.some((p: any) => p.id === pageId)).toBe(true)
  })

  test('TC-TMS-044 — GET /pages/:id returns page', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/pages/${pageId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.id).toBe(pageId)
  })

  test('TC-TMS-045 — GET /pages/:id/components returns component linked during setup', async ({ request }) => {
    const { componentId } = loadState()
    const res = await request.get(`${API_URL}/api/pages/${pageId}/components`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    expect(body.some((c: any) => c.id === componentId)).toBe(true)
  })
})

test.describe('Read API — API Key authentication', () => {
  let apiKey: string
  let applicationId: string
  let tagCode: string
  let pageCode: string
  let componentId: string

  test.beforeAll(() => {
    const state = loadState()
    apiKey        = state.apiKey
    applicationId = state.applicationId
    componentId   = state.componentId
    // tag/page codes embedded in state via tag/page IDs — fetch them separately
    // For the read API we use the code (not ID); global-setup stores IDs only.
    // The codes follow the pattern set in global-setup.ts.
    // We re-read them from the actual GET response below.
  })

  test('TC-TMS-046 — GET /translations/by-tag/:code with API key returns translations', async ({ request }) => {
    // Fetch the tag code first via JWT, then re-test with API key
    const jwt = loadState().token
    const tagRes = await request.get(`${API_URL}/api/tags/${loadState().tagId}`, {
      headers: { Authorization: `Bearer ${jwt}` },
    })
    const { code: tc } = await tagRes.json()

    const res = await request.get(
      `${API_URL}/api/applications/${applicationId}/translations/by-tag/${tc}?locale=en&stage=production`,
      { headers: { 'X-API-Key': apiKey } }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(typeof body).toBe('object')
    // Should contain the e2e component's translations (keyed by component code)
    const { componentCode } = loadState()
    expect(body).toHaveProperty(componentCode)
  })

  test('TC-TMS-047 — GET /translations/by-page/:code with API key returns translations', async ({ request }) => {
    const jwt = loadState().token
    const pageRes = await request.get(`${API_URL}/api/pages/${loadState().pageId}`, {
      headers: { Authorization: `Bearer ${jwt}` },
    })
    const { code: pc } = await pageRes.json()

    const res = await request.get(
      `${API_URL}/api/applications/${applicationId}/translations/by-page/${pc}?locale=en&stage=production`,
      { headers: { 'X-API-Key': apiKey } }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(typeof body).toBe('object')
    const { componentCode } = loadState()
    expect(body).toHaveProperty(componentCode)
  })

  test('TC-TMS-048 — GET /translations/bulk with API key returns component data', async ({ request }) => {
    const { applicationCode, componentCode } = loadState()
    const res = await request.get(
      `${API_URL}/api/translations/bulk?application_code=${applicationCode}&component_codes=${componentCode}&locale=en&stage=production`,
      { headers: { 'X-API-Key': apiKey } }
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(typeof body).toBe('object')
  })

  test('TC-TMS-049 — Read API with invalid API key returns 401', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/applications/${applicationId}/translations/by-page/anything?locale=en&stage=production`,
      { headers: { 'X-API-Key': 'invalid-key-xyz' } }
    )
    expect(res.status()).toBe(401)
  })
})
