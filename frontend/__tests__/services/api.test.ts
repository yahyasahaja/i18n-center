import MockAdapter from 'axios-mock-adapter'
import api, {
  authApi,
  applicationApi,
  componentApi,
  translationApi,
  tagApi,
  pageApi,
} from '@/services/api'

// axios-mock-adapter installed directly on the axios instance used by all api methods
const apiMock = new MockAdapter(api)

beforeEach(() => {
  apiMock.reset()
  localStorage.clear()
})

afterAll(() => {
  apiMock.restore()
})

// The axios instance has baseURL = http://localhost:8080/api
// axios-mock-adapter matches relative to the baseURL, so paths are /auth/login etc.
describe('authApi', () => {
  it('login posts credentials and returns data', async () => {
    const responseData = { token: 'tok', user: { id: '1', username: 'admin' } }
    apiMock.onPost('/auth/login').reply(200, responseData)
    const result = await authApi.login({ username: 'admin', password: 'pass' })
    expect(result).toEqual(responseData)
  })

  it('getCurrentUser fetches /auth/me', async () => {
    const user = { id: '1', username: 'admin', role: 'super_admin' }
    apiMock.onGet('/auth/me').reply(200, user)
    const result = await authApi.getCurrentUser()
    expect(result).toEqual(user)
  })

  it('getUsers fetches /auth/users', async () => {
    apiMock.onGet('/auth/users').reply(200, [])
    const result = await authApi.getUsers()
    expect(result).toEqual([])
  })

  it('createUser posts to /auth/users', async () => {
    const newUser = { id: 'u1', username: 'newuser' }
    apiMock.onPost('/auth/users').reply(201, newUser)
    const result = await authApi.createUser({ username: 'newuser', password: 'p' })
    expect(result).toEqual(newUser)
  })

  it('updateUser puts to /auth/users/:id', async () => {
    const updated = { id: 'u1', username: 'updated' }
    apiMock.onPut('/auth/users/u1').reply(200, updated)
    const result = await authApi.updateUser('u1', { username: 'updated' })
    expect(result).toEqual(updated)
  })
})

describe('applicationApi', () => {
  it('getAll fetches /applications', async () => {
    apiMock.onGet('/applications').reply(200, [{ id: 'a1' }])
    const result = await applicationApi.getAll()
    expect(result).toEqual([{ id: 'a1' }])
  })

  it('getById fetches /applications/:id', async () => {
    apiMock.onGet('/applications/a1').reply(200, { id: 'a1', name: 'App' })
    const result = await applicationApi.getById('a1')
    expect(result.name).toBe('App')
  })

  it('create posts to /applications', async () => {
    const body = { name: 'New App', code: 'new-app' }
    apiMock.onPost('/applications').reply(201, { id: 'a2', ...body })
    const result = await applicationApi.create(body)
    expect(result.id).toBe('a2')
  })

  it('update puts to /applications/:id', async () => {
    const updated = { id: 'a1', name: 'Updated' }
    apiMock.onPut('/applications/a1').reply(200, updated)
    const result = await applicationApi.update('a1', { name: 'Updated' })
    expect(result.name).toBe('Updated')
  })

  it('delete sends DELETE to /applications/:id', async () => {
    apiMock.onDelete('/applications/a1').reply(200, { success: true })
    const result = await applicationApi.delete('a1')
    expect(result).toEqual({ success: true })
  })

  it('bootstrap posts to correct URL with locale and stage params', async () => {
    const responseData = {
      components_created: 5,
      components_updated: 0,
      keys_imported: 10,
      flat_keys_in_common: 2,
      components: ['header'],
    }
    // bootstrap embeds locale & stage as query params in the URL string
    apiMock.onPost(/\/applications\/a1\/bootstrap/).reply(200, responseData)
    const result = await applicationApi.bootstrap('a1', { key: 'value' }, 'en', 'draft')
    expect(result.components_created).toBe(5)
  })

  it('listApiKeys fetches from /applications/:id/api-keys', async () => {
    apiMock.onGet('/applications/a1/api-keys').reply(200, [])
    const result = await applicationApi.listApiKeys('a1')
    expect(result).toEqual([])
  })

  it('createApiKey posts to /applications/:id/api-keys', async () => {
    const key = { id: 'k1', key_prefix: 'abc', name: 'test', created_at: '2024-01-01' }
    apiMock.onPost('/applications/a1/api-keys').reply(201, key)
    const result = await applicationApi.createApiKey('a1', { name: 'test' })
    expect(result.id).toBe('k1')
  })

  it('deleteApiKey deletes /applications/:id/api-keys/:keyId', async () => {
    apiMock.onDelete('/applications/a1/api-keys/k1').reply(200, { success: true })
    const result = await applicationApi.deleteApiKey('a1', 'k1')
    expect(result).toEqual({ success: true })
  })

  it('addLanguage posts to /applications/:id/languages', async () => {
    apiMock.onPost('/applications/a1/languages').reply(200, { success: true })
    const result = await applicationApi.addLanguage('a1', { locale: 'fr', auto_translate: false })
    expect(result).toEqual({ success: true })
  })

  it('getPendingDeploys fetches /applications/:id/pending-deploys', async () => {
    apiMock.onGet('/applications/a1/pending-deploys').reply(200, [])
    const result = await applicationApi.getPendingDeploys('a1')
    expect(result).toEqual([])
  })
})

