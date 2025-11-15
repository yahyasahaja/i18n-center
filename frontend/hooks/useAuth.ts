import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAppDispatch, useAppSelector } from './redux'
import { getCurrentUser, setToken } from '@/store/slices/authSlice'

export function useAuth(redirectToLogin = true) {
  const dispatch = useAppDispatch()
  const router = useRouter()
  const { isAuthenticated, loading, user } = useAppSelector((state) => state.auth)
  const [initializing, setInitializing] = useState(true)

  useEffect(() => {
    const initializeAuth = async () => {
      const token = localStorage.getItem('token')
      if (token) {
        try {
          dispatch(setToken(token))
          await dispatch(getCurrentUser()).unwrap()
        } catch (error) {
          // Token invalid, clear it
          localStorage.removeItem('token')
          if (redirectToLogin) {
            router.replace('/login')
          }
          setInitializing(false)
          return
        }
      } else {
        // No token
        if (redirectToLogin) {
          router.replace('/login')
        }
        setInitializing(false)
        return
      }
      setInitializing(false)
    }

    initializeAuth()
  }, [dispatch, router, redirectToLogin])

  return {
    isAuthenticated,
    loading: initializing || loading,
    user,
    initializing,
  }
}

