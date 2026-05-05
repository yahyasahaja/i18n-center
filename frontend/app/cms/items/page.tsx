'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { fetchApplications } from '@/store/slices/applicationSlice'
import { useAppContext } from '@/context/AppContext'
import { cmsApi, type CmsItem, type CmsTemplate } from '@/services/api'
import toast from 'react-hot-toast'
import { Plus, Edit, Trash2, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'

export default function CmsItemsPage() {
  const router = useRouter()
  const dispatch = useAppDispatch()
  const { applicationId, push } = useAppContext()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const [items, setItems] = useState<CmsItem[]>([])
  const [templates, setTemplates] = useState<CmsTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [showModal, setShowModal] = useState(false)
  const [editingItem, setEditingItem] = useState<CmsItem | null>(null)
  const [formIdentifier, setFormIdentifier] = useState('')
  const [formName, setFormName] = useState('')
  const [formDescription, setFormDescription] = useState('')
  const [formTemplateId, setFormTemplateId] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) { router.replace('/login'); return }
    dispatch(fetchApplications())
  }, [router, dispatch])

  useEffect(() => {
    if (!applicationId) return
    loadData()
  }, [applicationId]) // eslint-disable-line react-hooks/exhaustive-deps

  const loadData = async () => {
    if (!applicationId) return
    setLoading(true)
    try {
      const [itemsData, templatesData] = await Promise.all([
        cmsApi.listItems(applicationId),
        cmsApi.listTemplates(applicationId),
      ])
      setItems(itemsData)
      setTemplates(templatesData)
    } catch {
      toast.error('Failed to load CMS content')
    } finally {
      setLoading(false)
    }
  }

  const openCreate = () => {
    setEditingItem(null)
    setFormIdentifier(''); setFormName(''); setFormDescription('')
    setFormTemplateId(templates[0]?.id || '')
    setShowModal(true)
  }

  const openEdit = (item: CmsItem) => {
    setEditingItem(item)
    setFormIdentifier(item.identifier); setFormName(item.name)
    setFormDescription(item.description || ''); setFormTemplateId(item.template_id)
    setShowModal(true)
  }

  const handleSave = async () => {
    if (!applicationId || !formIdentifier.trim() || !formName.trim() || !formTemplateId) {
      toast.error('Identifier, name, and template are required')
      return
    }
    setSaving(true)
    try {
      if (editingItem) {
        await cmsApi.updateItem(editingItem.id, { name: formName, description: formDescription, template_id: formTemplateId })
        toast.success('CMS item updated')
      } else {
        await cmsApi.createItem(applicationId, {
          identifier: formIdentifier, name: formName,
          description: formDescription, template_id: formTemplateId,
        })
        toast.success('CMS item created')
      }
      setShowModal(false)
      loadData()
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to save CMS item')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (item: CmsItem) => {
    if (!confirm(`Delete "${item.name}" and all its localizations? This cannot be undone.`)) return
    try {
      await cmsApi.deleteItem(item.id)
      toast.success('CMS item deleted')
      loadData()
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to delete')
    }
  }

  if (!isAuthenticated) return null
  const canManage = user?.role === 'super_admin' || user?.role === 'operator'

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-gray-900">CMS Content</h1>
          {canManage && templates.length > 0 && (
            <Button variant="primary" onClick={openCreate}>
              <Plus className="w-4 h-4 mr-2" />New CMS Item
            </Button>
          )}
        </div>

        {!applicationId && (
          <p className="text-sm text-gray-500">Select an application in the sidebar.</p>
        )}
        {applicationId && templates.length === 0 && !loading && (
          <p className="text-sm text-yellow-600">No CMS templates found. Create a template first before adding content items.</p>
        )}

        <Card>
          {loading ? (
            <div className="text-center py-8 text-gray-500">Loading…</div>
          ) : items.length === 0 ? (
            <div className="text-center py-8 text-gray-500">No CMS items yet.</div>
          ) : (
            <Table headers={['Identifier', 'Name', 'Template', 'Description', 'Actions']}>
              {items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell><span className="font-mono text-sm text-gray-700">{item.identifier}</span></TableCell>
                  <TableCell><span className="font-medium">{item.name}</span></TableCell>
                  <TableCell>
                    <span className="text-sm text-gray-600">
                      {item.template?.name || templates.find(t => t.id === item.template_id)?.name || item.template_id}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-sm text-gray-500 max-w-xs truncate block">
                      {item.description || '—'}
                    </span>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Button variant="primary" size="sm" onClick={() => push(`/cms/items/${item.id}`)}>
                        <ArrowRight className="w-4 h-4 mr-1" />Manage
                      </Button>
                      {canManage && (
                        <>
                          <Button variant="outline" size="sm" onClick={() => openEdit(item)}>
                            <Edit className="w-4 h-4" />
                          </Button>
                          <Button variant="danger" size="sm" onClick={() => handleDelete(item)}>
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
          onClose={() => setShowModal(false)}
          title={editingItem ? 'Edit CMS Item' : 'New CMS Item'}
          footer={
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => setShowModal(false)}>Cancel</Button>
              <Button variant="primary" onClick={handleSave} isLoading={saving}>Save</Button>
            </div>
          }
        >
          <div className="space-y-4">
            {!editingItem && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Identifier *</label>
                <input className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-primary-500"
                  value={formIdentifier} onChange={e => setFormIdentifier(e.target.value)} placeholder="flash_sale_banner" />
                <p className="text-xs text-gray-500 mt-1">Unique identifier used by your app to fetch this content.</p>
              </div>
            )}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name *</label>
              <input className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
                value={formName} onChange={e => setFormName(e.target.value)} placeholder="Flash Sale Banner" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Template *</label>
              <select className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
                value={formTemplateId} onChange={e => setFormTemplateId(e.target.value)}>
                <option value="">Select template…</option>
                {templates.map(t => <option key={t.id} value={t.id}>{t.name} ({t.code})</option>)}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
              <input className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
                value={formDescription} onChange={e => setFormDescription(e.target.value)} placeholder="Optional" />
            </div>
          </div>
        </Modal>
      </div>
    </Layout>
  )
}
