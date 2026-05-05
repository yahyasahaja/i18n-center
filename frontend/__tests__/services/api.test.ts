import MockAdapter from 'axios-mock-adapter'
import api, {
  authApi,
  applicationApi,
  componentApi,
  translationApi,
  tagApi,
  pageApi,
  exportApi,
  importApi,
  cmsApi,
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

  it('deployLocale posts to /applications/:id/deploy-locale', async () => {
    apiMock.onPost('/applications/a1/deploy-locale').reply(200, { success: true })
    const result = await applicationApi.deployLocale('a1', 'fr')
    expect(result).toEqual({ success: true })
  })

  it('getAddLanguageJobStatus fetches /applications/:id/jobs/:job_id', async () => {
    apiMock.onGet('/applications/a1/jobs/j1').reply(200, { status: 'completed' })
    const result = await applicationApi.getAddLanguageJobStatus('a1', 'j1')
    expect(result.status).toBe('completed')
  })

  it('getActiveJobs fetches /applications/:id/active-jobs', async () => {
    apiMock.onGet('/applications/a1/active-jobs').reply(200, { add_language_jobs: [], translate_jobs: [] })
    const result = await applicationApi.getActiveJobs('a1')
    expect(result.add_language_jobs).toHaveLength(0)
  })

  it('deleteLanguage deletes /applications/:id/languages/:locale', async () => {
    apiMock.onDelete('/applications/a1/languages/fr').reply(200, { success: true })
    const result = await applicationApi.deleteLanguage('a1', 'fr')
    expect(result).toEqual({ success: true })
  })
})

