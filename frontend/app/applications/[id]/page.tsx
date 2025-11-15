'use client'

import { useEffect, useState } from 'react'
import { useRouter, useParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { fetchApplication } from '@/store/slices/applicationSlice'
import { fetchComponents } from '@/store/slices/componentSlice'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { ArrowLeft, Plus, Edit, Trash2, ArrowRight } from 'lucide-react'
import { componentApi } from '@/services/api'
import toast from 'react-hot-toast'

export default function ApplicationDetailPage() {
  const router = useRouter()
  const params = useParams()
  const applicationId = params.id as string
  const dispatch = useAppDispatch()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const { currentApplication } = useAppSelector((state) => state.applications)
  const { components } = useAppSelector((state) => state.components)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token || !isAuthenticated) {
      router.replace('/login')
      return
    }

    const loadData = async () => {
      try {
        await Promise.all([
          dispatch(fetchApplication(applicationId)),
          dispatch(fetchComponents(applicationId)),
        ])
      } catch (error: any) {
        toast.error('Failed to load application data')
      } finally {
        setLoading(false)
      }
    }

    if (applicationId) {
      loadData()
    }
  }, [applicationId, isAuthenticated, router, dispatch])

  const handleDeleteComponent = async (id: string) => {
    if (!confirm('Are you sure you want to delete this component?')) return
    try {
      await componentApi.delete(id)
      toast.success('Component deleted')
      dispatch(fetchComponents(applicationId))
    } catch (error: any) {
      toast.error('Failed to delete component')
    }
  }

  if (!isAuthenticated || loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center h-64">
          <div className="text-gray-500">Loading...</div>
        </div>
      </Layout>
    )
  }

  if (!currentApplication) {
    return (
      <Layout>
        <Card>
          <div className="text-center py-8">
            <p className="text-gray-500">Application not found</p>
            <Button
              variant="outline"
              onClick={() => router.push('/applications')}
              className="mt-4"
            >
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back to Applications
            </Button>
          </div>
        </Card>
      </Layout>
    )
  }

  const canManage = user?.role === 'super_admin' || user?.role === 'operator'
  const applicationComponents = components.filter(
    (c) => c.application_id === applicationId
  )

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-4">
            <Button variant="outline" onClick={() => router.push('/applications')}>
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back
            </Button>
            <div>
              <h1 className="text-3xl font-bold text-gray-900">
                {currentApplication.name}
              </h1>
              <p className="text-sm text-gray-500 mt-1">
                {currentApplication.description || 'No description'}
              </p>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Card>
            <div className="text-sm font-medium text-gray-500">Components</div>
            <div className="mt-2 text-3xl font-bold text-gray-900">
              {applicationComponents.length}
            </div>
          </Card>
          <Card>
            <div className="text-sm font-medium text-gray-500">Enabled Languages</div>
            <div className="mt-2 flex flex-wrap gap-2">
              {currentApplication.enabled_languages?.map((lang) => (
                <Badge key={lang} variant="info">
                  {lang.toUpperCase()}
                </Badge>
              )) || <span className="text-gray-400">None</span>}
            </div>
          </Card>
          <Card>
            <div className="text-sm font-medium text-gray-500">OpenAI Key</div>
            <div className="mt-2 text-sm text-gray-900">
              {currentApplication.has_openai_key ? (
                <Badge variant="success">Configured</Badge>
              ) : (
                <Badge variant="warning">Not configured</Badge>
              )}
            </div>
          </Card>
        </div>

        <Card
          title="Components"
          actions={
            canManage && (
              <Button
                variant="primary"
                size="sm"
                onClick={() => router.push(`/components?application_id=${applicationId}`)}
              >
                <Plus className="w-4 h-4 mr-2" />
                Add Component
              </Button>
            )
          }
        >
          {applicationComponents.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              No components yet. Create one to get started.
            </div>
          ) : (
            <Table headers={['Name', 'Code', 'Description', 'Default Locale', 'Actions']}>
              {applicationComponents.map((component) => (
                <TableRow key={component.id}>
                  <TableCell>
                    <div className="font-medium text-gray-900">{component.name}</div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm font-mono text-gray-600">{component.code}</div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm text-gray-500">
                      {component.description || 'No description'}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="info">{component.default_locale}</Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center space-x-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          router.push(`/components/${component.id}/translations`)
                        }
                      >
                        <ArrowRight className="w-4 h-4 mr-1" />
                        Translations
                      </Button>
                      {canManage && (
                        <>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() =>
                              router.push(`/components?edit=${component.id}`)
                            }
                          >
                            <Edit className="w-4 h-4" />
                          </Button>
                          <Button
                            variant="danger"
                            size="sm"
                            onClick={() => handleDeleteComponent(component.id)}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </Table>
          )}
        </Card>
      </div>
    </Layout>
  )
}

