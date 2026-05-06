'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { fetchApplications } from '@/store/slices/applicationSlice'
import { useAppContext } from '@/context/AppContext'
import { cmsApi, type CmsTemplate, type CmsTemplateField } from '@/services/api'
import toast from 'react-hot-toast'
import { Plus, Edit, Trash2, ChevronDown, ChevronUp } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Modal } from '@/components/ui/Modal'
import { Badge } from '@/components/ui/Badge'

const VALUE_TYPE_COLORS: Record<string, 'info' | 'success' | 'warning' | 'secondary'> = {
  text: 'info',
  textarea: 'secondary',
  rich_text: 'success',
  json: 'warning',
}

const EMPTY_FIELD: CmsTemplateField = { key: '', label: '', value_type: 'text', required: false, sort_order: 0 }

export default function CmsTemplatesPage() {
  const router = useRouter()
  const dispatch = useAppDispatch()
  const { applicationId } = useAppContext()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const [templates, setTemplates] = useState<CmsTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())
  const [showModal, setShowModal] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<CmsTemplate | null>(null)
  const [formName, setFormName] = useState('')
  const [formCode, setFormCode] = useState('')
  const [formDescription, setFormDescription] = useState('')
  const [formFields, setFormFields] = useState<CmsTemplateField[]>([{ ...EMPTY_FIELD }])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) { router.replace('/login'); return }
    dispatch(fetchApplications())
  }, [router, dispatch])

  useEffect(() => {
    if (!applicationId) return
    loadTemplates()
  }, [applicationId]) // eslint-disable-line react-hooks/exhaustive-deps

  const loadTemplates = async () => {
    if (!applicationId) return
    setLoading(true)
    try {
      const data = await cmsApi.listTemplates(applicationId)
      setTemplates(data)
    } catch {
      toast.error('Failed to load CMS templates')
    } finally {
      setLoading(false)
    }
  }

  const openCreate = () => {
    setEditingTemplate(null)
    setFormName(''); setFormCode(''); setFormDescription('')
    setFormFields([{ ...EMPTY_FIELD }])
    setShowModal(true)
  }

  const openEdit = (tmpl: CmsTemplate) => {
    setEditingTemplate(tmpl)
    setFormName(tmpl.name); setFormCode(tmpl.code); setFormDescription(tmpl.description || '')
    setFormFields(tmpl.fields.length > 0 ? tmpl.fields.map(f => ({ ...f })) : [{ ...EMPTY_FIELD }])
    setShowModal(true)
  }

  const handleSave = async () => {
    if (!applicationId || !formName.trim() || !formCode.trim()) {
      toast.error('Name and code are required')
      return
    }
    // Reassign sort_order by current array position so the backend stores the displayed order.
    const validFields = formFields
      .filter(f => f.key.trim() && f.label.trim())
      .map((f, idx) => ({ ...f, sort_order: idx }))
    setSaving(true)
    try {
      if (editingTemplate) {
        await cmsApi.updateTemplate(editingTemplate.id, { name: formName, description: formDescription, fields: validFields })
        toast.success('Template updated')
      } else {
        await cmsApi.createTemplate(applicationId, { name: formName, code: formCode, description: formDescription, fields: validFields })
        toast.success('Template created')
      }
      setShowModal(false)
      loadTemplates()
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to save template')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (tmpl: CmsTemplate) => {
    if (!confirm(`Delete template "${tmpl.name}"? This cannot be undone.`)) return
    try {
      await cmsApi.deleteTemplate(tmpl.id)
      toast.success('Template deleted')
      loadTemplates()
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to delete template')
    }
  }

  const addField = () => setFormFields(prev => [...prev, { ...EMPTY_FIELD, sort_order: prev.length }])
  const removeField = (idx: number) => setFormFields(prev => prev.filter((_, i) => i !== idx))
  const updateField = (idx: number, key: keyof CmsTemplateField, value: any) =>
    setFormFields(prev => prev.map((f, i) => i === idx ? { ...f, [key]: value } : f))
  const moveField = (idx: number, dir: -1 | 1) =>
    setFormFields(prev => {
      const next = [...prev]
      const target = idx + dir
      if (target < 0 || target >= next.length) return prev
      ;[next[idx], next[target]] = [next[target], next[idx]]
      return next
    })

  const toggleExpand = (id: string) =>
    setExpandedIds(prev => { const s = new Set(prev); s.has(id) ? s.delete(id) : s.add(id); return s })

  if (!isAuthenticated) return null
  const canManage = user?.role === 'super_admin' || user?.role === 'operator'

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-gray-900">CMS Templates</h1>
          {canManage && (
            <Button variant="primary" onClick={openCreate}>
              <Plus className="w-4 h-4 mr-2" />New Template
            </Button>
          )}
        </div>

        {!applicationId && (
          <p className="text-sm text-gray-500">Select an application in the sidebar to manage CMS templates.</p>
        )}

        <Card>
          {loading ? (
            <div className="text-center py-8 text-gray-500">Loading…</div>
          ) : templates.length === 0 ? (
            <div className="text-center py-8 text-gray-500">No CMS templates yet. Create one to get started.</div>
          ) : (
            <div className="divide-y divide-gray-200">
              {templates.map((tmpl) => (
                <div key={tmpl.id} className="p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3 cursor-pointer" onClick={() => toggleExpand(tmpl.id)}>
                      {expandedIds.has(tmpl.id) ? <ChevronUp className="w-4 h-4 text-gray-400" /> : <ChevronDown className="w-4 h-4 text-gray-400" />}
                      <div>
                        <span className="font-medium text-gray-900">{tmpl.name}</span>
                        <span className="ml-2 font-mono text-xs text-gray-500">{tmpl.code}</span>
                        {tmpl.description && <p className="text-sm text-gray-500 mt-0.5">{tmpl.description}</p>}
                      </div>
                      <Badge variant="secondary" size="sm">{tmpl.fields.length} field{tmpl.fields.length !== 1 ? 's' : ''}</Badge>
                    </div>
                    {canManage && (
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={() => openEdit(tmpl)}><Edit className="w-4 h-4" /></Button>
                        <Button variant="danger" size="sm" onClick={() => handleDelete(tmpl)}><Trash2 className="w-4 h-4" /></Button>
                      </div>
                    )}
                  </div>
                  {expandedIds.has(tmpl.id) && tmpl.fields.length > 0 && (
                    <div className="mt-3 ml-7 grid grid-cols-1 gap-1.5">
                      {tmpl.fields.map((f) => (
                        <div key={f.id} className="flex items-center gap-2 text-sm">
                          <span className="font-mono text-gray-700 w-32 truncate">{f.key}</span>
                          <span className="text-gray-500">{f.label}</span>
                          <Badge variant={VALUE_TYPE_COLORS[f.value_type] || 'secondary'} size="sm">{f.value_type}</Badge>
                          {f.required && <Badge variant="warning" size="sm">required</Badge>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </Card>

        {/* Create / Edit Modal */}
        <Modal
          isOpen={showModal}
          onClose={() => setShowModal(false)}
          title={editingTemplate ? 'Edit Template' : 'New CMS Template'}
          footer={
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => setShowModal(false)}>Cancel</Button>
              <Button variant="primary" onClick={handleSave} isLoading={saving}>Save</Button>
            </div>
          }
        >
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name *</label>
              <input className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
                value={formName} onChange={e => setFormName(e.target.value)} placeholder="Category Detail" />
            </div>
            {!editingTemplate && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Code *</label>
                <input className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-primary-500"
                  value={formCode} onChange={e => setFormCode(e.target.value.toLowerCase().replace(/\s+/g, '_'))} placeholder="category_detail" />
              </div>
            )}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
              <input className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
                value={formDescription} onChange={e => setFormDescription(e.target.value)} placeholder="Optional description" />
            </div>

            <div>
              <div className="flex items-center justify-between mb-1">
                <label className="block text-sm font-medium text-gray-700">Fields</label>
                <Button variant="outline" size="sm" onClick={addField}><Plus className="w-3 h-3 mr-1" />Add</Button>
              </div>
              <p className="text-xs text-gray-400 mb-2">
                Order only affects the edit UI — item data is stored by key name, so reordering never changes existing content.
              </p>
              <div className="space-y-2">
                {formFields.map((f, idx) => (
                  <div key={idx} className="flex gap-2 items-start bg-gray-50 p-2 rounded-md">
                    {/* Reorder arrows */}
                    <div className="flex flex-col gap-0.5 pt-0.5 shrink-0">
                      <button
                        type="button"
                        onClick={() => moveField(idx, -1)}
                        disabled={idx === 0}
                        className="p-0.5 rounded hover:bg-gray-200 disabled:opacity-20 disabled:cursor-not-allowed text-gray-500"
                        title="Move up"
                      >
                        <ChevronUp className="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onClick={() => moveField(idx, 1)}
                        disabled={idx === formFields.length - 1}
                        className="p-0.5 rounded hover:bg-gray-200 disabled:opacity-20 disabled:cursor-not-allowed text-gray-500"
                        title="Move down"
                      >
                        <ChevronDown className="w-3.5 h-3.5" />
                      </button>
                    </div>

                    <div className="flex-1 grid grid-cols-2 gap-2">
                      <input className="border border-gray-300 rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-primary-500"
                        placeholder="field_key" value={f.key} onChange={e => updateField(idx, 'key', e.target.value.toLowerCase().replace(/\s+/g, '_'))} />
                      <input className="border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
                        placeholder="Label" value={f.label} onChange={e => updateField(idx, 'label', e.target.value)} />
                      <select className="border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
                        value={f.value_type} onChange={e => updateField(idx, 'value_type', e.target.value)}>
                        <option value="text">text</option>
                        <option value="textarea">textarea</option>
                        <option value="rich_text">rich_text</option>
                        <option value="json">json</option>
                      </select>
                      <label className="flex items-center gap-1.5 text-sm text-gray-600">
                        <input type="checkbox" checked={f.required} onChange={e => updateField(idx, 'required', e.target.checked)} />
                        Required
                      </label>
                    </div>
                    <Button variant="danger" size="sm" onClick={() => removeField(idx)} disabled={formFields.length === 1}>
                      <Trash2 className="w-3 h-3" />
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </Modal>
      </div>
    </Layout>
  )
}
