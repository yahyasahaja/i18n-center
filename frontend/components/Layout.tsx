'use client'

import { useEffect } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { logout } from '@/store/slices/authSlice'
import { fetchApplications } from '@/store/slices/applicationSlice'
import Link from 'next/link'
import { LogOut, Home, Globe, Layers, FileText, Tag, Users } from 'lucide-react'
import { Button } from './ui/Button'
import { Badge } from './ui/Badge'
import { clsx } from 'clsx'
import { useAppContext, isValidStage, type Stage } from '@/context/AppContext'

const STAGE_OPTIONS: { value: Stage; label: string }[] = [
  { value: 'draft', label: 'Draft' },
  { value: 'staging', label: 'Staging' },
  { value: 'production', label: 'Production' },
]

function LayoutInner({ children }: { children: React.ReactNode }) {
  const dispatch = useAppDispatch()
  const router = useRouter()
  const pathname = usePathname()
  const { user } = useAppSelector((state) => state.auth)
  const { applications } = useAppSelector((state) => state.applications)
  const { applicationId, stage, setApplicationId, setStage, buildHref } = useAppContext()

  useEffect(() => {
    dispatch(fetchApplications())
  }, [dispatch])

  // Auto-select first application when none is set
  useEffect(() => {
    if (applications.length > 0 && !applicationId) {
      setApplicationId(applications[0].id)
    }
  }, [applications, applicationId, setApplicationId])

  const handleLogout = () => {
    dispatch(logout())
    router.push('/login')
  }

  const APPLICATION_CONTEXT_ROUTES = ['/components', '/tags', '/pages']

  const navigation = [
    { name: 'Dashboard', href: '/dashboard', icon: Home },
    { name: 'Applications', href: '/applications', icon: Globe },
    { name: 'Components', href: '/components', icon: Layers },
    { name: 'Tags', href: '/tags', icon: Tag },
    { name: 'Pages', href: '/pages', icon: FileText },
    { name: 'Users', href: '/users', icon: Users, roles: ['super_admin', 'user_manager'] as const },
  ]

  const isActive = (href: string) => pathname?.startsWith(href)

  // Disable Components, Tags, Pages only when no application is selected
  const needsApplication = (href: string) => APPLICATION_CONTEXT_ROUTES.includes(href)
  const linkHref = buildHref

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Sidebar */}
      <div className="hidden md:flex md:w-64 md:flex-col md:fixed md:inset-y-0">
        <div className="flex-1 flex flex-col min-h-0 bg-white border-r border-gray-200">
          <div className="flex-1 flex flex-col pt-5 pb-4 overflow-y-auto">
            <div className="flex items-center flex-shrink-0 px-4">
              <h1 className="text-2xl font-bold text-primary-600">i18n Center</h1>
            </div>

            {/* Application & Stage context */}
            <div className="px-3 mt-6 space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Application</label>
                <select
                  value={applicationId || ''}
                  onChange={(e) => setApplicationId(e.target.value || null)}
                  className="block w-full rounded-md border border-gray-300 bg-white py-1.5 px-2 text-sm text-gray-900 focus:border-primary-500 focus:ring-primary-500"
                >
                  <option value="">— Select —</option>
                  {applications.map((app) => (
                    <option key={app.id} value={app.id}>
                      {app.name}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Environment</label>
                <select
                  value={stage}
                  onChange={(e) => {
                    const v = e.target.value
                    if (isValidStage(v)) setStage(v)
                  }}
                  className="block w-full rounded-md border border-gray-300 bg-white py-1.5 px-2 text-sm text-gray-900 focus:border-primary-500 focus:ring-primary-500"
                >
                  {STAGE_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <nav className="mt-6 flex-1 px-2 space-y-1">
              {navigation.map((item) => {
                if (item.roles && !item.roles.includes(user?.role || '')) return null
                const Icon = item.icon
                const disabled = needsApplication(item.href) && !applicationId
                const baseClass = clsx(
                  'group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors',
                  isActive(item.href)
                    ? 'bg-primary-50 text-primary-600'
                    : 'text-gray-700 hover:bg-gray-50 hover:text-gray-900',
                  disabled && 'opacity-50 cursor-not-allowed pointer-events-none'
                )
                if (disabled) {
                  return (
                    <span key={item.name} className={baseClass} title="Select an application first">
                      <Icon className="mr-3 flex-shrink-0 h-5 w-5 text-gray-400" />
                      {item.name}
                    </span>
                  )
                }
                return (
                  <Link
                    key={item.name}
                    href={linkHref(item.href)}
                    className={baseClass}
                  >
                    <Icon
                      className={clsx(
                        'mr-3 flex-shrink-0 h-5 w-5',
                        isActive(item.href) ? 'text-primary-600' : 'text-gray-400 group-hover:text-gray-500'
                      )}
                    />
                    {item.name}
                  </Link>
                )
              })}
            </nav>
          </div>
          <div className="flex-shrink-0 flex border-t border-gray-200 p-4">
            <div className="flex-shrink-0 w-full group block">
              <div className="flex items-center">
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-gray-900 truncate">{user?.username}</p>
                  <p className="text-xs text-gray-500 truncate">
                    <Badge variant="info" size="sm">
                      {user?.role?.replace('_', ' ')}
                    </Badge>
                  </p>
                </div>
                <Button variant="outline" size="sm" onClick={handleLogout}>
                  <LogOut className="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="md:pl-64 flex flex-col flex-1">
        {/* Top bar for mobile */}
        <div className="sticky top-0 z-10 md:hidden pl-1 pt-1 sm:pl-3 sm:pt-3 bg-gray-50">
          <div className="flex items-center justify-between px-4 py-2 bg-white border-b border-gray-200">
            <h1 className="text-xl font-bold text-primary-600">i18n Center</h1>
            <Button variant="outline" size="sm" onClick={handleLogout}>
              <LogOut className="w-4 h-4" />
            </Button>
          </div>
        </div>

        <main className="flex-1">
          <div className="py-6">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 md:px-8">{children}</div>
          </div>
        </main>
      </div>
    </div>
  )
}

export default function Layout({ children }: { children: React.ReactNode }) {
  return <LayoutInner>{children}</LayoutInner>
}
