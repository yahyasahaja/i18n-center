/**
 * TC-TMS-063 → TC-TMS-079  Headless CMS API + Public Read Endpoint
 *
 * Tests cover CMS template CRUD, item CRUD, localization lifecycle
 * (save → deploy → revert → versions), the public read API (API key auth),
 * and the i18ncenter-js SDK getCmsContent method.
 *
 * Fixtures created by global-setup:
 *   - cmsTemplateId   — a template with `title` (text) + `subtitle` (textarea) fields
 *   - cmsItemId       — an item using that template
 *   - cmsItemIdentifier — the item's identifier string
 *   - EN localization deployed all the way to production
 */

import { test, expect } from '@playwright/test'
import { loadState }    from '../helpers/state'
import { I18nCenterClient } from '../../i18ncenter-js/src/client'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

// ─── CMS Templates ───────────────────────────────────────────────────────────

test.describe('CMS Templates API', () => {
  let headers: Record<string, string>
  let applicationId: string
  let cmsTemplateId: string
  let extraTemplateId: string  // created in this suite, deleted at end

  test.beforeAll(() => {
    const state = loadState()
    headers       = { Authorization: `Bearer ${state.token}` }
    applicationId = state.applicationId
    cmsTemplateId = state.cmsTemplateId
  })

  test('TC-TMS-063 — GET /applications/:id/cms/templates lists templates', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/${applicationId}/cms/templates`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    expect(body.some((t: any) => t.id === cmsTemplateId)).toBe(true)
  })

  test('TC-TMS-064 — GET /cms/templates/:id returns template with fields', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/cms/templates/${cmsTemplateId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.id).toBe(cmsTemplateId)
    expect(Array.isArray(body.fields)).toBe(true)
    expect(body.fields.length).toBeGreaterThan(0)
    expect(body.fields.some((f: any) => f.key === 'title')).toBe(true)
  })

  test('TC-TMS-065 — POST /applications/:id/cms/templates creates a new template', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/applications/${applicationId}/cms/templates`, {
      headers,
      data: {
        name:   'E2E Extra Template',
        code:   `e2e-extra-tmpl-${Date.now()}`,
        fields: [{ key: 'body', label: 'Body', value_type: 'rich_text', required: false, sort_order: 0 }],
      },
    })
    expect(res.status()).toBe(201)
    const body = await res.json()
    expect(body.id).toBeTruthy()
    expect(body.fields).toHaveLength(1)
    extraTemplateId = body.id
  })

  test('TC-TMS-066 — PUT /cms/templates/:id updates template name and replaces fields', async ({ request }) => {
    const res = await request.put(`${API_URL}/api/cms/templates/${extraTemplateId}`, {
      headers,
      data: {
        name:   'E2E Extra Template (updated)',
        fields: [
          { key: 'body',    label: 'Body',    value_type: 'rich_text', required: false, sort_order: 0 },
          { key: 'caption', label: 'Caption', value_type: 'text',      required: false, sort_order: 1 },
        ],
      },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.name).toBe('E2E Extra Template (updated)')
    expect(body.fields).toHaveLength(2)
  })

  test('TC-TMS-067 — DELETE /cms/templates/:id deletes template (no items)', async ({ request }) => {
    const res = await request.delete(`${API_URL}/api/cms/templates/${extraTemplateId}`, { headers })
    expect(res.status()).toBe(200)
  })

  test('TC-TMS-068 — DELETE /cms/templates/:id fails when items reference it', async ({ request }) => {
    // The fixture template has the fixture item referencing it — must be rejected
    const res = await request.delete(`${API_URL}/api/cms/templates/${cmsTemplateId}`, { headers })
    expect(res.status()).toBe(400)
    const body = await res.json()
    expect(body.error).toMatch(/used by existing CMS items/i)
  })
})

// ─── CMS Items ────────────────────────────────────────────────────────────────

test.describe('CMS Items API', () => {
  let headers: Record<string, string>
  let applicationId: string
  let cmsTemplateId: string
  let cmsItemId: string

  test.beforeAll(() => {
    const state = loadState()
    headers       = { Authorization: `Bearer ${state.token}` }
    applicationId = state.applicationId
    cmsTemplateId = state.cmsTemplateId
    cmsItemId     = state.cmsItemId
  })

  test('TC-TMS-069 — GET /applications/:id/cms/items lists items', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/applications/${applicationId}/cms/items`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    expect(body.some((i: any) => i.id === cmsItemId)).toBe(true)
  })

  test('TC-TMS-070 — GET /cms/items/:id returns item with template and fields', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/cms/items/${cmsItemId}`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.id).toBe(cmsItemId)
    expect(body.template).toBeTruthy()
    expect(Array.isArray(body.template.fields)).toBe(true)
  })

  test('TC-TMS-071 — PUT /cms/items/:id updates item name', async ({ request }) => {
    const res = await request.put(`${API_URL}/api/cms/items/${cmsItemId}`, {
      headers,
      data: { name: 'E2E Flash Banner (updated)' },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.name).toBe('E2E Flash Banner (updated)')
  })
})

// ─── CMS Localizations ───────────────────────────────────────────────────────

