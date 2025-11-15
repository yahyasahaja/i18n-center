'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import {
  fetchApplications,
  createApplication,
  updateApplication,
} from '@/store/slices/applicationSlice'
import { applicationApi } from '@/services/api'
import toast from 'react-hot-toast'
import { Plus, Edit, Trash2, ArrowRight, Globe } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Badge } from '@/components/ui/Badge'

export default function ApplicationsPage() {
  const router = useRouter()
  const dispatch = useAppDispatch()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const { applications, loading } = useAppSelector(
    (state) => state.applications
  )
  const [showModal, setShowModal] = useState(false)
  const [editingApp, setEditingApp] = useState<any>(null)
  const [formData, setFormData] = useState({
    name: '',
    code: '',
    description: '',
    enabled_languages: [] as string[],
    enabled_languages_input: '', // Raw input string for comma-separated languages
    openai_key: '', // For setting/updating the key
  })

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      router.replace('/login')
      return
    }

    // Don't check isAuthenticated immediately - it might not be initialized yet
    // Just check token and proceed with loading
    dispatch(fetchApplications())
  }, [router, dispatch])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      // Convert enabled_languages_input to array
      const enabledLanguages = formData.enabled_languages_input
        .split(',')
        .map((l) => l.trim())
        .filter((l) => l)

      const submitData = {
        name: formData.name,
        code: formData.code,
        description: formData.description,
        enabled_languages: enabledLanguages,
        openai_key: formData.openai_key,
      }

      if (editingApp) {
        await dispatch(
          updateApplication({ id: editingApp.id, data: submitData })
        ).unwrap()
        toast.success('Application updated')
      } else {
        await dispatch(createApplication(submitData)).unwrap()
        toast.success('Application created')
      }
      setShowModal(false)
      setEditingApp(null)
      setFormData({
        name: '',
        code: '',
        description: '',
        enabled_languages: [],
        enabled_languages_input: '',
        openai_key: '', // Optional - for setting OpenAI key
      })
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to save application')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this application?')) return
    try {
      await applicationApi.delete(id)
      toast.success('Application deleted')
      dispatch(fetchApplications())
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to delete application')
    }
  }

  const handleEdit = (app: any) => {
    setEditingApp(app)
    setFormData({
      name: app.name,
      code: app.code || '',
      description: app.description || '',
      enabled_languages: app.enabled_languages || [],
      enabled_languages_input: (app.enabled_languages || []).join(', '),
      openai_key: '', // Leave empty - user can set new key if needed
    })
    setShowModal(true)
  }

  if (!isAuthenticated) return null

  const canManage =
    user?.role === 'super_admin' || user?.role === 'operator'

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-gray-900">Applications</h1>
          {canManage && (
            <Button
              variant="primary"
              onClick={() => {
                setEditingApp(null)
                setFormData({
                  name: '',
                  code: '',
                  description: '',
                  enabled_languages: [],
                  enabled_languages_input: '',
                  openai_key: '',
                })
                setShowModal(true)
              }}
            >
              <Plus className="w-4 h-4 mr-2" />
              New Application
            </Button>
          )}
        </div>

        {loading ? (
          <Card>
            <div className="text-center py-8 text-gray-500">Loading...</div>
          </Card>
        ) : applications.length === 0 ? (
          <Card>
            <div className="text-center py-12">
              <Globe className="mx-auto h-12 w-12 text-gray-400" />
              <h3 className="mt-2 text-sm font-medium text-gray-900">No applications</h3>
              <p className="mt-1 text-sm text-gray-500">
                Get started by creating a new application.
              </p>
              {canManage && (
                <div className="mt-6">
                  <Button
                    variant="primary"
                    onClick={() => {
                      setEditingApp(null)
                      setFormData({
                        name: '',
                        code: '',
                        description: '',
                        enabled_languages: [],
                        enabled_languages_input: '',
                        openai_key: '',
                      })
                      setShowModal(true)
                    }}
                  >
                    <Plus className="w-4 h-4 mr-2" />
                    New Application
                  </Button>
                </div>
              )}
            </div>
          </Card>
        ) : (
          <Card>
            <Table headers={['Name', 'Code', 'Description', 'Languages', 'OpenAI', 'Actions']}>
              {applications.map((app) => (
                <TableRow key={app.id}>
                  <TableCell>
                    <div className="font-medium text-gray-900">{app.name}</div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm font-mono text-gray-600">{app.code}</div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm text-gray-500 max-w-md">
                      {app.description || 'No description'}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {app.enabled_languages?.length > 0 ? (
                        app.enabled_languages.map((lang) => (
                          <Badge key={lang} variant="info" size="sm">
                            {lang.toUpperCase()}
                          </Badge>
                        ))
                      ) : (
                        <span className="text-xs text-gray-400">None</span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    {app.has_openai_key ? (
                      <Badge variant="success" size="sm">Configured</Badge>
                    ) : (
                      <Badge variant="warning" size="sm">Not set</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center space-x-2">
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => router.push(`/applications/${app.id}`)}
                      >
                        <ArrowRight className="w-4 h-4 mr-1" />
                        View
                      </Button>
                      {canManage && (
                        <>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleEdit(app)}
                          >
                            <Edit className="w-4 h-4" />
                          </Button>
                          {user?.role === 'super_admin' && (
                            <Button
                              variant="danger"
                              size="sm"
                              onClick={() => handleDelete(app.id)}
                            >
                              <Trash2 className="w-4 h-4" />
                            </Button>
                          )}
                        </>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </Table>
          </Card>
        )}

        <Modal
          isOpen={showModal}
          onClose={() => {
            setShowModal(false)
            setEditingApp(null)
          }}
          title={editingApp ? 'Edit Application' : 'Create Application'}
          footer={
            <>
              <Button variant="primary" onClick={handleSubmit} form="app-form">
                {editingApp ? 'Update' : 'Create'}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setShowModal(false)
                  setEditingApp(null)
                }}
              >
                Cancel
              </Button>
            </>
          }
        >
          <form id="app-form" onSubmit={handleSubmit} className="space-y-4">
            <Input
              label="Name"
              required
              value={formData.name}
              onChange={(e) =>
                setFormData({ ...formData, name: e.target.value })
              }
              helperText="Display name for the application"
            />
            <Input
              label="Code"
              required
              value={formData.code}
              onChange={(e) =>
                setFormData({ ...formData, code: e.target.value })
              }
              helperText="Unique code identifier (e.g., whatsapp, web-app). Used in API calls."
              placeholder="e.g., whatsapp"
            />
            <Textarea
              label="Description"
              value={formData.description}
              onChange={(e) =>
                setFormData({ ...formData, description: e.target.value })
              }
              rows={3}
            />
            <Input
              label="Enabled Languages"
              value={formData.enabled_languages_input}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  enabled_languages_input: e.target.value,
                })
              }
              helperText="Comma-separated list of language codes (e.g., en, id, es, fr)"
            />
            <Input
              label="OpenAI API Key"
              type="password"
              value={formData.openai_key}
              onChange={(e) =>
                setFormData({ ...formData, openai_key: e.target.value })
              }
              helperText={
                editingApp
                  ? 'Leave empty to keep existing key, or enter new key to update'
                  : 'Optional: Set OpenAI API key for auto-translation features'
              }
            />
          </form>
        </Modal>
      </div>
    </Layout>
  )
}
