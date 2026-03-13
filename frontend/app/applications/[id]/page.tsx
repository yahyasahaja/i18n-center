'use client'

import { useEffect, useState } from 'react'
import { useRouter, useParams, useSearchParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { fetchApplication } from '@/store/slices/applicationSlice'
import { fetchComponents } from '@/store/slices/componentSlice'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { ArrowLeft, Plus, Edit, Trash2, ArrowRight, Languages, Rocket } from 'lucide-react'
import { componentApi, applicationApi } from '@/services/api'
import toast from 'react-hot-toast'

type PendingDeploy = { locale: string; stage_completed: string; next_stage: string }

export default function ApplicationDetailPage() {
  const router = useRouter()
  const params = useParams()
  const searchParams = useSearchParams()
  const applicationId = params.id as string
  const dispatch = useAppDispatch()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const { currentApplication } = useAppSelector((state) => state.applications)
  const { components } = useAppSelector((state) => state.components)
  const [loading, setLoading] = useState(true)
  const [showAddLanguageModal, setShowAddLanguageModal] = useState(false)
  const [addLanguageLocale, setAddLanguageLocale] = useState('')
  const [addLanguageAutoTranslate, setAddLanguageAutoTranslate] = useState(true)
  const [addLanguageLoading, setAddLanguageLoading] = useState(false)
  const [pendingDeploys, setPendingDeploys] = useState<PendingDeploy[]>([])
  const [deployingLocale, setDeployingLocale] = useState<string | null>(null)

  const loadPendingDeploys = async () => {
    if (!applicationId) return
    try {
      const res = await applicationApi.getPendingDeploys(applicationId)
      setPendingDeploys(res.pending_deploys || [])
    } catch {
      setPendingDeploys([])
    }
  }

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
        await loadPendingDeploys()
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

  useEffect(() => {
    if (searchParams.get('addLanguage') === '1' && currentApplication) {
      setShowAddLanguageModal(true)
    }
  }, [searchParams, currentApplication])

  const handleAddLanguage = async (e: React.FormEvent) => {
    e.preventDefault()
    const locale = addLanguageLocale.trim().toLowerCase()
    if (!locale) {
      toast.error('Enter a language code')
      return
    }
    if (currentApplication?.enabled_languages?.some((l) => l.toLowerCase() === locale)) {
      toast.error('This language is already enabled')
      return
    }
    setAddLanguageLoading(true)
    try {
      await applicationApi.addLanguage(applicationId, {
        locale,
        auto_translate: addLanguageAutoTranslate,
      })
      toast.success(`Language ${locale.toUpperCase()} added${addLanguageAutoTranslate ? '. Translations created in draft.' : ''}`)
      setShowAddLanguageModal(false)
      setAddLanguageLocale('')
      setAddLanguageAutoTranslate(true)
      await dispatch(fetchApplication(applicationId))
      await loadPendingDeploys()
    } catch (error: any) {
      const data = error.response?.data
      const detail = data?.detail || data?.error || 'Failed to add language'
      const retry = data?.retry ? ' You can retry.' : ''
      toast.error(detail + retry)
    } finally {
      setAddLanguageLoading(false)
    }
  }

  const handleDeployLocale = async (locale: string) => {
    setDeployingLocale(locale)
    try {
      await applicationApi.deployLocale(applicationId, locale)
      const item = pendingDeploys.find((p) => p.locale === locale)
      toast.success(`Deployed ${locale.toUpperCase()} to ${item?.next_stage || 'next stage'}`)
      await loadPendingDeploys()
    } catch (error: any) {
      const data = error.response?.data
      const detail = data?.detail || data?.error || 'Deploy failed'
      toast.error(detail + (data?.retry ? ' You can retry.' : ''))
    } finally {
      setDeployingLocale(null)
    }
  }

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
          <Card
            title="Enabled Languages"
            actions={
              canManage && (
                <Button variant="outline" size="sm" onClick={() => setShowAddLanguageModal(true)}>
                  <Languages className="w-4 h-4 mr-2" />
                  Add language
                </Button>
              )
            }
          >
            <div className="flex flex-wrap gap-2">
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

        {pendingDeploys.length > 0 && (
          <Card title="Pending locale deploys">
            <p className="text-sm text-gray-600 mb-4">
              These locales have draft (or staging) translations. Deploy them to the next stage until production. State is saved—you can continue after reload.
            </p>
            <Table headers={['Locale', 'Current stage', 'Action']}>
              {pendingDeploys.map((p) => (
                <TableRow key={p.locale}>
                  <TableCell>
                    <Badge variant="info">{p.locale.toUpperCase()}</Badge>
                  </TableCell>
                  <TableCell>
                    <span className="text-sm text-gray-600">{p.stage_completed}</span>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => handleDeployLocale(p.locale)}
                      isLoading={deployingLocale === p.locale}
                      disabled={!!deployingLocale}
                    >
                      <Rocket className="w-4 h-4 mr-2" />
                      Deploy to {p.next_stage}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </Table>
          </Card>
        )}

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

        <Modal
          isOpen={showAddLanguageModal}
          onClose={() => setShowAddLanguageModal(false)}
          title="Add language"
          footer={
            <>
              <Button variant="primary" onClick={handleAddLanguage} disabled={addLanguageLoading} isLoading={addLanguageLoading}>
                Add language
              </Button>
              <Button variant="outline" onClick={() => setShowAddLanguageModal(false)} disabled={addLanguageLoading}>
                Cancel
              </Button>
            </>
          }
        >
          <form onSubmit={handleAddLanguage} className="space-y-4">
            <Input
              label="Language code"
              value={addLanguageLocale}
              onChange={(e) => setAddLanguageLocale(e.target.value)}
              placeholder="e.g. id, es, fr"
              helperText="Two-letter or locale code (e.g. en, id, es)"
            />
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={addLanguageAutoTranslate}
                onChange={(e) => setAddLanguageAutoTranslate(e.target.checked)}
                className="rounded border-gray-300"
              />
              <span className="text-sm text-gray-700">Auto-translate from each component&apos;s default locale</span>
            </label>
            {addLanguageAutoTranslate && (
              <p className="text-xs text-gray-500">
                All components will be translated to this locale (draft). You can then deploy draft → staging → production. If any step fails, changes are rolled back and you can retry.
              </p>
            )}
          </form>
        </Modal>
      </div>
    </Layout>
  )
}

