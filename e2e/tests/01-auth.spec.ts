/**
 * TC-TMS-001 → TC-TMS-004  Authentication
 */

import { test, expect } from '@playwright/test'
import { loadState }    from '../helpers/state'

const API_URL  = process.env.API_URL ?? 'http://localhost:8080'
const USERNAME = process.env.TEST_USERNAME ?? 'admin'
const PASSWORD = process.env.TEST_PASSWORD ?? 'admin123'

test.describe('Authentication', () => {
  test('TC-TMS-001 — POST /auth/login with valid credentials returns 200 + JWT', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/auth/login`, {
      data: { username: USERNAME, password: PASSWORD },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('token')
    expect(typeof body.token).toBe('string')
    expect(body.token.length).toBeGreaterThan(10)
  })

  test('TC-TMS-002 — POST /auth/login with invalid credentials returns 401', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/auth/login`, {
      data: { username: USERNAME, password: 'wrong-password-xyz' },
    })
    expect(res.status()).toBe(401)
  })

  test('TC-TMS-003 — GET /auth/me with valid token returns current user info', async ({ request }) => {
    const { token } = loadState()
    const res = await request.get(`${API_URL}/api/auth/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('username')
    expect(body).toHaveProperty('role')
    expect(body.username).toBe(USERNAME)
  })

  test('TC-TMS-004 — GET /auth/me without token returns 401', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/auth/me`)
    expect(res.status()).toBe(401)
  })
})
