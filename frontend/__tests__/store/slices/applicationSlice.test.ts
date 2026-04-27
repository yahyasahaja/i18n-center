import { configureStore } from '@reduxjs/toolkit'
import applicationReducer, {
  fetchApplications,
  fetchApplication,
  createApplication,
  updateApplication,
} from '@/store/slices/applicationSlice'

jest.mock('@/services/api', () => ({
  applicationApi: {
    getAll: jest.fn(),
    getById: jest.fn(),
    create: jest.fn(),
    update: jest.fn(),
  },
}))

import { applicationApi } from '@/services/api'

const mockApplicationApi = applicationApi as jest.Mocked<typeof applicationApi>

function makeStore() {
  return configureStore({ reducer: { applications: applicationReducer } })
}

const mockApp = {
  id: 'app-1',
  name: 'My App',
  code: 'my-app',
  description: 'Test app',
  enabled_languages: ['en', 'id'],
}

const mockApp2 = {
  id: 'app-2',
  name: 'Second App',
  code: 'second-app',
  description: '',
  enabled_languages: ['en'],
}

describe('applicationSlice', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  describe('initial state', () => {
    it('has correct initial state', () => {
      const store = makeStore()
      const state = store.getState().applications
      expect(state.applications).toEqual([])
      expect(state.currentApplication).toBeNull()
      expect(state.loading).toBe(false)
      expect(state.error).toBeNull()
    })
  })

  describe('fetchApplications thunk', () => {
    it('sets loading=true while pending', () => {
      mockApplicationApi.getAll.mockReturnValue(new Promise(() => {}))
      const store = makeStore()
      store.dispatch(fetchApplications())
      expect(store.getState().applications.loading).toBe(true)
    })

    it('on success populates applications and clears loading', async () => {
      mockApplicationApi.getAll.mockResolvedValue([mockApp, mockApp2])
      const store = makeStore()
      await store.dispatch(fetchApplications())
      const state = store.getState().applications
      expect(state.loading).toBe(false)
      expect(state.applications).toHaveLength(2)
      expect(state.applications[0]).toEqual(mockApp)
    })
  })

  describe('fetchApplication thunk', () => {
    it('sets currentApplication on success', async () => {
      mockApplicationApi.getById.mockResolvedValue(mockApp)
      const store = makeStore()
      await store.dispatch(fetchApplication('app-1'))
      expect(store.getState().applications.currentApplication).toEqual(mockApp)
    })
  })

  describe('createApplication thunk', () => {
    it('appends new application to the list', async () => {
      mockApplicationApi.create.mockResolvedValue(mockApp)
      const store = makeStore()
      await store.dispatch(createApplication({ name: 'My App', code: 'my-app' }))
      const state = store.getState().applications
      expect(state.applications).toHaveLength(1)
      expect(state.applications[0]).toEqual(mockApp)
    })
  })

  describe('updateApplication thunk', () => {
    it('updates existing application in list', async () => {
      mockApplicationApi.getAll.mockResolvedValue([mockApp, mockApp2])
      const updated = { ...mockApp, name: 'Updated App' }
      mockApplicationApi.update.mockResolvedValue(updated)
      const store = makeStore()
      await store.dispatch(fetchApplications())
      await store.dispatch(updateApplication({ id: 'app-1', data: { name: 'Updated App' } }))
      const state = store.getState().applications
      expect(state.applications[0].name).toBe('Updated App')
    })

    it('updates currentApplication if it matches updated id', async () => {
      mockApplicationApi.getById.mockResolvedValue(mockApp)
      const updated = { ...mockApp, name: 'Updated App' }
      mockApplicationApi.update.mockResolvedValue(updated)
      const store = makeStore()
      await store.dispatch(fetchApplication('app-1'))
      await store.dispatch(updateApplication({ id: 'app-1', data: { name: 'Updated App' } }))
      expect(store.getState().applications.currentApplication?.name).toBe('Updated App')
    })
  })
})