test.describe('CMS Localizations API', () => {
  let headers: Record<string, string>
  let cmsItemId: string
  let savedVersion: number

  test.beforeAll(() => {
    const state = loadState()
    headers   = { Authorization: `Bearer ${state.token}` }
    cmsItemId = state.cmsItemId
  })

  test('TC-TMS-072 — GET /cms/items/:id/localizations/detail returns draft localization', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/cms/items/${cmsItemId}/localizations/detail?locale=en&stage=draft`,
      { headers },
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.locale).toBe('en')
    expect(body.stage).toBe('draft')
    expect(body.data).toMatchObject({ title: 'Flash Sale!' })
    savedVersion = body.version
  })

  test('TC-TMS-073 — POST /cms/items/:id/localizations creates a new version', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/cms/items/${cmsItemId}/localizations`, {
      headers,
      data: { locale: 'en', stage: 'draft', data: { title: 'Summer Sale!', subtitle: 'Up to 70% off' } },
    })
    expect(res.status()).toBe(201)
    const body = await res.json()
    expect(body.version).toBeGreaterThan(savedVersion)
    expect(body.data.title).toBe('Summer Sale!')
  })

  test('TC-TMS-074 — GET /cms/items/:id/localizations/versions lists all versions', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/cms/items/${cmsItemId}/localizations/versions?locale=en&stage=draft`,
      { headers },
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    expect(body.length).toBeGreaterThanOrEqual(2)
  })

  test('TC-TMS-075 — POST /cms/items/:id/localizations/revert creates new version from old data', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/cms/items/${cmsItemId}/localizations/revert`, {
      headers,
      data: { locale: 'en', stage: 'draft', version: savedVersion },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    // Reverted data matches the original version
    expect(body.data.title).toBe('Flash Sale!')
    // Version number is higher than the version we reverted to
    expect(body.version).toBeGreaterThan(savedVersion)
  })

  test('TC-TMS-076 — POST /cms/items/:id/localizations/deploy promotes draft → staging', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/cms/items/${cmsItemId}/localizations/deploy`, {
      headers,
      data: { locale: 'en', from_stage: 'draft', to_stage: 'staging' },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.stage).toBe('staging')
  })

  test('TC-TMS-077 — GET /cms/items/:id/localizations lists all stage records', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/cms/items/${cmsItemId}/localizations`, { headers })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
    const stages = [...new Set(body.map((l: any) => l.stage))]
    expect(stages).toContain('draft')
    expect(stages).toContain('production')
  })
})

// ─── Public CMS Read API ──────────────────────────────────────────────────────

test.describe('CMS Public Read API — API Key authentication', () => {
  let apiKey: string
  let applicationId: string
  let cmsItemIdentifier: string

  test.beforeAll(() => {
    const state         = loadState()
    apiKey              = state.apiKey
    applicationId       = state.applicationId
    cmsItemIdentifier   = state.cmsItemIdentifier
  })

  test('TC-TMS-078 — GET /applications/:id/cms/:identifier with API key returns content', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/applications/${applicationId}/cms/${cmsItemIdentifier}?locale=en&stage=production`,
      { headers: { 'X-API-Key': apiKey } },
    )
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.identifier).toBe(cmsItemIdentifier)
    expect(body.locale).toBe('en')
    expect(body.stage).toBe('production')
    expect(body.data).toMatchObject({ title: 'Flash Sale!' })
  })

  test('TC-TMS-079 — GET with invalid API key returns 401', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/applications/${applicationId}/cms/${cmsItemIdentifier}?locale=en&stage=production`,
      { headers: { 'X-API-Key': 'invalid-key-xyz' } },
    )
    expect(res.status()).toBe(401)
  })

  test('TC-TMS-080 — GET for unknown identifier returns 404', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/applications/${applicationId}/cms/does-not-exist?locale=en&stage=production`,
      { headers: { 'X-API-Key': apiKey } },
    )
    expect(res.status()).toBe(404)
  })
})

// ─── SDK Integration ──────────────────────────────────────────────────────────

test.describe('i18ncenter-js SDK — getCmsContent', () => {
  let apiKey: string
  let applicationId: string
  let cmsItemIdentifier: string

  test.beforeAll(() => {
    const state         = loadState()
    apiKey              = state.apiKey
    applicationId       = state.applicationId
    cmsItemIdentifier   = state.cmsItemIdentifier
  })

  test('TC-TMS-081 — getCmsContent returns typed CMS content', async () => {
    const client = new I18nCenterClient({
      apiUrl:    `${API_URL}/api`,
      apiToken:  apiKey,
      enableCache: false,
    })

    const content = await client.getCmsContent(applicationId, cmsItemIdentifier, 'en', 'production')
    expect(content.identifier).toBe(cmsItemIdentifier)
    expect(content.locale).toBe('en')
    expect(content.stage).toBe('production')
    expect(content.data.title).toBe('Flash Sale!')
  })

  test('TC-TMS-082 — getCmsContent uses cache on second call', async () => {
    const client = new I18nCenterClient({
      apiUrl:   `${API_URL}/api`,
      apiToken: apiKey,
    })

    const first  = await client.getCmsContent(applicationId, cmsItemIdentifier, 'en', 'production')
    const second = await client.getCmsContent(applicationId, cmsItemIdentifier, 'en', 'production')
    expect(first.data.title).toBe(second.data.title)
  })

  test('TC-TMS-083 — getCmsContent throws on unknown identifier', async () => {
    const client = new I18nCenterClient({
      apiUrl:      `${API_URL}/api`,
      apiToken:    apiKey,
      enableCache: false,
    })

    await expect(
      client.getCmsContent(applicationId, 'no-such-item', 'en', 'production')
    ).rejects.toThrow('CMS content not found')
  })
})
