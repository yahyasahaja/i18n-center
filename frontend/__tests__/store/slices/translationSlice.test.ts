import { configureStore } from '@reduxjs/toolkit'
import translationReducer, {
  fetchTranslation,
  saveTranslation,
} from '@/store/slices/translationSlice'

jest.mock('@/services/api', () => ({
  translationApi: {
    get: jest.fn(),
    save: jest.fn(),
  },
}))

import { translationApi } from '@/services/api'

const mockTranslationApi = translationApi as jest.Mocked<typeof translationApi>

function makeStore() {
  return configureStore({ reducer: { translations: translationReducer } })
}

const mockTranslation = {
  id: 'trans-1',
  component_id: 'comp-1',
  locale: 'en',
  stage: 'draft',
  data: { title: 'Hello', subtitle: 'World' },
}

describe('translationSlice', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  describe('initial state', () => {
    it('has correct initial state', () => {
      const store = makeStore()
      const state = store.getState().translations
      expect(state.translations).toEqual([])
      expect(state.currentTranslation).toBeNull()
      expect(state.loading).toBe(false)
      expect(state.error).toBeNull()
    })
  })

  describe('fetchTranslation thunk', () => {
    it('sets loading=true while pending', () => {
      mockTranslationApi.get.mockReturnValue(new Promise(() => {}))
      const store = makeStore()
      store.dispatch(fetchTranslation({ componentId: 'comp-1', locale: 'en', stage: 'draft' }))
      expect(store.getState().translations.loading).toBe(true)
    })

    it('on success sets currentTranslation and clears loading', async () => {
      mockTranslationApi.get.mockResolvedValue(mockTranslation)
      const store = makeStore()
      await store.dispatch(fetchTranslation({ componentId: 'comp-1', locale: 'en', stage: 'draft' }))
      const state = store.getState().translations
      expect(state.loading).toBe(false)
      expect(state.currentTranslation).toEqual(mockTranslation)
    })

    it('calls api with correct parameters', async () => {
      mockTranslationApi.get.mockResolvedValue(mockTranslation)
      const store = makeStore()
      await store.dispatch(fetchTranslation({ componentId: 'comp-1', locale: 'fr', stage: 'staging' }))
      expect(mockTranslationApi.get).toHaveBeenCalledWith('comp-1', 'fr', 'staging')
    })
  })

  describe('saveTranslation thunk', () => {
    it('updates currentTranslation on success', async () => {
      const savedTranslation = { ...mockTranslation, data: { title: 'Updated' } }
      mockTranslationApi.save.mockResolvedValue(savedTranslation)
      const store = makeStore()
      await store.dispatch(
        saveTranslation({
          componentId: 'comp-1',
          locale: 'en',
          stage: 'draft',
          data: { title: 'Updated' },
        })
      )
      expect(store.getState().translations.currentTranslation).toEqual(savedTranslation)
    })

    it('calls api with correct parameters', async () => {
      mockTranslationApi.save.mockResolvedValue(mockTranslation)
      const store = makeStore()
      await store.dispatch(
        saveTranslation({
          componentId: 'comp-1',
          locale: 'en',
          stage: 'draft',
          data: { title: 'Hello' },
        })
      )
      expect(mockTranslationApi.save).toHaveBeenCalledWith('comp-1', 'en', 'draft', { title: 'Hello' })
    })
  })
})
