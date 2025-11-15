'use client'

import { useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import {
  fetchComponents,
  createComponent,
  updateComponent,
} from '@/store/slices/componentSlice'
import { fetchApplications } from '@/store/slices/applicationSlice'
import { componentApi } from '@/services/api'
import toast from 'react-hot-toast'
import { Plus, Edit, Trash2, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Select } from '@/components/ui/Select'
import { Badge } from '@/components/ui/Badge'

export default function ComponentsPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const dispatch = useAppDispatch()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const { components, loading } = useAppSelector((state) => state.components)
  const { applications } = useAppSelector((state) => state.applications)
  const [showModal, setShowModal] = useState(false)
  const [editingComponent, setEditingComponent] = useState<any>(null)
  const [filterApplication, setFilterApplication] = useState<string>('')
  const [formData, setFormData] = useState({
    application_id: searchParams.get('application_id') || '',
    name: '',
    code: '',
    description: '',
    structure: {} as Record<string, any>,
    default_locale: 'en',
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
    dispatch(fetchComponents())
  }, [router, dispatch])

  useEffect(() => {
    const editId = searchParams.get('edit')
    if (editId && components.length > 0) {
      const component = components.find((c) => c.id === editId)
      if (component) {
        setEditingComponent(component)
        setFormData({
          application_id: component.application_id,
          name: component.name,
          code: component.code || '',
          description: component.description || '',
          structure: component.structure || {},
          default_locale: component.default_locale,
        })
        setShowModal(true)
      }
    }
  }, [searchParams, components])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingComponent) {
        await dispatch(
          updateComponent({ id: editingComponent.id, data: formData })
        ).unwrap()
        toast.success('Component updated')
      } else {
        await dispatch(createComponent(formData)).unwrap()
        toast.success('Component created')
      }
      setShowModal(false)
      setEditingComponent(null)
      setFormData({
        application_id: '',
        name: '',
        code: '',
        description: '',
        structure: {},
        default_locale: 'en',
      })
    } catch (error: any) {
      toast.error(error.message || 'Failed to save component')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this component?')) return
    try {
      await componentApi.delete(id)
      toast.success('Component deleted')
      dispatch(fetchComponents())
    } catch (error: any) {
      toast.error(error.message || 'Failed to delete component')
    }
  }

  if (!isAuthenticated) return null

  const canManage =
    user?.role === 'super_admin' || user?.role === 'operator'

  const filteredComponents = filterApplication
    ? components.filter((c) => c.application_id === filterApplication)
    : components

  const getApplicationName = (appId: string) => {
    return applications.find((a) => a.id === appId)?.name || appId
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-gray-900">Components</h1>
          {canManage && (
            <Button
              variant="primary"
              onClick={() => {
                setEditingComponent(null)
                setFormData({
                  application_id: searchParams.get('application_id') || '',
                  name: '',
                  code: '',
                  description: '',
                  structure: {},
                  default_locale: 'en',
                })
                setShowModal(true)
              }}
            >
              <Plus className="w-4 h-4 mr-2" />
              New Component
            </Button>
          )}
        </div>

        <Card>
          <div className="mb-4">
            <Select
              label="Filter by Application"
              value={filterApplication}
              onChange={(e) => setFilterApplication(e.target.value)}
              options={[
                { value: '', label: 'All Applications' },
                ...applications.map((app) => ({
                  value: app.id,
                  label: app.name,
                })),
              ]}
            />
          </div>

          {loading ? (
            <div className="text-center py-8 text-gray-500">Loading...</div>
          ) : filteredComponents.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              No components found. Create one to get started.
            </div>
          ) : (
            <Table
              headers={['Name', 'Code', 'Application', 'Description', 'Default Locale', 'Actions']}
            >
              {filteredComponents.map((comp) => (
                <TableRow key={comp.id}>
                  <TableCell>
                    <div className="font-medium text-gray-900">{comp.name}</div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm font-mono text-gray-600">{comp.code}</div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="info">{getApplicationName(comp.application_id)}</Badge>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm text-gray-500 max-w-md truncate">
                      {comp.description || 'No description'}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="info">{comp.default_locale}</Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center space-x-2">
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() =>
                          router.push(`/components/${comp.id}/translations`)
                        }
                      >
                        <ArrowRight className="w-4 h-4 mr-1" />
                        Manage
                      </Button>
                      {canManage && (
                        <>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => {
                              setEditingComponent(comp)
                              setFormData({
                                application_id: comp.application_id,
                                name: comp.name,
                                code: comp.code || '',
                                description: comp.description || '',
                                structure: comp.structure || {},
                                default_locale: comp.default_locale,
                              })
                              setShowModal(true)
                            }}
                          >
                            <Edit className="w-4 h-4" />
                          </Button>
                          <Button
                            variant="danger"
                            size="sm"
                            onClick={() => handleDelete(comp.id)}
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

        <Modal
          isOpen={showModal}
          onClose={() => {
            setShowModal(false)
            setEditingComponent(null)
          }}
          title={editingComponent ? 'Edit Component' : 'Create Component'}
          footer={
            <>
              <Button
                variant="primary"
                onClick={handleSubmit}
                type="submit"
                form="component-form"
              >
                {editingComponent ? 'Update' : 'Create'}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setShowModal(false)
                  setEditingComponent(null)
                }}
              >
                Cancel
              </Button>
            </>
          }
        >
          <form id="component-form" onSubmit={handleSubmit} className="space-y-4">
            <Select
              label="Application"
              required
              value={formData.application_id}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  application_id: e.target.value,
                })
              }
              options={[
                { value: '', label: 'Select application' },
                ...applications.map((app) => ({
                  value: app.id,
                  label: app.name,
                })),
              ]}
            />
            <Input
              label="Name"
              required
              value={formData.name}
              onChange={(e) =>
                setFormData({ ...formData, name: e.target.value })
              }
              helperText="Display name for the component"
            />
            <Input
              label="Code"
              required
              value={formData.code}
              onChange={(e) =>
                setFormData({ ...formData, code: e.target.value })
              }
              helperText="Unique code identifier (e.g., pdp_form, checkout_form). Used in API calls."
              placeholder="e.g., pdp_form"
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
              label="Default Locale"
              required
              value={formData.default_locale}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  default_locale: e.target.value,
                })
              }
              helperText="Default language for this component (e.g., en, id, es)"
            />
          </form>
        </Modal>
      </div>
    </Layout>
  )
}