describe('componentApi', () => {
  it('getAll fetches /components', async () => {
    apiMock.onGet('/components').reply(200, [{ id: 'c1' }])
    const result = await componentApi.getAll()
    expect(result).toEqual([{ id: 'c1' }])
  })

  it('create posts to /components', async () => {
    apiMock.onPost('/components').reply(201, { id: 'c2' })
    const result = await componentApi.create({ name: 'Header' })
    expect(result.id).toBe('c2')
  })

  it('update puts to /components/:id', async () => {
    apiMock.onPut('/components/c1').reply(200, { id: 'c1', name: 'Updated' })
    const result = await componentApi.update('c1', { name: 'Updated' })
    expect(result.name).toBe('Updated')
  })

  it('delete sends DELETE to /components/:id', async () => {
    apiMock.onDelete('/components/c1').reply(200, { success: true })
    const result = await componentApi.delete('c1')
    expect(result).toEqual({ success: true })
  })
})

describe('translationApi', () => {
  it('get fetches translation with correct query params', async () => {
    const trans = { id: 't1', data: { hello: 'world' } }
    // translationApi.get embeds locale & stage in URL string
    apiMock.onGet(/\/components\/c1\/translations/).reply(200, trans)
    const result = await translationApi.get('c1', 'en', 'draft')
    expect(result).toEqual(trans)
  })

  it('save posts translation data', async () => {
    apiMock.onPost('/components/c1/translations').reply(200, { id: 't1' })
    const result = await translationApi.save('c1', 'en', 'draft', { hello: 'world' })
    expect(result.id).toBe('t1')
  })

  it('deploy posts to /translations/deploy endpoint', async () => {
    apiMock.onPost('/components/c1/translations/deploy').reply(200, { success: true })
    const result = await translationApi.deploy('c1', 'en', 'draft', 'staging')
    expect(result).toEqual({ success: true })
  })

  it('revert posts to /translations/revert endpoint', async () => {
    // revert embeds locale & stage as query params in the URL string
    apiMock.onPost(/\/components\/c1\/translations\/revert/).reply(200, { success: true })
    const result = await translationApi.revert('c1', 'en', 'draft')
    expect(result).toEqual({ success: true })
  })

  it('autoTranslate posts to /translations/auto-translate endpoint', async () => {
    apiMock.onPost('/components/c1/translations/auto-translate').reply(200, { job_id: 'j1' })
    const result = await translationApi.autoTranslate('c1', 'en', 'fr', 'draft')
    expect(result.job_id).toBe('j1')
  })

  it('listVersions fetches translation versions', async () => {
    // listVersions embeds locale & stage as query params in URL string
    apiMock.onGet(/\/components\/c1\/translations\/versions/).reply(200, [{ version: 1 }])
    const result = await translationApi.listVersions('c1', 'en', 'draft')
    expect(result).toHaveLength(1)
  })
})

describe('tagApi', () => {
  it('listByApplication fetches tags for an application', async () => {
    apiMock.onGet('/applications/a1/tags').reply(200, [{ id: 't1', code: 'mobile' }])
    const result = await tagApi.listByApplication('a1')
    expect(result[0].code).toBe('mobile')
  })

  it('create posts to /applications/:id/tags', async () => {
    apiMock.onPost('/applications/a1/tags').reply(201, { id: 't1', code: 'mobile' })
    const result = await tagApi.create('a1', { code: 'mobile' })
    expect(result.id).toBe('t1')
  })

  it('delete sends DELETE to /tags/:id', async () => {
    apiMock.onDelete('/tags/t1').reply(200, { success: true })
    const result = await tagApi.delete('t1')
    expect(result).toEqual({ success: true })
  })

  it('update puts to /tags/:id', async () => {
    apiMock.onPut('/tags/t1').reply(200, { id: 't1', code: 'updated' })
    const result = await tagApi.update('t1', { code: 'updated' })
    expect(result.code).toBe('updated')
  })
})

describe('pageApi', () => {
  it('listByApplication fetches pages for an application', async () => {
    apiMock.onGet('/applications/a1/pages').reply(200, [{ id: 'p1', code: 'home' }])
    const result = await pageApi.listByApplication('a1')
    expect(result[0].code).toBe('home')
  })

  it('create posts to /applications/:id/pages', async () => {
    apiMock.onPost('/applications/a1/pages').reply(201, { id: 'p1', code: 'home' })
    const result = await pageApi.create('a1', { code: 'home' })
    expect(result.id).toBe('p1')
  })

  it('delete sends DELETE to /pages/:id', async () => {
    apiMock.onDelete('/pages/p1').reply(200, { success: true })
    const result = await pageApi.delete('p1')
    expect(result).toEqual({ success: true })
  })

  it('update puts to /pages/:id', async () => {
    apiMock.onPut('/pages/p1').reply(200, { id: 'p1', code: 'updated' })
    const result = await pageApi.update('p1', { code: 'updated' })
    expect(result.code).toBe('updated')
  })
})

describe('request interceptor', () => {
  it('attaches Authorization header when token is in localStorage', async () => {
    localStorage.setItem('token', 'my-jwt-token')
    let capturedHeaders: Record<string, string> = {}

    apiMock.onGet('/auth/me').reply((config) => {
      capturedHeaders = config.headers as Record<string, string>
      return [200, {}]
    })

    await authApi.getCurrentUser()
    expect(capturedHeaders['Authorization']).toBe('Bearer my-jwt-token')
  })

  it('does not set Authorization header when no token in localStorage', async () => {
    let capturedHeaders: Record<string, string> = {}

    apiMock.onGet('/auth/me').reply((config) => {
      capturedHeaders = config.headers as Record<string, string>
      return [200, {}]
    })

    await authApi.getCurrentUser()
    expect(capturedHeaders['Authorization']).toBeUndefined()
  })
})
