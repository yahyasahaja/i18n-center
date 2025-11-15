import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

export function middleware(request: NextRequest) {
  // Get token from Authorization header (since we're using localStorage, not cookies)
  const authHeader = request.headers.get('authorization')
  const token = authHeader?.replace('Bearer ', '') || null

  // Public routes - allow access
  if (request.nextUrl.pathname === '/login') {
    // If user has token and tries to access login, allow it (client-side will redirect)
    return NextResponse.next()
  }

  // For protected routes, we can't check localStorage in middleware
  // So we allow the request and let client-side handle auth
  // The client-side pages will check auth and redirect if needed
  return NextResponse.next()
}

export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - api (API routes)
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     */
    '/((?!api|_next/static|_next/image|favicon.ico).*)',
  ],
}