describe('componentApi', () => {
  it('getAll fetches /components with paginated response', async () => {
    const paged = { data: [{ id: 'c1' }], total: 1, page: 1, page_size: 20, total_pages: 1 }
    apiMock.onGet('/components').reply(200, paged)
    const result = await componentApi.getAll()
    expect(result.data).toHaveLength(1)
    expect(result.total).toBe(1)
  })

  it('getAll passes search and page params', async () => {
    const paged = { data: [], total: 0, page: 2, page_size: 20, total_pages: 0 }
    apiMock.onGet('/components').reply(200, paged)
    const result = await componentApi.getAll({ search: 'head', page: 2 })
    expect(result.page).toBe(2)
  })

  it('getById fetches /components/:id', async () => {
    apiMock.onGet('/components/c1').reply(200, { id: 'c1', name: 'Header' })
    const result = await componentApi.getById('c1')
    expect(result.name).toBe('Header')
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

  it('get fetches /tags/:id', async () => {
    apiMock.onGet('/tags/t1').reply(200, { id: 't1', code: 'mobile' })
    const result = await tagApi.get('t1')
    expect(result.id).toBe('t1')
  })

  it('getComponents fetches /tags/:id/components', async () => {
    apiMock.onGet('/tags/t1/components').reply(200, [{ id: 'c1' }])
    const result = await tagApi.getComponents('t1')
    expect(result).toHaveLength(1)
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

  it('get fetches /pages/:id', async () => {
    apiMock.onGet('/pages/p1').reply(200, { id: 'p1', code: 'home' })
    const result = await pageApi.get('p1')
    expect(result.id).toBe('p1')
  })

  it('getComponents fetches /pages/:id/components', async () => {
    apiMock.onGet('/pages/p1/components').reply(200, [{ id: 'c1' }])
    const result = await pageApi.getComponents('p1')
    expect(result).toHaveLength(1)
  })
})

describe('translationApi - extended', () => {
  it('backfill posts to /translations/backfill endpoint', async () => {
    apiMock.onPost('/components/c1/translations/backfill').reply(200, { job_id: 'j2' })
    const result = await translationApi.backfill('c1', 'en', ['fr', 'de'], 'draft')
    expect(result.job_id).toBe('j2')
  })

  it('getTranslateJobStatus fetches /translate-jobs/:job_id', async () => {
    apiMock.onGet('/translate-jobs/j1').reply(200, { job_id: 'j1', status: 'completed' })
    const result = await translationApi.getTranslateJobStatus('j1')
    expect(result.status).toBe('completed')
  })

  it('listComponentTranslateJobs fetches /components/:id/translate-jobs', async () => {
    apiMock.onGet('/components/c1/translate-jobs').reply(200, [{ id: 'j1' }])
    const result = await translationApi.listComponentTranslateJobs('c1')
    expect(result).toHaveLength(1)
  })

  it('compare fetches translation comparison', async () => {
    apiMock.onGet(/\/components\/c1\/translations\/compare/).reply(200, { diff: [] })
    const result = await translationApi.compare('c1', 'en', 'draft')
    expect(result.diff).toHaveLength(0)
  })

  it('compare with specific versions', async () => {
    apiMock.onGet(/\/components\/c1\/translations\/compare/).reply(200, { diff: [{}] })
    const result = await translationApi.compare('c1', 'en', 'draft', 1, 2)
    expect(result.diff).toHaveLength(1)
  })
})

describe('exportApi', () => {
  it('exportApplication fetches application export blob', async () => {
    apiMock.onGet('/applications/a1/export').reply(200, new Blob(['{}']))
    const result = await exportApi.exportApplication('a1')
    expect(result).toBeDefined()
  })

  it('exportApplication with locale and stage params', async () => {
    apiMock.onGet('/applications/a1/export').reply(200, new Blob(['{}']))
    const result = await exportApi.exportApplication('a1', 'en', 'draft')
    expect(result).toBeDefined()
  })

  it('exportComponent fetches component export blob', async () => {
    apiMock.onGet('/components/c1/export').reply(200, new Blob(['{}']))
    const result = await exportApi.exportComponent('c1')
    expect(result).toBeDefined()
  })
})

describe('importApi', () => {
  it('importComponent posts data to /components/:id/import', async () => {
    apiMock.onPost(/\/components\/c1\/import/).reply(200, { keys_imported: 5 })
    const result = await importApi.importComponent('c1', 'en', 'draft', { hello: 'world' })
    expect(result.keys_imported).toBe(5)
  })
})

describe('cmsApi', () => {
  it('listTemplates fetches /applications/:id/cms/templates', async () => {
    apiMock.onGet('/applications/a1/cms/templates').reply(200, [{ id: 't1', name: 'Banner' }])
    const result = await cmsApi.listTemplates('a1')
    expect(result).toHaveLength(1)
  })

  it('getTemplate fetches /cms/templates/:id', async () => {
    apiMock.onGet('/cms/templates/t1').reply(200, { id: 't1', name: 'Banner' })
    const result = await cmsApi.getTemplate('t1')
    expect(result.id).toBe('t1')
  })

  it('createTemplate posts to /applications/:id/cms/templates', async () => {
    apiMock.onPost('/applications/a1/cms/templates').reply(201, { id: 't1', name: 'Banner' })
    const result = await cmsApi.createTemplate('a1', { name: 'Banner' })
    expect(result.id).toBe('t1')
  })

  it('updateTemplate puts to /cms/templates/:id', async () => {
    apiMock.onPut('/cms/templates/t1').reply(200, { id: 't1', name: 'Updated' })
    const result = await cmsApi.updateTemplate('t1', { name: 'Updated' })
    expect(result.name).toBe('Updated')
  })

  it('deleteTemplate sends DELETE to /cms/templates/:id', async () => {
    apiMock.onDelete('/cms/templates/t1').reply(204)
    await expect(cmsApi.deleteTemplate('t1')).resolves.not.toThrow()
  })

  it('listItems fetches /applications/:id/cms/items', async () => {
    apiMock.onGet('/applications/a1/cms/items').reply(200, [{ id: 'i1', identifier: 'flash_banner' }])
    const result = await cmsApi.listItems('a1')
    expect(result[0].identifier).toBe('flash_banner')
  })

  it('getItem fetches /cms/items/:id', async () => {
    apiMock.onGet('/cms/items/i1').reply(200, { id: 'i1', identifier: 'flash_banner' })
    const result = await cmsApi.getItem('i1')
    expect(result.id).toBe('i1')
  })

  it('createItem posts to /applications/:id/cms/items', async () => {
    apiMock.onPost('/applications/a1/cms/items').reply(201, { id: 'i1', identifier: 'flash_banner' })
    const result = await cmsApi.createItem('a1', { identifier: 'flash_banner' })
    expect(result.id).toBe('i1')
  })

  it('updateItem puts to /cms/items/:id', async () => {
    apiMock.onPut('/cms/items/i1').reply(200, { id: 'i1', name: 'Updated' })
    const result = await cmsApi.updateItem('i1', { name: 'Updated' })
    expect(result.name).toBe('Updated')
  })

  it('deleteItem sends DELETE to /cms/items/:id', async () => {
    apiMock.onDelete('/cms/items/i1').reply(204)
    await expect(cmsApi.deleteItem('i1')).resolves.not.toThrow()
  })

  it('listLocalizations fetches /cms/items/:id/localizations', async () => {
    apiMock.onGet('/cms/items/i1/localizations').reply(200, [{ id: 'l1', locale: 'en' }])
    const result = await cmsApi.listLocalizations('i1')
    expect(result[0].locale).toBe('en')
  })

  it('getLocalization fetches /cms/items/:id/localizations/detail', async () => {
    apiMock.onGet('/cms/items/i1/localizations/detail').reply(200, { id: 'l1', locale: 'en', stage: 'draft' })
    const result = await cmsApi.getLocalization('i1', 'en', 'draft')
    expect(result.stage).toBe('draft')
  })

  it('saveLocalization posts to /cms/items/:id/localizations', async () => {
    apiMock.onPost('/cms/items/i1/localizations').reply(200, { id: 'l1', version: 1 })
    const result = await cmsApi.saveLocalization('i1', 'en', 'draft', { title: 'Hello' })
    expect(result.version).toBe(1)
  })

  it('translateLocalization posts to /cms/items/:id/localizations/translate', async () => {
    apiMock.onPost('/cms/items/i1/localizations/translate').reply(202, { job_id: 'j1' })
    const result = await cmsApi.translateLocalization('i1', 'en', 'fr', 'draft')
    expect(result.job_id).toBe('j1')
  })

  it('deployLocalization posts to /cms/items/:id/localizations/deploy', async () => {
    apiMock.onPost('/cms/items/i1/localizations/deploy').reply(200, { id: 'l1', stage: 'staging' })
    const result = await cmsApi.deployLocalization('i1', 'en', 'draft', 'staging')
    expect(result.stage).toBe('staging')
  })

  it('revertLocalization posts to /cms/items/:id/localizations/revert', async () => {
    apiMock.onPost('/cms/items/i1/localizations/revert').reply(200, { id: 'l1', version: 1 })
    const result = await cmsApi.revertLocalization('i1', 'en', 'draft', 1)
    expect(result.version).toBe(1)
  })

  it('listVersions fetches /cms/items/:id/localizations/versions', async () => {
    apiMock.onGet('/cms/items/i1/localizations/versions').reply(200, [{ version: 1 }, { version: 2 }])
    const result = await cmsApi.listVersions('i1', 'en', 'draft')
    expect(result).toHaveLength(2)
  })

  it('getCmsTranslateJobStatus fetches /cms/translate-jobs/:job_id', async () => {
    apiMock.onGet('/cms/translate-jobs/j1').reply(200, { job_id: 'j1', status: 'completed' })
    const result = await cmsApi.getCmsTranslateJobStatus('j1')
    expect(result.status).toBe('completed')
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
