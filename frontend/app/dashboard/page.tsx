'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { useAuth } from '@/hooks/useAuth'
import { fetchApplications } from '@/store/slices/applicationSlice'
import { fetchComponents } from '@/store/slices/componentSlice'
import Link from 'next/link'
import { Globe, Layers, ArrowRight, Users } from 'lucide-react'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'

export default function DashboardPage() {
  const router = useRouter()
  const dispatch = useAppDispatch()
  const { isAuthenticated, user, loading } = useAuth()
  const { applications } = useAppSelector((state) => state.applications)
  const { components } = useAppSelector((state) => state.components)

  useEffect(() => {
    if (!loading && isAuthenticated) {
      dispatch(fetchApplications())
      dispatch(fetchComponents())
    }
  }, [loading, isAuthenticated, dispatch])

  if (loading || !isAuthenticated) {
    return (
      <Layout>
        <div className="flex items-center justify-center h-64">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
            <p className="mt-4 text-gray-600">Loading...</p>
          </div>
        </div>
      </Layout>
    )
  }

  const canManageUsers =
    user?.role === 'super_admin' || user?.role === 'user_manager'

  return (
    <Layout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
          <p className="mt-2 text-sm text-gray-600">
            Welcome back, {user?.username}!
          </p>
        </div>

        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
          <Card>
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Globe className="h-8 w-8 text-primary-600" />
              </div>
              <div className="ml-4 flex-1">
                <p className="text-sm font-medium text-gray-500">Applications</p>
                <p className="text-2xl font-bold text-gray-900">{applications.length}</p>
              </div>
            </div>
            <div className="mt-4">
              <Button
                variant="outline"
                size="sm"
                onClick={() => router.push('/applications')}
              >
                View all <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </div>
          </Card>

          <Card>
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Layers className="h-8 w-8 text-primary-600" />
              </div>
              <div className="ml-4 flex-1">
                <p className="text-sm font-medium text-gray-500">Components</p>
                <p className="text-2xl font-bold text-gray-900">{components.length}</p>
              </div>
            </div>
            <div className="mt-4">
              <Button
                variant="outline"
                size="sm"
                onClick={() => router.push('/components')}
              >
                View all <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </div>
          </Card>

          {canManageUsers && (
            <Card>
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <Users className="h-8 w-8 text-primary-600" />
                </div>
                <div className="ml-4 flex-1">
                  <p className="text-sm font-medium text-gray-500">User Management</p>
                  <p className="text-sm text-gray-400">Manage users and roles</p>
                </div>
              </div>
              <div className="mt-4">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => router.push('/users')}
                >
                  Manage Users <ArrowRight className="ml-2 h-4 w-4" />
                </Button>
              </div>
            </Card>
          )}
        </div>

        <Card title="Quick Actions">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Button
              variant="primary"
              onClick={() => router.push('/applications')}
              className="justify-start"
            >
              <Globe className="w-5 h-5 mr-2" />
              Manage Applications
            </Button>
            <Button
              variant="primary"
              onClick={() => router.push('/components')}
              className="justify-start"
            >
              <Layers className="w-5 h-5 mr-2" />
              Manage Components
            </Button>
          </div>
        </Card>
      </div>
    </Layout>
  )
}

