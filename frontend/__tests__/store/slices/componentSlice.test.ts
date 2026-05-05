import { configureStore } from '@reduxjs/toolkit'
import componentReducer, {
  fetchComponents,
  fetchComponent,
  createComponent,
  updateComponent,
} from '@/store/slices/componentSlice'
import type { ComponentListResponse } from '@/services/api'

jest.mock('@/services/api', () => ({
  componentApi: {
    getAll: jest.fn(),
    getById: jest.fn(),
    create: jest.fn(),
    update: jest.fn(),
  },
}))

import { componentApi } from '@/services/api'

const mockComponentApi = componentApi as jest.Mocked<typeof componentApi>

function makeStore() {
  return configureStore({ reducer: { components: componentReducer } })
}

const mockComponent = {
  id: 'comp-1',
  application_id: 'app-1',
  name: 'Header',
  code: 'header',
  description: 'Header component',
  structure: { title: '' },
  default_locale: 'en',
}

const mockComponent2 = {
  id: 'comp-2',
  application_id: 'app-1',
  name: 'Footer',
  code: 'footer',
  description: '',
  structure: { text: '' },
  default_locale: 'en',
}

const makePagedResponse = (items: any[]): ComponentListResponse => ({
  data: items,
  total: items.length,
  page: 1,
  page_size: 20,
  total_pages: 1,
})

describe('componentSlice', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  describe('initial state', () => {
    it('has correct initial state', () => {
      const store = makeStore()
      const state = store.getState().components
      expect(state.components).toEqual([])
      expect(state.currentComponent).toBeNull()
      expect(state.loading).toBe(false)
      expect(state.error).toBeNull()
    })
  })

  describe('fetchComponents thunk', () => {
    it('sets loading=true while pending', () => {
      mockComponentApi.getAll.mockReturnValue(new Promise(() => {}))
      const store = makeStore()
      store.dispatch(fetchComponents())
      expect(store.getState().components.loading).toBe(true)
    })

    it('on success populates components list', async () => {
      mockComponentApi.getAll.mockResolvedValue(makePagedResponse([mockComponent, mockComponent2]))
      const store = makeStore()
      await store.dispatch(fetchComponents())
      const state = store.getState().components
      expect(state.loading).toBe(false)
      expect(state.components).toHaveLength(2)
      expect(state.total).toBe(2)
    })

    it('passes params to api when provided', async () => {
      mockComponentApi.getAll.mockResolvedValue(makePagedResponse([mockComponent]))
      const store = makeStore()
      await store.dispatch(fetchComponents({ applicationId: 'app-1' }))
      expect(mockComponentApi.getAll).toHaveBeenCalledWith({ applicationId: 'app-1' })
    })
  })

  describe('fetchComponent thunk', () => {
    it('sets currentComponent on success', async () => {
      mockComponentApi.getById.mockResolvedValue(mockComponent)
      const store = makeStore()
      await store.dispatch(fetchComponent('comp-1'))
      expect(store.getState().components.currentComponent).toEqual(mockComponent)
    })
  })

  describe('createComponent thunk', () => {
    it('appends new component to list', async () => {
      mockComponentApi.create.mockResolvedValue(mockComponent)
      const store = makeStore()
      await store.dispatch(createComponent({ name: 'Header', code: 'header', application_id: 'app-1' }))
      const state = store.getState().components
      expect(state.components).toHaveLength(1)
      expect(state.components[0].code).toBe('header')
    })
  })

  describe('updateComponent thunk', () => {
    it('replaces existing component in list', async () => {
      mockComponentApi.getAll.mockResolvedValue(makePagedResponse([mockComponent, mockComponent2]))
      const updated = { ...mockComponent, name: 'Updated Header' }
      mockComponentApi.update.mockResolvedValue(updated)
      const store = makeStore()
      await store.dispatch(fetchComponents())
      await store.dispatch(updateComponent({ id: 'comp-1', data: { name: 'Updated Header' } }))
      const state = store.getState().components
      expect(state.components[0].name).toBe('Updated Header')
    })

    it('updates currentComponent if id matches', async () => {
      mockComponentApi.getById.mockResolvedValue(mockComponent)
      const updated = { ...mockComponent, name: 'Updated Header' }
      mockComponentApi.update.mockResolvedValue(updated)
      const store = makeStore()
      await store.dispatch(fetchComponent('comp-1'))
      await store.dispatch(updateComponent({ id: 'comp-1', data: { name: 'Updated Header' } }))
      expect(store.getState().components.currentComponent?.name).toBe('Updated Header')
    })
  })
})
