/**
 * Global Setup — runs once before all test suites.
 *
 * Creates a self-contained test application with:
 *   - EN language enabled
 *   - One component (e2e-component)
 *   - Draft/Staging/Production translations
 *   - One tag + one page (both linked to the component)
 *   - One API key
 *
 * All IDs are saved to e2e/.test-state.json for use by spec files.
 * A browser auth state is saved to e2e/.auth.json for UI tests.
 */

import { chromium, request } from '@playwright/test'
import { saveState }         from './helpers/state'
import * as path             from 'path'
import * as fs               from 'fs'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'
const UI_URL  = process.env.UI_URL  ?? 'http://localhost:3000'

const USERNAME = process.env.TEST_USERNAME ?? 'admin'
const PASSWORD = process.env.TEST_PASSWORD ?? 'admin123'

const TS   = Date.now()
const TAG  = `e2e-tag-${TS}`
const PAGE = `e2e-page-${TS}`
const APP  = `e2e-app-${TS}`
const COMP = `e2e-component-${TS}`

async function post(ctx: Awaited<ReturnType<typeof request.newContext>>, path: string, body: unknown) {
  const res = await ctx.post(`${API_URL}${path}`, { data: body })
  if (!res.ok()) {
    throw new Error(`POST ${path} → ${res.status()}: ${await res.text()}`)
  }
  return res.json()
}

async function get(ctx: Awaited<ReturnType<typeof request.newContext>>, path: string) {
  const res = await ctx.get(`${API_URL}${path}`)
  if (!res.ok()) {
    throw new Error(`GET ${path} → ${res.status()}: ${await res.text()}`)
  }
  return res.json()
}

export default async function globalSetup() {
  console.log('\n[setup] Creating e2e test fixtures…')

  // ── 1. Login ────────────────────────────────────────────────────────────────
  const ctx = await request.newContext()
  const { token } = await post(ctx, '/api/auth/login', { username: USERNAME, password: PASSWORD })

  const auth = await request.newContext({
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  })

  // ── 2. Application ─────────────────────────────────────────────────────────
  const app = await post(auth, '/api/applications', {
    name: APP,
    code: APP,
    description: 'Automated e2e test application — safe to delete',
    openai_key: 'mock',
  })
  const applicationId: string = app.id
  console.log(`[setup] application: ${applicationId} (${APP})`)

  // ── 3. Enable EN language ───────────────────────────────────────────────────
  await post(auth, `/api/applications/${applicationId}/languages`, {
    locale: 'en',
    auto_translate: false,
  })

  // ── 4. Tag ─────────────────────────────────────────────────────────────────
  const tag = await post(auth, `/api/applications/${applicationId}/tags`, { code: TAG })
  const tagId: string = tag.id
  console.log(`[setup] tag: ${tagId} (${TAG})`)

  // ── 5. Page ────────────────────────────────────────────────────────────────
  const page = await post(auth, `/api/applications/${applicationId}/pages`, { code: PAGE })
  const pageId: string = page.id
  console.log(`[setup] page: ${pageId} (${PAGE})`)

  // ── 6. Component (linked to tag + page) ─────────────────────────────────────
  const comp = await post(auth, '/api/components', {
    application_id: applicationId,
    code:           COMP,
    name:           'E2E Test Component',
    default_locale: 'en',
    tag_ids:        [tagId],
    page_ids:       [pageId],
  })
  const componentId: string = comp.id
  console.log(`[setup] component: ${componentId} (${COMP})`)

  // ── 7. Save EN Draft translation ────────────────────────────────────────────
  await post(auth, `/api/components/${componentId}/translations`, {
    locale: 'en',
    stage:  'draft',
    data:   { hello: 'Hello', world: 'World', url: 'https://example.com' },
  })

  // ── 8. Deploy draft → staging ───────────────────────────────────────────────
  await post(auth, `/api/components/${componentId}/translations/deploy`, {
    locale:     'en',
    from_stage: 'draft',
    to_stage:   'staging',
  })

  // ── 9. Deploy staging → production ─────────────────────────────────────────
  await post(auth, `/api/components/${componentId}/translations/deploy`, {
    locale:     'en',
    from_stage: 'staging',
    to_stage:   'production',
  })

  // ── 10. API Key ─────────────────────────────────────────────────────────────
  const keyRes = await post(auth, `/api/applications/${applicationId}/api-keys`, {
    name: 'e2e-test-key',
  })
  const apiKey:   string = keyRes.key    // full key (only returned once)
  const apiKeyId: string = keyRes.id

  console.log(`[setup] api-key: ${apiKeyId}`)

  // ── 11. Persist test state ─────────────────────────────────────────────────
  saveState({ token, applicationId, applicationCode: APP, componentId, componentCode: COMP, tagId, pageId, apiKey, apiKeyId })

  // ── 12. Browser auth state (for UI tests) ─────────────────────────────────
  const authDir = path.join(__dirname, 'e2e')
  fs.mkdirSync(authDir, { recursive: true })

  const browser = await chromium.launch()
  const browserCtx = await browser.newContext()
  const bPage = await browserCtx.newPage()

  // Set token in localStorage so the frontend's AuthInitializer picks it up
  await bPage.goto(UI_URL)
  await bPage.evaluate((t: string) => localStorage.setItem('token', t), token)
  await browserCtx.storageState({ path: path.join(authDir, '.auth.json') })
  await browser.close()

  console.log('[setup] Done ✓')
}
