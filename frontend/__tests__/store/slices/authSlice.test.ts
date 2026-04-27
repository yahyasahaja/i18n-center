import { configureStore } from '@reduxjs/toolkit'
import authReducer, {
  login,
  getCurrentUser,
  logout,
  setToken,
} from '@/store/slices/authSlice'

// Mock the authApi
jest.mock('@/services/api', () => ({
  authApi: {
    login: jest.fn(),
    getCurrentUser: jest.fn(),
  },
}))

import { authApi } from '@/services/api'

const mockAuthApi = authApi as jest.Mocked<typeof authApi>

function makeStore() {
  return configureStore({ reducer: { auth: authReducer } })
}

const mockUser = { id: '1', username: 'admin', role: 'super_admin', is_active: true }

describe('authSlice', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    localStorage.clear()
  })

  describe('initial state', () => {
    it('has correct initial state', () => {
      const store = makeStore()
      const state = store.getState().auth
      expect(state.user).toBeNull()
      expect(state.token).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(state.loading).toBe(false)
      expect(state.error).toBeNull()
    })
  })

  describe('logout action', () => {
    it('clears user, token and isAuthenticated', () => {
      const store = makeStore()
      store.dispatch(setToken('abc123'))
      store.dispatch(logout())
      const state = store.getState().auth
      expect(state.user).toBeNull()
      expect(state.token).toBeNull()
      expect(state.isAuthenticated).toBe(false)
    })

    it('removes token from localStorage', () => {
      localStorage.setItem('token', 'abc123')
      const store = makeStore()
      store.dispatch(logout())
      expect(localStorage.getItem('token')).toBeNull()
    })
  })

  describe('setToken action', () => {
    it('sets token and marks isAuthenticated', () => {
      const store = makeStore()
      store.dispatch(setToken('mytoken'))
      const state = store.getState().auth
      expect(state.token).toBe('mytoken')
      expect(state.isAuthenticated).toBe(true)
    })
  })

  describe('login thunk', () => {
    it('sets loading=true while pending', () => {
      mockAuthApi.login.mockReturnValue(new Promise(() => {}))
      const store = makeStore()
      store.dispatch(login({ username: 'admin', password: 'pass' }))
      const state = store.getState().auth
      expect(state.loading).toBe(true)
      expect(state.error).toBeNull()
    })

    it('on success stores user, token and isAuthenticated', async () => {
      const response = { user: mockUser, token: 'token123' }
      mockAuthApi.login.mockResolvedValue(response)
      const store = makeStore()
      await store.dispatch(login({ username: 'admin', password: 'pass' }))
      const state = store.getState().auth
      expect(state.loading).toBe(false)
      expect(state.user).toEqual(mockUser)
      expect(state.token).toBe('token123')
      expect(state.isAuthenticated).toBe(true)
    })

    it('stores token in localStorage on success', async () => {
      mockAuthApi.login.mockResolvedValue({ user: mockUser, token: 'tok' })
      const store = makeStore()
      await store.dispatch(login({ username: 'admin', password: 'pass' }))
      expect(localStorage.getItem('token')).toBe('tok')
    })

    it('on failure sets error and clears loading', async () => {
      mockAuthApi.login.mockRejectedValue(new Error('Invalid credentials'))
      const store = makeStore()
      await store.dispatch(login({ username: 'admin', password: 'wrong' }))
      const state = store.getState().auth
      expect(state.loading).toBe(false)
      expect(state.error).toBe('Invalid credentials')
      expect(state.isAuthenticated).toBe(false)
    })
  })

  describe('getCurrentUser thunk', () => {
    it('on success sets user and isAuthenticated', async () => {
      mockAuthApi.getCurrentUser.mockResolvedValue(mockUser)
      const store = makeStore()
      await store.dispatch(getCurrentUser())
      const state = store.getState().auth
      expect(state.user).toEqual(mockUser)
      expect(state.isAuthenticated).toBe(true)
    })
  })
})
