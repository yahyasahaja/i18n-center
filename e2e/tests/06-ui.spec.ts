/**
 * TC-TMS-050 → TC-TMS-061  UI / Dashboard (Chromium)
 *
 * Browser auth is pre-seeded by global-setup (localStorage token).
 * Tests navigate the Next.js frontend and assert key UI elements.
 */

import { test, expect, type Page } from '@playwright/test'
import { loadState }               from '../helpers/state'

const UI_URL = process.env.UI_URL ?? 'http://localhost:3000'

// Helper: navigate and wait for hydration
async function goto(page: Page, path: string) {
  await page.goto(`${UI_URL}${path}`)
  await page.waitForLoadState('networkidle')
}

test.describe('UI — Authentication', () => {
  test('TC-TMS-050 — /login renders login form', async ({ browser }) => {
    // Explicitly create a context with NO storage state (truly unauthenticated)
    const ctx  = await browser.newContext({ storageState: { cookies: [], origins: [] } })
    const page = await ctx.newPage()
    await goto(page, '/login')

    await expect(page.locator('input[type="text"], input[name="username"]')).toBeVisible()
    await expect(page.locator('input[type="password"]')).toBeVisible()
    await ctx.close()
  })

  test('TC-TMS-051 — Login with invalid credentials returns 401 and stays on login page', async ({ browser }) => {
    const ctx  = await browser.newContext({ storageState: { cookies: [], origins: [] } })
    const page = await ctx.newPage()
    await goto(page, '/login')

    await page.locator('input[name="username"]').fill('admin')
    await page.locator('input[type="password"]').fill('wrong-password')

    // Capture the login API response while clicking submit
    const [response] = await Promise.all([
      page.waitForResponse((resp) => resp.url().includes('/auth/login')),
      page.locator('button[type="submit"]').click(),
    ])

    // Backend must reject with 401
    expect(response.status()).toBe(401)
    // Page must stay on login (not redirect to dashboard)
    await expect(page).toHaveURL(/login/, { timeout: 3000 })
    await ctx.close()
  })
})

test.describe('UI — Dashboard & Navigation', () => {
  // These tests use the pre-authenticated storage state
  test('TC-TMS-052 — /dashboard is accessible and shows stats', async ({ page }) => {
    await goto(page, '/dashboard')
    // Should NOT be redirected to login
    await expect(page).not.toHaveURL(/login/)
    // Dashboard has at least one stat card or heading
    await expect(page.locator('h1, h2, [class*="card"], [class*="stat"]').first()).toBeVisible()
  })

  test('TC-TMS-053 — /applications lists at least the e2e application', async ({ page }) => {
    await goto(page, '/applications')
    await expect(page).not.toHaveURL(/login/)
    // Look for the e2e app in the table (code column), not in hidden <option> elements
    await expect(page.locator('table').locator('text=/e2e-app/i').first()).toBeVisible({ timeout: 8000 })
  })

  test('TC-TMS-054 — Application detail page loads and shows components count', async ({ page }) => {
    const { applicationId } = loadState()
    await goto(page, `/applications/${applicationId}`)
    await expect(page).not.toHaveURL(/login/)
    // Components count card should be visible
    await expect(page.locator('text=/Components/i').first()).toBeVisible({ timeout: 8000 })
  })

  test('TC-TMS-055 — Application detail shows Bootstrap Import button', async ({ page }) => {
    const { applicationId } = loadState()
    await goto(page, `/applications/${applicationId}`)
    await expect(page.locator('button', { hasText: /Bootstrap Import/i })).toBeVisible({ timeout: 8000 })
  })

  test('TC-TMS-056 — Application detail shows Download JSON button', async ({ page }) => {
    const { applicationId } = loadState()
    await goto(page, `/applications/${applicationId}`)
    await expect(page.locator('button', { hasText: /Download JSON/i })).toBeVisible({ timeout: 8000 })
  })
})

test.describe('UI — Components & Translation Editor', () => {
  test('TC-TMS-057 — /components lists the e2e component', async ({ page }) => {
    const { applicationId } = loadState()
    await goto(page, `/components?application_id=${applicationId}`)
    await expect(page).not.toHaveURL(/login/)
    await expect(page.locator('text=/e2e-component/i').first()).toBeVisible({ timeout: 8000 })
  })

  test('TC-TMS-058 — Translation editor page loads and shows JSON editor', async ({ page }) => {
    const { componentId } = loadState()
    await goto(page, `/components/${componentId}/translations?locale=en&stage=draft`)
    await expect(page).not.toHaveURL(/login/)
    // Editor should show the monaco/code editor area
    await expect(page.locator('[class*="editor"], .monaco-editor, textarea').first()).toBeVisible({ timeout: 10000 })
  })

  test('TC-TMS-059 — Translation editor shows Save button (disabled when no changes)', async ({ page }) => {
    const { componentId } = loadState()
    await goto(page, `/components/${componentId}/translations?locale=en&stage=draft`)
    const saveBtn = page.locator('button', { hasText: /^Save$/ })
    await expect(saveBtn).toBeVisible({ timeout: 8000 })
    // Save should be disabled when there are no unsaved changes
    await expect(saveBtn).toBeDisabled()
  })

  test('TC-TMS-060 — Add language modal opens from application detail', async ({ page }) => {
    const { applicationId } = loadState()
    await goto(page, `/applications/${applicationId}`)
    await page.locator('button', { hasText: /Add language/i }).click()
    // The custom Modal renders a heading with the title; check it is visible
    await expect(page.locator('h3', { hasText: /Add language/i })).toBeVisible({ timeout: 5000 })
  })

  test('TC-TMS-060A — Add language submits successfully (manual mode)', async ({ page }) => {
    const { applicationId } = loadState()
    await goto(page, `/applications/${applicationId}`)
    await page.locator('button', { hasText: /Add language/i }).click()
    await expect(page.locator('h3', { hasText: /Add language/i })).toBeVisible({ timeout: 5000 })

    await page.getByPlaceholder('e.g. id, es, fr').fill('de')
    const autoTranslateCheckbox = page.getByRole('checkbox')
    await expect(autoTranslateCheckbox).toBeChecked()
    await autoTranslateCheckbox.uncheck()
    await expect(autoTranslateCheckbox).not.toBeChecked()

    await page.getByPlaceholder('e.g. id, es, fr').press('Enter')
    await expect(page.locator('h3', { hasText: /Add language/i })).not.toBeVisible({ timeout: 10000 })
    await expect(page.locator('span', { hasText: /^DE$/ })).toBeVisible({ timeout: 10000 })
  })

  test('TC-TMS-061 — Bootstrap Import modal opens and closes', async ({ page }) => {
    const { applicationId } = loadState()
    await goto(page, `/applications/${applicationId}`)
    await page.locator('button', { hasText: /Bootstrap Import/i }).click()
    // The modal title appears as an h3
    await expect(page.locator('h3', { hasText: /Bootstrap Import/i })).toBeVisible({ timeout: 5000 })
    // Close via the footer Close button
    await page.locator('button', { hasText: /^Close$/ }).click()
    await expect(page.locator('h3', { hasText: /Bootstrap Import/i })).not.toBeVisible({ timeout: 3000 })
  })
})

test.describe('UI — Audit & Users', () => {
  test('TC-TMS-062 — /users page accessible by super_admin', async ({ page }) => {
    await goto(page, '/users')
    await expect(page).not.toHaveURL(/login/)
    // Users page should show a table or list
    await expect(page.locator('h1, h2, table, [class*="user"]').first()).toBeVisible({ timeout: 8000 })
  })
})
