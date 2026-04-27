/**
 * Global Teardown — runs once after all test suites.
 * Deletes the e2e test application (cascades to components, translations, tags, pages, API keys).
 */

import { request }               from '@playwright/test'
import { loadState, stateExists } from './helpers/state'
import * as fs   from 'fs'
import * as path from 'path'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export default async function globalTeardown() {
  if (!stateExists()) {
    console.log('\n[teardown] No state file — nothing to clean up.')
    return
  }

  const { token, applicationId } = loadState()

  const ctx = await request.newContext({
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  })

  const res = await ctx.delete(`${API_URL}/api/applications/${applicationId}`)
  if (res.ok()) {
    console.log(`\n[teardown] Deleted e2e test application ${applicationId} ✓`)
  } else {
    console.warn(`\n[teardown] Could not delete application ${applicationId}: ${res.status()} ${await res.text()}`)
  }

  // Remove runtime state files
  for (const f of [
    path.join(__dirname, '.test-state.json'),
    path.join(__dirname, 'e2e', '.auth.json'),
  ]) {
    try { fs.unlinkSync(f) } catch { /* already gone */ }
  }

  console.log('[teardown] Done ✓')
}
