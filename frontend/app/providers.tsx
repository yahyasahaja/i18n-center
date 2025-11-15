'use client'

import { useEffect } from 'react'
import { Provider, useDispatch } from 'react-redux'
import { store } from '@/store/store'
import { Toaster } from 'react-hot-toast'
import { getCurrentUser, setToken } from '@/store/slices/authSlice'

// Component to initialize auth on app load
function AuthInitializer({ children }: { children: React.ReactNode }) {
  const dispatch = useDispatch()

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (token) {
      // Initialize auth state from localStorage
      dispatch(setToken(token))
      // Fetch user data in background (don't block render)
      dispatch(getCurrentUser())
    }
  }, [dispatch])

  return <>{children}</>
}

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <Provider store={store}>
      <AuthInitializer>
        {children}
        <Toaster position="top-right" />
      </AuthInitializer>
    </Provider>
  )
}

