/**
 * TC-TMS-020 → TC-TMS-025  Components API
 */

import { test, expect } from '@playwright/test'
import { loadState }    from '../helpers/state'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

test.describe('Components API', () => {
  let headers: Record<string, string>
  let applicationId: string
  let componentId: string
  let componentCode: string
  let tagId: string
  let pageId: string
  let tempComponentId: string

  test.beforeAll(() => {
    const state = loadState()
    headers       = { Authorization: `Bearer ${state.token}` }
    applicationId = state.applicationId
    componentId   = state.componentId
    componentCode = state.componentCode
    tagId         = state.tagId
    pageId        = state.pageId
  })

  test('TC-TMS-020 — GET /components?application_id returns e2e component', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/components?application_id=${applicationId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    const found = body.find((c: any) => c.id === componentId)
    expect(found).toBeDefined()
    expect(found.code).toBe(componentCode)
  })

  test('TC-TMS-021 — GET /components/:id returns component with tags and pages', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/components/${componentId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.id).toBe(componentId)
    expect(Array.isArray(body.tags)).toBe(true)
    expect(Array.isArray(body.pages)).toBe(true)
    expect(body.tags.some((t: any) => t.id === tagId)).toBe(true)
    expect(body.pages.some((p: any) => p.id === pageId)).toBe(true)
  })

  test('TC-TMS-022 — PUT /components/:id updates component name', async ({ request }) => {
    const res = await request.put(`${API_URL}/api/components/${componentId}`, {
      headers,
      data: { name: 'E2E Component Updated' },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.name).toBe('E2E Component Updated')
  })

  test('TC-TMS-023 — POST /components creates a temporary component', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/components`, {
      headers,
      data: {
        application_id: applicationId,
        code:           `e2e-temp-${Date.now()}`,
        name:           'E2E Temp',
        default_locale: 'en',
      },
    })
    expect(res.status()).toBe(201)
    const body = await res.json()
    expect(body).toHaveProperty('id')
    expect(body.code).toMatch(/^e2e-temp-/)
    tempComponentId = body.id
  })

  test('TC-TMS-024 — DELETE /components/:id deletes the temporary component', async ({ request }) => {
    test.skip(!tempComponentId, 'TC-TMS-023 must pass first')
    const res = await request.delete(`${API_URL}/api/components/${tempComponentId}`, { headers })
    expect([200, 204]).toContain(res.status())
  })

  test('TC-TMS-025 — GET /components/:id for deleted component returns 404', async ({ request }) => {
    test.skip(!tempComponentId, 'TC-TMS-024 must pass first')
    const res = await request.get(`${API_URL}/api/components/${tempComponentId}`, { headers })
    expect(res.status()).toBe(404)
  })
})
