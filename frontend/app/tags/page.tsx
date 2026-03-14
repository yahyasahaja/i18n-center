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
import { Tag as TagIcon } from 'lucide-react'
import { tagApi, componentApi, type Tag } from '@/services/api'
import toast from 'react-hot-toast'

export default function TagsPage() {
  const router = useRouter()
  const { applicationId, stage, push } = useAppContext()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const [tags, setTags] = useState<Tag[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingTag, setEditingTag] = useState<Tag | null>(null)
  const [formCode, setFormCode] = useState('')
  const [componentsModal, setComponentsModal] = useState<{ tag: Tag; components: { id: string; name: string; code: string }[] } | null>(null)
  const [addToTagModal, setAddToTagModal] = useState<{ tag: Tag; available: { id: string; name: string; code: string }[] } | null>(null)
  const [addToTagSelected, setAddToTagSelected] = useState<string[]>([])
  const [addToTagLoading, setAddToTagLoading] = useState(false)
  const [componentsCount, setComponentsCount] = useState<Record<string, number>>({})

  useEffect(() => {
    const t = localStorage.getItem('token')
    if (!t) {
      router.replace('/login')
      return
    }
    if (!applicationId) {
      setLoading(false)
      setTags([])
      return
    }
    setLoading(true)
    tagApi
      .listByApplication(applicationId)
      .then(setTags)
      .catch(() => setTags([]))
      .finally(() => setLoading(false))
  }, [applicationId, router])

  const canManage = user?.role === 'super_admin' || user?.role === 'operator'

  const openForm = (tag?: Tag | null) => {
    setEditingTag(tag ?? null)
    setFormCode(tag?.code ?? '')
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
      if (editingTag) {
        await tagApi.update(editingTag.id, { code })
        toast.success('Tag updated')
      } else {
        await tagApi.create(applicationId, { code })
        toast.success('Tag created')
      }
      setTags(await tagApi.listByApplication(applicationId))
      setShowModal(false)
      openForm(null)
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to save tag')
    }
  }

  const handleDelete = async (tag: Tag) => {
    if (!confirm(`Delete tag "${tag.code}"?`)) return
    try {
      await tagApi.delete(tag.id)
      toast.success('Tag deleted')
      setTags(await tagApi.listByApplication(applicationId!))
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to delete tag')
    }
  }

  const openComponents = async (tag: Tag) => {
    try {
      const list = await tagApi.getComponents(tag.id)
      setComponentsModal({ tag, components: list || [] })
      if (list?.length != null) setComponentsCount((prev) => ({ ...prev, [tag.id]: list.length }))
    } catch {
      toast.error('Failed to load components')
    }
  }

  const openAddComponentToTag = async () => {
    if (!componentsModal || !applicationId) return
    try {
      const all = await componentApi.getAll(applicationId)
      const currentIds = new Set(componentsModal.components.map((c) => c.id))
      const available = (all || []).filter((c: { id: string }) => !currentIds.has(c.id))
      setAddToTagModal({ tag: componentsModal.tag, available })
      setAddToTagSelected([])
    } catch {
      toast.error('Failed to load components')
    }
  }

  const addComponentsToTag = async () => {
    if (!addToTagModal || addToTagSelected.length === 0) return
    setAddToTagLoading(true)
    try {
      for (const compId of addToTagSelected) {
        const comp = await componentApi.getById(compId) as { id: string; tags?: { id: string }[] }
        const currentTagIds = (comp.tags || []).map((t) => t.id)
        if (currentTagIds.includes(addToTagModal.tag.id)) continue
        await componentApi.update(compId, { tag_ids: [...currentTagIds, addToTagModal.tag.id] })
      }
      toast.success('Components added to tag')
      const list = await tagApi.getComponents(addToTagModal.tag.id)
      setComponentsModal((prev) => (prev ? { ...prev, components: list || [] } : null))
      setAddToTagModal(null)
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to add components')
    } finally {
      setAddToTagLoading(false)
    }
  }

  const removeComponentFromTag = async (compId: string) => {
    if (!componentsModal) return
    try {
      const comp = await componentApi.getById(compId) as { id: string; tags?: { id: string }[] }
      const newTagIds = (comp.tags || []).map((t) => t.id).filter((id) => id !== componentsModal.tag.id)
      await componentApi.update(compId, { tag_ids: newTagIds })
      toast.success('Component removed from tag')
      const list = await tagApi.getComponents(componentsModal.tag.id)
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
          <h1 className="text-3xl font-bold text-gray-900">Tags</h1>
          {canManage && applicationId && (
            <Button variant="primary" onClick={() => openForm()}>
              <TagIcon className="w-4 h-4 mr-2" />
              Add tag
            </Button>
          )}
        </div>

        {!applicationId ? (
          <Card>
            <div className="text-center py-12 text-gray-500">
              <p>Select an application in the sidebar to view and manage tags.</p>
            </div>
          </Card>
        ) : loading ? (
          <Card>
            <div className="text-center py-12 text-gray-500">Loading...</div>
          </Card>
        ) : (
          <Card>
            {tags.length === 0 ? (
              <div className="text-center py-8 text-gray-500">No tags yet. Add one to get started.</div>
            ) : (
              <div className="flex flex-wrap gap-2">
                {tags.map((tag) => (
                  <span
                    key={tag.id}
                    className="inline-flex items-center gap-1 rounded-full bg-gray-100 text-gray-800 px-3 py-1.5 text-sm"
                  >
                    <button type="button" onClick={() => openComponents(tag)} className="hover:underline font-medium text-left">
                      {tag.code}
                      {componentsCount[tag.id] != null && (
                        <span className="text-gray-600 font-normal ml-0.5">
                          ({componentsCount[tag.id]} component{componentsCount[tag.id] !== 1 ? 's' : ''})
                        </span>
                      )}
                    </button>
                    {canManage && (
                      <>
                        <button type="button" onClick={() => openForm(tag)} className="text-gray-500 hover:text-gray-700" title="Edit">✎</button>
                        <button type="button" onClick={() => handleDelete(tag)} className="text-red-500 hover:text-red-700" title="Delete">×</button>
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
          title={editingTag ? 'Edit tag' : 'Add tag'}
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
              placeholder="e.g. checkout, pdp"
              disabled={!!editingTag}
              helperText={editingTag ? 'Code cannot be changed' : 'Unique code for this tag'}
            />
          </form>
        </Modal>

        {componentsModal && (
          <Modal
            isOpen={!!componentsModal}
            onClose={() => { setComponentsModal(null); setAddToTagModal(null) }}
            title={`Components in tag: ${componentsModal.tag.code}`}
            footer={
              <div className="flex items-center justify-between w-full">
                <div>
                  {canManage && (
                    <Button variant="primary" size="sm" onClick={openAddComponentToTag} className="mr-2">
                      Add components to this tag
                    </Button>
                  )}
                </div>
                <Button variant="outline" onClick={() => { setComponentsModal(null); setAddToTagModal(null) }}>Close</Button>
              </div>
            }
          >
            <p className="text-gray-600 text-sm mb-3">
              Manage which components use this tag. {canManage && <>Use <strong>Add components to this tag</strong> below to assign more.</>}
            </p>
            {componentsModal.components.length === 0 ? (
              <p className="text-gray-500 text-sm">No components use this tag yet. {canManage && 'Click the blue button below to add components.'}</p>
            ) : (
              <ul className="space-y-2">
                {componentsModal.components.map((c) => (
                  <li key={c.id} className="flex items-center justify-between gap-2">
                    <div>
                      <button
                        type="button"
                        onClick={() => {
                          setComponentsModal(null)
                          setAddToTagModal(null)
                          push(`/components/${c.id}/translations`)
                        }}
                        className="text-primary-600 hover:underline font-medium"
                      >
                        {c.name}
                      </button>
                      <span className="text-gray-500 text-sm ml-2 font-mono">{c.code}</span>
                    </div>
                    {canManage && (
                      <Button variant="outline" size="sm" onClick={() => removeComponentFromTag(c.id)} className="text-red-600 shrink-0">
                        Remove
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Modal>
        )}

        {addToTagModal && (
          <Modal
            isOpen={!!addToTagModal}
            onClose={() => setAddToTagModal(null)}
            title={`Add components to tag: ${addToTagModal.tag.code}`}
            footer={
              <>
                <Button variant="primary" onClick={addComponentsToTag} disabled={addToTagSelected.length === 0 || addToTagLoading}>
                  {addToTagLoading ? 'Adding…' : `Add ${addToTagSelected.length > 0 ? `(${addToTagSelected.length})` : ''}`}
                </Button>
                <Button variant="outline" onClick={() => setAddToTagModal(null)}>Cancel</Button>
              </>
            }
          >
            {addToTagModal.available.length === 0 ? (
              <p className="text-gray-500 text-sm">All components in this application already have this tag.</p>
            ) : (
              <ul className="space-y-2 max-h-64 overflow-y-auto">
                {addToTagModal.available.map((c) => (
                  <li key={c.id}>
                    <label className="flex items-center gap-2 cursor-pointer text-gray-900">
                      <input
                        type="checkbox"
                        checked={addToTagSelected.includes(c.id)}
                        onChange={(e) => {
                          setAddToTagSelected((prev) =>
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
