'use client'

import { useEffect, useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Select } from '@/components/ui/Select'
import { Badge } from '@/components/ui/Badge'
import { componentApi, tagApi, pageApi, type Tag, type Page } from '@/services/api'
import toast from 'react-hot-toast'

type Application = { id: string; name: string }
type ComponentForEdit = {
  id: string
  application_id: string
  name: string
  code: string
  description?: string
  structure?: Record<string, unknown>
  key_contexts?: Record<string, string> | null
  default_locale: string
  tags?: Tag[]
  pages?: Page[]
}

// Key-context row helpers used to live here; the editor moved to the
// Translation page (see TranslationEditor.tsx) and the modal now just
// preserves whatever key_contexts the component already has.

interface ComponentFormModalProps {
  isOpen: boolean
  onClose: () => void
  component: ComponentForEdit | null
  applications: Application[]
  defaultApplicationId?: string
  onSaved: () => void
}

export function ComponentFormModal({
  isOpen,
  onClose,
  component,
  applications,
  defaultApplicationId = '',
  onSaved,
}: ComponentFormModalProps) {
  const [formData, setFormData] = useState({
    application_id: defaultApplicationId,
    name: '',
    code: '',
    description: '',
    structure: {} as Record<string, unknown>,
    default_locale: 'en',
    tag_ids: [] as string[],
    page_ids: [] as string[],
  })
  // Preserved verbatim from the component being edited; this modal no longer
  // edits it — the Translation page owns Key Contexts UX.
  const [preservedKeyContexts, setPreservedKeyContexts] = useState<Record<string, string> | null | undefined>(null)
  const [appTags, setAppTags] = useState<Tag[]>([])
  const [appPages, setAppPages] = useState<Page[]>([])
  const [newTagCode, setNewTagCode] = useState('')
  const [newPageCode, setNewPageCode] = useState('')
  const [addingTag, setAddingTag] = useState(false)
  const [addingPage, setAddingPage] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (!isOpen) return
    if (component) {
      setFormData({
        application_id: component.application_id,
        name: component.name,
        code: component.code || '',
        description: component.description || '',
        structure: component.structure || {},
        default_locale: component.default_locale || 'en',
        tag_ids: (component.tags || []).map((t) => t.id),
        page_ids: (component.pages || []).map((p) => p.id),
      })
      setPreservedKeyContexts(component.key_contexts)
    } else {
      setFormData({
        application_id: defaultApplicationId,
        name: '',
        code: '',
        description: '',
        structure: {},
        default_locale: 'en',
        tag_ids: [],
        page_ids: [],
      })
      setPreservedKeyContexts(null)
    }
    setNewTagCode('')
    setNewPageCode('')
  }, [isOpen, component, defaultApplicationId])

  useEffect(() => {
    if (!formData.application_id) {
      setAppTags([])
      setAppPages([])
      return
    }
    Promise.all([
      tagApi.listByApplication(formData.application_id).catch(() => []),
      pageApi.listByApplication(formData.application_id).catch(() => []),
    ]).then(([tags, pages]) => {
      setAppTags(Array.isArray(tags) ? tags : [])
      setAppPages(Array.isArray(pages) ? pages : [])
    })
  }, [formData.application_id])

  const handleAddNewTag = async () => {
    const code = newTagCode.trim().toLowerCase()
    if (!code || !formData.application_id) return
    setAddingTag(true)
    try {
      const tag = await tagApi.create(formData.application_id, { code })
      setAppTags((prev) => [...prev, tag])
      setFormData((prev) => ({ ...prev, tag_ids: [...prev.tag_ids, tag.id] }))
      setNewTagCode('')
      toast.success(`Tag "${code}" added`)
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } }
      toast.error(e.response?.data?.error || 'Failed to create tag')
    } finally {
      setAddingTag(false)
    }
  }

  const handleAddNewPage = async () => {
    const code = newPageCode.trim().toLowerCase()
    if (!code || !formData.application_id) return
    setAddingPage(true)
    try {
      const page = await pageApi.create(formData.application_id, { code })
      setAppPages((prev) => [...prev, page])
      setFormData((prev) => ({ ...prev, page_ids: [...prev.page_ids, page.id] }))
      setNewPageCode('')
      toast.success(`Page "${code}" added`)
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } }
      toast.error(e.response?.data?.error || 'Failed to create page')
    } finally {
      setAddingPage(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    try {
      // Round-trip whatever key_contexts the component already had. The
      // Translation page is the canonical editor for that field — this modal
      // just passes the value through unchanged so we don't silently wipe it.
      const payload = {
        ...formData,
        key_contexts: preservedKeyContexts || {},
      }
      if (component) {
        await componentApi.update(component.id, payload)
        toast.success('Component updated')
      } else {
        await componentApi.create(payload)
        toast.success('Component created')
      }
      onSaved()
      onClose()
    } catch (error: unknown) {
      const err = error as { message?: string }
      toast.error(err.message || 'Failed to save component')
    } finally {
      setSubmitting(false)
    }
  }

  if (!isOpen) return null

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={component ? 'Edit Component' : 'Create Component'}
      size="lg"
      footer={
        <>
          <Button
            variant="primary"
            disabled={submitting}
            type="submit"
            form="component-form-modal"
          >
            {submitting ? 'Saving…' : component ? 'Update' : 'Create'}
          </Button>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
        </>
      }
    >
      <form id="component-form-modal" onSubmit={handleSubmit} className="space-y-4">
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
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          helperText="Display name for the component"
        />
        <Input
          label="Code"
          required
          value={formData.code}
          onChange={(e) => setFormData({ ...formData, code: e.target.value })}
          helperText="Unique code identifier (e.g., pdp_form, checkout_form). Used in API calls."
          placeholder="e.g., pdp_form"
        />
        <Textarea
          label="Description"
          value={formData.description}
          onChange={(e) => setFormData({ ...formData, description: e.target.value })}
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

        {/* Key Contexts moved out of this modal — the canonical home is now
            the translation editor page, where it sits next to the JSON being
            described (and where the path autocomplete can pull leaf keys from
            the live JSON). We keep a breadcrumb here so authors who land on
            this modal still find their way. */}
        {component && (
          <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
            <strong className="font-semibold">Looking for Key Contexts?</strong>{' '}
            Edit them on the component&apos;s Translation page — you&apos;ll get inline
            autocomplete against the live keys.
          </div>
        )}

        {formData.application_id && (
          <>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Tags</label>
              <div className="border border-gray-200 rounded-md p-2 space-y-2 text-gray-900">
                {appTags.length === 0 ? (
                  <span className="text-gray-500 text-sm">No tags in this application</span>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {appTags.map((t) => (
                      <label key={t.id} className="inline-flex items-center gap-1.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={formData.tag_ids.includes(t.id)}
                          onChange={(e) => {
                            setFormData({
                              ...formData,
                              tag_ids: e.target.checked
                                ? [...formData.tag_ids, t.id]
                                : formData.tag_ids.filter((id) => id !== t.id),
                            })
                          }}
                          className="rounded border-gray-300"
                        />
                        <span className="text-sm text-gray-900">{t.code}</span>
                      </label>
                    ))}
                  </div>
                )}
                <div className="flex flex-wrap items-center gap-2 pt-1 border-t border-gray-100">
                  <span className="text-xs text-gray-500">Selected:</span>
                  {formData.tag_ids.map((id) => {
                    const t = appTags.find((x) => x.id === id)
                    return t ? (
                      <Badge key={t.id} variant="secondary" className="text-xs">
                        {t.code}
                        <button
                          type="button"
                          onClick={() =>
                            setFormData({ ...formData, tag_ids: formData.tag_ids.filter((i) => i !== id) })
                          }
                          className="ml-1 hover:text-red-600"
                          aria-label={`Remove ${t.code}`}
                        >
                          ×
                        </button>
                      </Badge>
                    ) : null
                  })}
                </div>
                <div className="flex gap-2 items-center flex-wrap">
                  <div className="w-28 shrink-0">
                    <Input
                      placeholder="New tag code"
                      value={newTagCode}
                      onChange={(e) => setNewTagCode(e.target.value)}
                      onKeyDown={(e) =>
                        e.key === 'Enter' && (e.preventDefault(), handleAddNewTag())
                      }
                      className="min-w-0 w-full"
                    />
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleAddNewTag}
                    disabled={!newTagCode.trim() || addingTag}
                  >
                    {addingTag ? 'Adding…' : 'Add tag'}
                  </Button>
                </div>
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Pages</label>
              <div className="border border-gray-200 rounded-md p-2 space-y-2 text-gray-900">
                {appPages.length === 0 ? (
                  <span className="text-gray-500 text-sm">No pages in this application</span>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {appPages.map((p) => (
                      <label key={p.id} className="inline-flex items-center gap-1.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={formData.page_ids.includes(p.id)}
                          onChange={(e) => {
                            setFormData({
                              ...formData,
                              page_ids: e.target.checked
                                ? [...formData.page_ids, p.id]
                                : formData.page_ids.filter((id) => id !== p.id),
                            })
                          }}
                          className="rounded border-gray-300"
                        />
                        <span className="text-sm text-gray-900">{p.code}</span>
                      </label>
                    ))}
                  </div>
                )}
                <div className="flex flex-wrap items-center gap-2 pt-1 border-t border-gray-100">
                  <span className="text-xs text-gray-500">Selected:</span>
                  {formData.page_ids.map((id) => {
                    const p = appPages.find((x) => x.id === id)
                    return p ? (
                      <Badge key={p.id} variant="secondary" className="text-xs">
                        {p.code}
                        <button
                          type="button"
                          onClick={() =>
                            setFormData({ ...formData, page_ids: formData.page_ids.filter((i) => i !== id) })
                          }
                          className="ml-1 hover:text-red-600"
                          aria-label={`Remove ${p.code}`}
                        >
                          ×
                        </button>
                      </Badge>
                    ) : null
                  })}
                </div>
                <div className="flex gap-2 items-center flex-wrap">
                  <div className="w-28 shrink-0">
                    <Input
                      placeholder="New page code"
                      value={newPageCode}
                      onChange={(e) => setNewPageCode(e.target.value)}
                      onKeyDown={(e) =>
                        e.key === 'Enter' && (e.preventDefault(), handleAddNewPage())
                      }
                      className="min-w-0 w-full"
                    />
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleAddNewPage}
                    disabled={!newPageCode.trim() || addingPage}
                  >
                    {addingPage ? 'Adding…' : 'Add page'}
                  </Button>
                </div>
              </div>
            </div>
          </>
        )}
      </form>
    </Modal>
  )
}
