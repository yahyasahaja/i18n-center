'use client'

import { useEffect, useState } from 'react'
import { useRouter, useParams, useSearchParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { useAppContext } from '@/context/AppContext'
import { fetchApplication } from '@/store/slices/applicationSlice'
import { fetchComponents } from '@/store/slices/componentSlice'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { ArrowLeft, Plus, Edit, Trash2, ArrowRight, Languages, Rocket, Tag, FileText, Key } from 'lucide-react'
import { componentApi, applicationApi, tagApi, pageApi, type Tag as TagType, type Page as PageType, type ApplicationAPIKey } from '@/services/api'
import toast from 'react-hot-toast'

type PendingDeploy = { locale: string; stage_completed: string; next_stage: string }
type ComponentWithMeta = { id: string; name: string; code: string; tags?: TagType[]; pages?: PageType[] }

export default function ApplicationDetailPage() {
  const router = useRouter()
  const params = useParams()
  const searchParams = useSearchParams()
  const pathname = `/applications/${params.id}`
  const applicationId = params.id as string
  const dispatch = useAppDispatch()
  const { setApplicationId, push } = useAppContext()
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
  const [showDeleteLanguageModal, setShowDeleteLanguageModal] = useState(false)
  const [deleteLanguageLocale, setDeleteLanguageLocale] = useState<string | null>(null)
  const [deleteLanguageConfirm, setDeleteLanguageConfirm] = useState('')
  const [deleteLanguageLoading, setDeleteLanguageLoading] = useState(false)

  const [tags, setTags] = useState<TagType[]>([])
  const [pages, setPages] = useState<PageType[]>([])
  const [showTagModal, setShowTagModal] = useState(false)
  const [showPageModal, setShowPageModal] = useState(false)
  const [editingTag, setEditingTag] = useState<TagType | null>(null)
  const [editingPage, setEditingPage] = useState<PageType | null>(null)
  const [tagFormCode, setTagFormCode] = useState('')
  const [pageFormCode, setPageFormCode] = useState('')
  const [showComponentsInTagModal, setShowComponentsInTagModal] = useState<{ tag: TagType; components: ComponentWithMeta[] } | null>(null)
  const [showComponentsInPageModal, setShowComponentsInPageModal] = useState<{ page: PageType; components: ComponentWithMeta[] } | null>(null)
  const [showComponentMetaModal, setShowComponentMetaModal] = useState<ComponentWithMeta | null>(null)
  const [apiKeys, setApiKeys] = useState<ApplicationAPIKey[]>([])
  const [showNewKeyModal, setShowNewKeyModal] = useState<{ key: string } | null>(null)
  const [addApiKeyLoading, setAddApiKeyLoading] = useState(false)

  const loadPendingDeploys = async () => {
    if (!applicationId) return
    try {
      const res = await applicationApi.getPendingDeploys(applicationId)
      setPendingDeploys(res.pending_deploys || [])
    } catch {
      setPendingDeploys([])
    }
  }

  // Sync sidebar context: when viewing an application, set URL so sidebar shows this application
  useEffect(() => {
    if (!applicationId) return
    if (searchParams.get('application_id') !== applicationId) {
      setApplicationId(applicationId)
    }
  }, [applicationId, searchParams, setApplicationId])

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      router.replace('/login')
      return
    }

    // Don't check isAuthenticated immediately - it might not be initialized yet (e.g. on reload)
    const loadData = async () => {
      try {
        await Promise.all([
          dispatch(fetchApplication(applicationId)),
          dispatch(fetchComponents(applicationId)),
        ])
        await loadPendingDeploys()
        const [tagsRes, pagesRes, keysRes] = await Promise.all([
          tagApi.listByApplication(applicationId).catch(() => []),
          pageApi.listByApplication(applicationId).catch(() => []),
          user?.role === 'super_admin' ? applicationApi.listApiKeys(applicationId).catch(() => []) : Promise.resolve([]),
        ])
        setTags(Array.isArray(tagsRes) ? tagsRes : [])
        setPages(Array.isArray(pagesRes) ? pagesRes : [])
        setApiKeys(Array.isArray(keysRes) ? keysRes : [])
      } catch (error: any) {
        toast.error('Failed to load application data')
      } finally {
        setLoading(false)
      }
    }

    if (applicationId) {
      loadData()
    }
  }, [applicationId, router, dispatch, user?.role])

  useEffect(() => {
    if (searchParams.get('addLanguage') === '1' && currentApplication) {
      setShowAddLanguageModal(true)
    }
  }, [searchParams, currentApplication])

  const pollJobStatus = async (jobId: string): Promise<'completed' | 'failed'> => {
    const maxAttempts = 300 // 5 min at 1s interval
    for (let i = 0; i < maxAttempts; i++) {
      const res = await applicationApi.getAddLanguageJobStatus(applicationId, jobId)
      if (res.status === 'completed') return 'completed'
      if (res.status === 'failed') return 'failed'
      await new Promise((r) => setTimeout(r, 2000)) // poll every 2s
    }
    return 'failed'
  }

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
      const data = await applicationApi.addLanguage(applicationId, {
        locale,
        auto_translate: addLanguageAutoTranslate,
      })
      if (data.job_id) {
        toast.loading(`Adding ${locale.toUpperCase()} and translating components…`, { id: 'add-lang' })
        const result = await pollJobStatus(data.job_id)
        toast.dismiss('add-lang')
        if (result === 'failed') {
          const jobStatus = await applicationApi.getAddLanguageJobStatus(applicationId, data.job_id)
          const detail = jobStatus?.error_detail || jobStatus?.error_message || 'Job failed'
          toast.error(detail + (jobStatus?.retry ? ' You can retry.' : ''))
          await dispatch(fetchApplication(applicationId))
          return
        }
        toast.success(`Language ${locale.toUpperCase()} added. Translations created in draft.`)
      } else {
        toast.success(`Language ${locale.toUpperCase()} added`)
      }
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

  const deleteLanguageConfirmPhrase = deleteLanguageLocale ? `Delete ${deleteLanguageLocale.toUpperCase()}` : ''
  const canConfirmDeleteLanguage = deleteLanguageConfirm.trim() === deleteLanguageConfirmPhrase

  const handleDeleteLanguage = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!deleteLanguageLocale || !canConfirmDeleteLanguage) return
    setDeleteLanguageLoading(true)
    try {
      await applicationApi.deleteLanguage(applicationId, deleteLanguageLocale)
      toast.success(`Language ${deleteLanguageLocale.toUpperCase()} removed`)
      setDeleteLanguageLocale(null)
      setDeleteLanguageConfirm('')
      setShowDeleteLanguageModal(false)
      await dispatch(fetchApplication(applicationId))
      await loadPendingDeploys()
    } catch (error: any) {
      const data = error.response?.data
      const detail = data?.error || 'Failed to delete language'
      toast.error(detail)
    } finally {
      setDeleteLanguageLoading(false)
    }
  }

  const openDeleteLanguageModal = (locale: string) => {
    setDeleteLanguageLocale(locale)
    setDeleteLanguageConfirm('')
    setShowDeleteLanguageModal(true)
  }

  const closeDeleteLanguageModal = () => {
    if (!deleteLanguageLoading) {
      setShowDeleteLanguageModal(false)
      setDeleteLanguageLocale(null)
      setDeleteLanguageConfirm('')
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

  const openTagForm = (tag?: TagType | null) => {
    setEditingTag(tag ?? null)
    setTagFormCode(tag?.code ?? '')
    setShowTagModal(true)
  }
  const openPageForm = (page?: PageType | null) => {
    setEditingPage(page ?? null)
    setPageFormCode(page?.code ?? '')
    setShowPageModal(true)
  }
  const handleSaveTag = async (e: React.FormEvent) => {
    e.preventDefault()
    const code = tagFormCode.trim().toLowerCase()
    if (!code) {
      toast.error('Code is required')
      return
    }
    try {
      if (editingTag) {
        await tagApi.update(editingTag.id, { code })
        toast.success('Tag updated')
      } else {
        await tagApi.create(applicationId, { code })
        toast.success('Tag created')
      }
      setTags(await tagApi.listByApplication(applicationId))
      setShowTagModal(false)
      openTagForm(null)
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to save tag')
    }
  }
  const handleSavePage = async (e: React.FormEvent) => {
    e.preventDefault()
    const code = pageFormCode.trim().toLowerCase()
    if (!code) {
      toast.error('Code is required')
      return
    }
    try {
      if (editingPage) {
        await pageApi.update(editingPage.id, { code })
        toast.success('Page updated')
      } else {
        await pageApi.create(applicationId, { code })
        toast.success('Page created')
      }
      setPages(await pageApi.listByApplication(applicationId))
      setShowPageModal(false)
      openPageForm(null)
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to save page')
    }
  }
  const handleDeleteTag = async (tag: TagType) => {
    if (!confirm(`Delete tag "${tag.code}"?`)) return
    try {
      await tagApi.delete(tag.id)
      toast.success('Tag deleted')
      setTags(await tagApi.listByApplication(applicationId))
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to delete tag')
    }
  }
  const handleAddApiKey = async () => {
    if (!applicationId) return
    setAddApiKeyLoading(true)
    try {
      const res = await applicationApi.createApiKey(applicationId)
      setShowNewKeyModal({ key: res.key })
      setApiKeys(await applicationApi.listApiKeys(applicationId))
      toast.success('API key created. Copy it now — it won’t be shown again.')
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to create API key')
    } finally {
      setAddApiKeyLoading(false)
    }
  }
  const handleDeleteApiKey = async (k: ApplicationAPIKey) => {
    if (!confirm(`Delete API key ${k.key_prefix}…?`)) return
    try {
      await applicationApi.deleteApiKey(applicationId, k.id)
      toast.success('API key deleted')
      setApiKeys(await applicationApi.listApiKeys(applicationId))
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to delete API key')
    }
  }
  const handleDeletePage = async (page: PageType) => {
    if (!confirm(`Delete page "${page.code}"?`)) return
    try {
      await pageApi.delete(page.id)
      toast.success('Page deleted')
      setPages(await pageApi.listByApplication(applicationId))
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to delete page')
    }
  }
  const openComponentsInTag = async (tag: TagType) => {
    try {
      const list = await tagApi.getComponents(tag.id)
      setShowComponentsInTagModal({ tag, components: list || [] })
    } catch {
      toast.error('Failed to load components')
    }
  }
  const openComponentsInPage = async (page: PageType) => {
    try {
      const list = await pageApi.getComponents(page.id)
      setShowComponentsInPageModal({ page, components: list || [] })
    } catch {
      toast.error('Failed to load components')
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
              onClick={() => push('/applications')}
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
  const canManageApiKeys = user?.role === 'super_admin'
  const applicationComponents = components.filter(
    (c) => c.application_id === applicationId
  )

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-4">
            <Button variant="outline" onClick={() => push('/applications')}>
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
            <div className="flex flex-wrap gap-2 items-center">
              {currentApplication.enabled_languages?.map((lang) => {
                const isDefaultLocale = applicationComponents.some((c) => c.default_locale?.toLowerCase() === lang.toLowerCase())
                return (
                  <span key={lang} className="inline-flex items-center gap-1 rounded-full bg-blue-100 text-blue-800 px-2.5 py-0.5 text-sm font-medium">
                    {lang.toUpperCase()}
                    {canManage && (
                      <button
                        type="button"
                        onClick={() => !isDefaultLocale && openDeleteLanguageModal(lang)}
                        disabled={isDefaultLocale}
                        className="rounded p-0.5 hover:bg-blue-200 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-transparent"
                        title={isDefaultLocale ? `Cannot delete: ${lang.toUpperCase()} is the default locale of one or more components` : `Delete ${lang.toUpperCase()}`}
                        aria-label={isDefaultLocale ? `${lang.toUpperCase()} is default locale` : `Delete language ${lang.toUpperCase()}`}
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </span>
                )
              }) || <span className="text-gray-400">None</span>}
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
          {canManageApiKeys && (
            <Card
              title="API Keys"
              actions={
                <Button variant="outline" size="sm" onClick={handleAddApiKey} disabled={addApiKeyLoading}>
                  <Key className="w-4 h-4 mr-2" />
                  Add API Key
                </Button>
              }
            >
              <div className="space-y-2">
                {apiKeys.length === 0 ? (
                  <p className="text-sm text-gray-500">No API keys. Add one to let client apps access translations for this application.</p>
                ) : (
                  <ul className="divide-y divide-gray-200">
                    {apiKeys.map((k) => (
                      <li key={k.id} className="flex items-center justify-between py-2 first:pt-0">
                        <span className="text-sm font-mono text-gray-700">{k.key_prefix}…</span>
                        {k.name && <span className="text-sm text-gray-500">{k.name}</span>}
                        <Button variant="ghost" size="sm" className="text-red-600 hover:text-red-700" onClick={() => handleDeleteApiKey(k)}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </Card>
          )}
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Card
            title="Tags"
            actions={
              canManage && (
                <Button variant="outline" size="sm" onClick={() => openTagForm()}>
                  <Tag className="w-4 h-4 mr-2" />
                  Add tag
                </Button>
              )
            }
          >
            <div className="flex flex-wrap gap-2">
              {tags.length === 0 ? (
                <span className="text-gray-400 text-sm">No tags yet</span>
              ) : (
                tags.map((tag) => (
                  <span key={tag.id} className="inline-flex items-center gap-1 rounded-full bg-gray-100 text-gray-800 px-2.5 py-0.5 text-sm">
                    <button
                      type="button"
                      onClick={() => openComponentsInTag(tag)}
                      className="hover:underline font-medium"
                    >
                      {tag.code}
                    </button>
                    {canManage && (
                      <>
                        <button type="button" onClick={() => openTagForm(tag)} className="text-gray-500 hover:text-gray-700" title="Edit">✎</button>
                        <button type="button" onClick={() => handleDeleteTag(tag)} className="text-red-500 hover:text-red-700" title="Delete">×</button>
                      </>
                    )}
                  </span>
                ))
              )}
            </div>
          </Card>
          <Card
            title="Pages"
            actions={
              canManage && (
                <Button variant="outline" size="sm" onClick={() => openPageForm()}>
                  <FileText className="w-4 h-4 mr-2" />
                  Add page
                </Button>
              )
            }
          >
            <div className="flex flex-wrap gap-2">
              {pages.length === 0 ? (
                <span className="text-gray-400 text-sm">No pages yet</span>
              ) : (
                pages.map((page) => (
                  <span key={page.id} className="inline-flex items-center gap-1 rounded-full bg-gray-100 text-gray-800 px-2.5 py-0.5 text-sm">
                    <button
                      type="button"
                      onClick={() => openComponentsInPage(page)}
                      className="hover:underline font-medium"
                    >
                      {page.code}
                    </button>
                    {canManage && (
                      <>
                        <button type="button" onClick={() => openPageForm(page)} className="text-gray-500 hover:text-gray-700" title="Edit">✎</button>
                        <button type="button" onClick={() => handleDeletePage(page)} className="text-red-500 hover:text-red-700" title="Delete">×</button>
                      </>
                    )}
                  </span>
                ))
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
                onClick={() => push('/components')}
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
            <Table headers={['Name', 'Code', 'Description', 'Default Locale', 'Pages & tags', 'Actions']}>
              {applicationComponents.map((component) => {
                const comp = component as ComponentWithMeta
                const tagList = comp.tags || []
                const pageList = comp.pages || []
                const tagCount = tagList.length
                const pageCount = pageList.length
                const extra = Math.max(0, tagCount - 2) + Math.max(0, pageCount - 2)
                return (
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
                    <div className="flex flex-wrap items-center gap-1.5">
                      {tagList.slice(0, 2).map((t) => (
                        <span key={t.id} className="rounded bg-gray-100 text-gray-700 px-1.5 py-0.5 text-xs">{t.code}</span>
                      ))}
                      {pageList.slice(0, 2).map((p) => (
                        <span key={p.id} className="rounded bg-blue-50 text-blue-700 px-1.5 py-0.5 text-xs">{p.code}</span>
                      ))}
                      {(tagCount > 2 || pageCount > 2) && (
                        <span className="text-gray-500 text-xs">+{extra}</span>
                      )}
                      <button
                        type="button"
                        onClick={() => setShowComponentMetaModal(comp)}
                        className="text-xs text-primary-600 hover:underline font-medium"
                      >
                        Pages & tags
                      </button>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center space-x-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          push(`/components/${component.id}/translations`)
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
                              push('/components', { extraParams: { edit: component.id } })
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
              )})}
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

        <Modal
          isOpen={showDeleteLanguageModal}
          onClose={closeDeleteLanguageModal}
          title="Delete language"
          footer={
            <>
              <Button
                variant="danger"
                onClick={handleDeleteLanguage}
                disabled={!canConfirmDeleteLanguage || deleteLanguageLoading}
                isLoading={deleteLanguageLoading}
              >
                Delete {deleteLanguageLocale?.toUpperCase()}
              </Button>
              <Button variant="outline" onClick={closeDeleteLanguageModal} disabled={deleteLanguageLoading}>
                Cancel
              </Button>
            </>
          }
        >
          {deleteLanguageLocale && (
            <form onSubmit={handleDeleteLanguage} className="space-y-4">
              <p className="text-sm text-gray-700">
                Removing <strong>{deleteLanguageLocale.toUpperCase()}</strong> will delete all translations for this language across all components (draft, staging, and production). This cannot be undone.
              </p>
              <p className="text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded-md px-3 py-2">
                To confirm, type <strong>{deleteLanguageConfirmPhrase}</strong> below.
              </p>
              <Input
                label="Confirmation"
                value={deleteLanguageConfirm}
                onChange={(e) => setDeleteLanguageConfirm(e.target.value)}
                placeholder={deleteLanguageConfirmPhrase}
                autoComplete="off"
                className="font-mono"
              />
            </form>
          )}
        </Modal>

        <Modal
          isOpen={!!showNewKeyModal}
          onClose={() => setShowNewKeyModal(null)}
          title="API key created"
          size="lg"
          footer={
            <Button variant="primary" onClick={() => setShowNewKeyModal(null)}>Done</Button>
          }
        >
          {showNewKeyModal && (
            <div className="space-y-4 min-w-0">
              <p className="text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded-md px-3 py-2">
                Copy this key now. It won’t be shown again. Use it as <code className="bg-amber-100 px-1">Authorization: Bearer &lt;key&gt;</code> or <code className="bg-amber-100 px-1">X-API-Key: &lt;key&gt;</code> when calling the translations API.
              </p>
              <div className="min-w-0 max-w-full overflow-x-auto">
                <pre className="p-3 bg-gray-100 rounded font-mono text-sm break-all select-all whitespace-pre-wrap">{showNewKeyModal.key}</pre>
              </div>
            </div>
          )}
        </Modal>

        <Modal
          isOpen={showTagModal}
          onClose={() => { setShowTagModal(false); setEditingTag(null); }}
          title={editingTag ? 'Edit tag' : 'Add tag'}
          footer={
            <>
              <Button variant="primary" onClick={handleSaveTag}>Save</Button>
              <Button variant="outline" onClick={() => setShowTagModal(false)}>Cancel</Button>
            </>
          }
        >
          <form onSubmit={handleSaveTag} className="space-y-4">
            <Input label="Code" value={tagFormCode} onChange={(e) => setTagFormCode(e.target.value)} placeholder="e.g. checkout, pdp" disabled={!!editingTag} helperText={editingTag ? 'Code cannot be changed' : 'Unique code for this tag'} />
          </form>
        </Modal>

        <Modal
          isOpen={showPageModal}
          onClose={() => { setShowPageModal(false); setEditingPage(null); }}
          title={editingPage ? 'Edit page' : 'Add page'}
          footer={
            <>
              <Button variant="primary" onClick={handleSavePage}>Save</Button>
              <Button variant="outline" onClick={() => setShowPageModal(false)}>Cancel</Button>
            </>
          }
        >
          <form onSubmit={handleSavePage} className="space-y-4">
            <Input label="Code" value={pageFormCode} onChange={(e) => setPageFormCode(e.target.value)} placeholder="e.g. home, cart" disabled={!!editingPage} helperText={editingPage ? 'Code cannot be changed' : 'Unique code for this page'} />
          </form>
        </Modal>

        {showComponentsInTagModal && (
          <Modal
            isOpen={!!showComponentsInTagModal}
            onClose={() => setShowComponentsInTagModal(null)}
            title={`Components in tag: ${showComponentsInTagModal.tag.code}`}
            footer={<Button variant="outline" onClick={() => setShowComponentsInTagModal(null)}>Close</Button>}
          >
            {showComponentsInTagModal.components.length === 0 ? (
              <p className="text-gray-500 text-sm">No components use this tag.</p>
            ) : (
              <ul className="space-y-2">
                {showComponentsInTagModal.components.map((c) => (
                  <li key={c.id}>
                    <button type="button" onClick={() => { setShowComponentsInTagModal(null); push(`/components/${c.id}/translations`); }} className="text-primary-600 hover:underline font-medium">
                      {c.name}
                    </button>
                    <span className="text-gray-500 text-sm ml-2 font-mono">{c.code}</span>
                  </li>
                ))}
              </ul>
            )}
          </Modal>
        )}

        {showComponentsInPageModal && (
          <Modal
            isOpen={!!showComponentsInPageModal}
            onClose={() => setShowComponentsInPageModal(null)}
            title={`Components in page: ${showComponentsInPageModal.page.code}`}
            footer={<Button variant="outline" onClick={() => setShowComponentsInPageModal(null)}>Close</Button>}
          >
            {showComponentsInPageModal.components.length === 0 ? (
              <p className="text-gray-500 text-sm">No components use this page.</p>
            ) : (
              <ul className="space-y-2">
                {showComponentsInPageModal.components.map((c) => (
                  <li key={c.id}>
                    <button type="button" onClick={() => { setShowComponentsInPageModal(null); push(`/components/${c.id}/translations`); }} className="text-primary-600 hover:underline font-medium">
                      {c.name}
                    </button>
                    <span className="text-gray-500 text-sm ml-2 font-mono">{c.code}</span>
                  </li>
                ))}
              </ul>
            )}
          </Modal>
        )}

        {showComponentMetaModal && (
          <Modal
            isOpen={!!showComponentMetaModal}
            onClose={() => setShowComponentMetaModal(null)}
            title={`Pages & tags: ${showComponentMetaModal.name}`}
            footer={<Button variant="outline" onClick={() => setShowComponentMetaModal(null)}>Close</Button>}
          >
            <div className="space-y-4">
              <div>
                <div className="text-sm font-medium text-gray-700 mb-1">Tags</div>
                {(showComponentMetaModal.tags?.length ?? 0) === 0 ? (
                  <p className="text-gray-500 text-sm">None</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {showComponentMetaModal.tags!.map((t) => (
                      <span key={t.id} className="rounded-full bg-gray-100 text-gray-800 px-2.5 py-0.5 text-sm">{t.code}</span>
                    ))}
                  </div>
                )}
              </div>
              <div>
                <div className="text-sm font-medium text-gray-700 mb-1">Pages</div>
                {(showComponentMetaModal.pages?.length ?? 0) === 0 ? (
                  <p className="text-gray-500 text-sm">None</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {showComponentMetaModal.pages!.map((p) => (
                      <span key={p.id} className="rounded-full bg-blue-50 text-blue-800 px-2.5 py-0.5 text-sm">{p.code}</span>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </Modal>
        )}
      </div>
    </Layout>
  )
}

