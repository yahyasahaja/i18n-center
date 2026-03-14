'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppSelector } from '@/hooks/redux'
import { useAppContext } from '@/context/AppContext'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { FileText } from 'lucide-react'
import { pageApi, componentApi, type Page } from '@/services/api'
import toast from 'react-hot-toast'

export default function PagesPage() {
  const router = useRouter()
  const { applicationId, stage, push } = useAppContext()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const [pages, setPages] = useState<Page[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingPage, setEditingPage] = useState<Page | null>(null)
  const [formCode, setFormCode] = useState('')
  const [componentsModal, setComponentsModal] = useState<{ page: Page; components: { id: string; name: string; code: string }[] } | null>(null)
  const [addToPageModal, setAddToPageModal] = useState<{ page: Page; available: { id: string; name: string; code: string }[] } | null>(null)
  const [addToPageSelected, setAddToPageSelected] = useState<string[]>([])
  const [addToPageLoading, setAddToPageLoading] = useState(false)

  useEffect(() => {
    const t = localStorage.getItem('token')
    if (!t) {
      router.replace('/login')
      return
    }
    if (!applicationId) {
      setLoading(false)
      setPages([])
      return
    }
    setLoading(true)
    pageApi
      .listByApplication(applicationId)
      .then(setPages)
      .catch(() => setPages([]))
      .finally(() => setLoading(false))
  }, [applicationId, router])

  const canManage = user?.role === 'super_admin' || user?.role === 'operator'

  const openForm = (page?: Page | null) => {
    setEditingPage(page ?? null)
    setFormCode(page?.code ?? '')
    setShowModal(true)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!applicationId) return
    const code = formCode.trim().toLowerCase()
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
      setShowModal(false)
      openForm(null)
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to save page')
    }
  }

  const handleDelete = async (page: Page) => {
    if (!confirm(`Delete page "${page.code}"?`)) return
    try {
      await pageApi.delete(page.id)
      toast.success('Page deleted')
      setPages(await pageApi.listByApplication(applicationId!))
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to delete page')
    }
  }

  const [componentsCount, setComponentsCount] = useState<Record<string, number>>({})

  const openComponents = async (page: Page) => {
    try {
      const list = await pageApi.getComponents(page.id)
      setComponentsModal({ page, components: list || [] })
      if (list?.length != null) setComponentsCount((prev) => ({ ...prev, [page.id]: list.length }))
    } catch {
      toast.error('Failed to load components')
    }
  }

  const openAddComponentToPage = async () => {
    if (!componentsModal || !applicationId) return
    try {
      const all = await componentApi.getAll(applicationId)
      const currentIds = new Set(componentsModal.components.map((c) => c.id))
      const available = (all || []).filter((c: { id: string }) => !currentIds.has(c.id))
      setAddToPageModal({ page: componentsModal.page, available })
      setAddToPageSelected([])
    } catch {
      toast.error('Failed to load components')
    }
  }

  const addComponentsToPage = async () => {
    if (!addToPageModal || addToPageSelected.length === 0) return
    setAddToPageLoading(true)
    try {
      for (const compId of addToPageSelected) {
        const comp = await componentApi.getById(compId) as { id: string; pages?: { id: string }[] }
        const currentPageIds = (comp.pages || []).map((p) => p.id)
        if (currentPageIds.includes(addToPageModal.page.id)) continue
        await componentApi.update(compId, { page_ids: [...currentPageIds, addToPageModal.page.id] })
      }
      toast.success('Components added to page')
      const list = await pageApi.getComponents(addToPageModal.page.id)
      setComponentsModal((prev) => (prev ? { ...prev, components: list || [] } : null))
      setAddToPageModal(null)
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to add components')
    } finally {
      setAddToPageLoading(false)
    }
  }

  const removeComponentFromPage = async (compId: string) => {
    if (!componentsModal) return
    try {
      const comp = await componentApi.getById(compId) as { id: string; pages?: { id: string }[] }
      const newPageIds = (comp.pages || []).map((p) => p.id).filter((id) => id !== componentsModal.page.id)
      await componentApi.update(compId, { page_ids: newPageIds })
      toast.success('Component removed from page')
      const list = await pageApi.getComponents(componentsModal.page.id)
      setComponentsModal({ ...componentsModal, components: list || [] })
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to remove')
    }
  }

  if (!isAuthenticated) return null

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-gray-900">Pages</h1>
          {canManage && applicationId && (
            <Button variant="primary" onClick={() => openForm()}>
              <FileText className="w-4 h-4 mr-2" />
              Add page
            </Button>
          )}
        </div>

        {!applicationId ? (
          <Card>
            <div className="text-center py-12 text-gray-500">
              <p>Select an application in the sidebar to view and manage pages.</p>
            </div>
          </Card>
        ) : loading ? (
          <Card>
            <div className="text-center py-12 text-gray-500">Loading...</div>
          </Card>
        ) : (
          <Card>
            {pages.length === 0 ? (
              <div className="text-center py-8 text-gray-500">No pages yet. Add one to get started.</div>
            ) : (
              <div className="flex flex-wrap gap-2">
                {pages.map((page) => (
                  <span
                    key={page.id}
                    className="inline-flex items-center gap-1 rounded-full bg-blue-50 text-blue-800 px-3 py-1.5 text-sm"
                  >
                    <button type="button" onClick={() => openComponents(page)} className="hover:underline font-medium text-left">
                      {page.code}
                      {componentsCount[page.id] != null && (
                        <span className="text-blue-600/80 font-normal ml-0.5">
                          ({componentsCount[page.id]} component{componentsCount[page.id] !== 1 ? 's' : ''})
                        </span>
                      )}
                    </button>
                    {canManage && (
                      <>
                        <button type="button" onClick={() => openForm(page)} className="text-blue-600 hover:text-blue-800" title="Edit">✎</button>
                        <button type="button" onClick={() => handleDelete(page)} className="text-red-500 hover:text-red-700" title="Delete">×</button>
                      </>
                    )}
                  </span>
                ))}
              </div>
            )}
          </Card>
        )}

        <Modal
          isOpen={showModal}
          onClose={() => setShowModal(false)}
          title={editingPage ? 'Edit page' : 'Add page'}
          footer={
            <>
              <Button variant="primary" onClick={handleSave}>Save</Button>
              <Button variant="outline" onClick={() => setShowModal(false)}>Cancel</Button>
            </>
          }
        >
          <form onSubmit={handleSave} className="space-y-4">
            <Input
              label="Code"
              value={formCode}
              onChange={(e) => setFormCode(e.target.value)}
              placeholder="e.g. home, cart"
              disabled={!!editingPage}
              helperText={editingPage ? 'Code cannot be changed' : 'Unique code for this page'}
            />
          </form>
        </Modal>

        {componentsModal && (
          <Modal
            isOpen={!!componentsModal}
            onClose={() => { setComponentsModal(null); setAddToPageModal(null) }}
            title={`Components in page: ${componentsModal.page.code}`}
            footer={
              <div className="flex items-center justify-between w-full">
                <div>
                  {canManage && (
                    <Button variant="primary" size="sm" onClick={openAddComponentToPage} className="mr-2">
                      Add components to this page
                    </Button>
                  )}
                </div>
                <Button variant="outline" onClick={() => { setComponentsModal(null); setAddToPageModal(null) }}>Close</Button>
              </div>
            }
          >
            <p className="text-gray-600 text-sm mb-3">
              Manage which components appear on this page. {canManage && <>Use <strong>Add components to this page</strong> below to assign more.</>}
            </p>
            {componentsModal.components.length === 0 ? (
              <p className="text-gray-500 text-sm">No components use this page yet. {canManage && 'Click the blue button below to add components.'}</p>
            ) : (
              <ul className="space-y-2">
                {componentsModal.components.map((c) => (
                  <li key={c.id} className="flex items-center justify-between gap-2">
                    <div>
                      <button
                        type="button"
                        onClick={() => {
                          setComponentsModal(null)
                          setAddToPageModal(null)
                          push(`/components/${c.id}/translations`)
                        }}
                        className="text-primary-600 hover:underline font-medium"
                      >
                        {c.name}
                      </button>
                      <span className="text-gray-500 text-sm ml-2 font-mono">{c.code}</span>
                    </div>
                    {canManage && (
                      <Button variant="outline" size="sm" onClick={() => removeComponentFromPage(c.id)} className="text-red-600 shrink-0">
                        Remove
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Modal>
        )}

        {addToPageModal && (
          <Modal
            isOpen={!!addToPageModal}
            onClose={() => setAddToPageModal(null)}
            title={`Add components to page: ${addToPageModal.page.code}`}
            footer={
              <>
                <Button variant="primary" onClick={addComponentsToPage} disabled={addToPageSelected.length === 0 || addToPageLoading}>
                  {addToPageLoading ? 'Adding…' : `Add ${addToPageSelected.length > 0 ? `(${addToPageSelected.length})` : ''}`}
                </Button>
                <Button variant="outline" onClick={() => setAddToPageModal(null)}>Cancel</Button>
              </>
            }
          >
            {addToPageModal.available.length === 0 ? (
              <p className="text-gray-500 text-sm">All components in this application already have this page.</p>
            ) : (
              <ul className="space-y-2 max-h-64 overflow-y-auto">
                {addToPageModal.available.map((c) => (
                  <li key={c.id}>
                    <label className="flex items-center gap-2 cursor-pointer text-gray-900">
                      <input
                        type="checkbox"
                        checked={addToPageSelected.includes(c.id)}
                        onChange={(e) => {
                          setAddToPageSelected((prev) =>
                            e.target.checked ? [...prev, c.id] : prev.filter((id) => id !== c.id)
                          )
                        }}
                        className="rounded border-gray-300"
                      />
                      <span className="font-medium text-gray-900">{c.name}</span>
                      <span className="text-gray-600 text-sm font-mono">{c.code}</span>
                    </label>
                  </li>
                ))}
              </ul>
            )}
          </Modal>
        )}
      </div>
    </Layout>
  )
}
