'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { usePathname } from 'next/navigation'

export default function Home() {
  const router = useRouter()
  const pathname = usePathname()

  useEffect(() => {
    // CRITICAL: Only redirect if we're actually on the root path
    // Check both pathname and window.location to be absolutely sure
    if (pathname !== '/') {
      return
    }
    if (typeof window !== 'undefined' && window.location.pathname !== '/') {
      return
    }

    // Now we're sure we're on root, do the redirect
    const token = localStorage.getItem('token')
    if (token) {
      router.replace('/dashboard')
    } else {
      router.replace('/login')
    }
  }, [router, pathname])

  // If not on root path, return null immediately (don't render anything)
  if (pathname !== '/') {
    return null
  }
  if (typeof window !== 'undefined' && window.location.pathname !== '/') {
    return null
  }

  // Only show loading if we're actually on root
  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
        <p className="mt-4 text-gray-600">Loading...</p>
      </div>
    </div>
  )
}

