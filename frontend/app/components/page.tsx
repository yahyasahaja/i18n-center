'use client'

import { useEffect, useState, useCallback, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { useAppContext } from '@/context/AppContext'
import { fetchComponents } from '@/store/slices/componentSlice'
import { fetchApplications } from '@/store/slices/applicationSlice'
import { componentApi, type Tag, type Page } from '@/services/api'
import toast from 'react-hot-toast'
import { Plus, Edit, Trash2, ArrowRight, Search, ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import { Badge } from '@/components/ui/Badge'
import { Input } from '@/components/ui/Input'
import { ComponentFormModal } from '@/components/ComponentFormModal'

export default function ComponentsPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const dispatch = useAppDispatch()
  const { applicationId: contextApplicationId, push } = useAppContext()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const { components, loading, total, page, pageSize, totalPages } = useAppSelector((state) => state.components)
  const { applications } = useAppSelector((state) => state.applications)
  const [showModal, setShowModal] = useState(false)
  const [editingComponent, setEditingComponent] = useState<any>(null)
  const [listModal, setListModal] = useState<{ type: 'tags' | 'pages'; items: { code: string }[]; title: string } | null>(null)
  const [search, setSearch] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const searchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const loadComponents = useCallback((searchVal: string, pg: number) => {
    dispatch(fetchComponents({
      applicationId: contextApplicationId || undefined,
      search: searchVal || undefined,
      page: pg,
      pageSize: 20,
    }))
  }, [dispatch, contextApplicationId])

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      router.replace('/login')
      return
    }
    dispatch(fetchApplications())
    loadComponents(search, currentPage)
  }, [router, dispatch]) // eslint-disable-line react-hooks/exhaustive-deps

  // Reload when application context changes
  useEffect(() => {
    setCurrentPage(1)
    loadComponents(search, 1)
  }, [contextApplicationId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Open edit modal from URL param
  useEffect(() => {
    const editId = searchParams.get('edit')
    if (editId && components.length > 0) {
      const component = components.find((c) => c.id === editId) as any
      if (component) {
        setEditingComponent(component)
        setShowModal(true)
      }
    }
  }, [searchParams, components])

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setSearch(val)
    if (searchTimerRef.current) clearTimeout(searchTimerRef.current)
    searchTimerRef.current = setTimeout(() => {
      setCurrentPage(1)
      loadComponents(val, 1)
    }, 350)
  }

  const handlePageChange = (pg: number) => {
    setCurrentPage(pg)
    loadComponents(search, pg)
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this component?')) return
    try {
      await componentApi.delete(id)
      toast.success('Component deleted')
      loadComponents(search, currentPage)
    } catch (error: any) {
      toast.error(error.message || 'Failed to delete component')
    }
  }

  if (!isAuthenticated) return null

  const canManage = user?.role === 'super_admin' || user?.role === 'operator'

  const getApplicationName = (appId: string) =>
    applications.find((a) => a.id === appId)?.name || appId

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-gray-900">Components</h1>
          {canManage && (
            <Button
              variant="primary"
              onClick={() => { setEditingComponent(null); setShowModal(true) }}
            >
              <Plus className="w-4 h-4 mr-2" />
              New Component
            </Button>
          )}
        </div>

        {contextApplicationId && (
          <p className="text-sm text-gray-600">
            Showing components for: <strong>{getApplicationName(contextApplicationId)}</strong>
            {' '}(change application in the sidebar)
          </p>
        )}
        {!contextApplicationId && (
          <p className="text-sm text-gray-500">Select an application in the sidebar to filter components.</p>
        )}

        {/* Search bar */}
        <div className="relative max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 pointer-events-none" />
          <Input
            type="text"
            placeholder="Search by name or code…"
            value={search}
            onChange={handleSearchChange}
            className="pl-9"
          />
        </div>

        <Card>
          {loading ? (
            <div className="text-center py-8 text-gray-500">Loading...</div>
          ) : components.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              {search ? `No components match "${search}".` : 'No components found. Create one to get started.'}
            </div>
          ) : (
            <>
              <Table headers={['Name', 'Code', 'Tags', 'Pages', 'Description', 'Default Locale', 'Actions']}>
                {components.map((comp) => {
                  const compAny = comp as any
                  const tags = compAny.tags || []
                  const pages = compAny.pages || []
                  const maxShow = 2
                  return (
                    <TableRow key={comp.id}>
                      <TableCell>
                        <div className="font-medium text-gray-900">{comp.name}</div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm font-mono text-gray-600">{comp.code}</div>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap items-center gap-1">
                          {tags.slice(0, maxShow).map((t: Tag) => (
                            <Badge key={t.id} variant="secondary" className="text-xs">{t.code}</Badge>
                          ))}
                          {tags.length > maxShow && (
                            <Button type="button" variant="outline" size="sm" className="h-6 px-1.5 text-xs"
                              onClick={() => setListModal({ type: 'tags', items: tags, title: `Tags for ${comp.name}` })}>
                              +{tags.length - maxShow}
                            </Button>
                          )}
                          {tags.length === 0 && <span className="text-gray-400 text-sm">—</span>}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap items-center gap-1">
                          {pages.slice(0, maxShow).map((p: Page) => (
                            <Badge key={p.id} variant="secondary" className="text-xs">{p.code}</Badge>
                          ))}
                          {pages.length > maxShow && (
                            <Button type="button" variant="outline" size="sm" className="h-6 px-1.5 text-xs"
                              onClick={() => setListModal({ type: 'pages', items: pages, title: `Pages for ${comp.name}` })}>
                              +{pages.length - maxShow}
                            </Button>
                          )}
                          {pages.length === 0 && <span className="text-gray-400 text-sm">—</span>}
                        </div>
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
                          <Button variant="primary" size="sm" onClick={() => push(`/components/${comp.id}/translations`)}>
                            <ArrowRight className="w-4 h-4 mr-1" />
                            Manage
                          </Button>
                          {canManage && (
                            <>
                              <Button variant="outline" size="sm" onClick={() => { setEditingComponent(comp); setShowModal(true) }}>
                                <Edit className="w-4 h-4" />
                              </Button>
                              <Button variant="danger" size="sm" onClick={() => handleDelete(comp.id)}>
                                <Trash2 className="w-4 h-4" />
                              </Button>
                            </>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </Table>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="flex items-center justify-between px-4 py-3 border-t border-gray-200">
                  <p className="text-sm text-gray-600">
                    Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} of {total} components
                  </p>
                  <div className="flex items-center gap-1">
                    <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => handlePageChange(page - 1)}>
                      <ChevronLeft className="w-4 h-4" />
                    </Button>
                    {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => {
                      let pg: number
                      if (totalPages <= 7) pg = i + 1
                      else if (page <= 4) pg = i + 1
                      else if (page >= totalPages - 3) pg = totalPages - 6 + i
                      else pg = page - 3 + i
                      return (
                        <Button
                          key={pg}
                          variant={pg === page ? 'primary' : 'outline'}
                          size="sm"
                          onClick={() => handlePageChange(pg)}
                          className="w-8"
                        >
                          {pg}
                        </Button>
                      )
                    })}
                    <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => handlePageChange(page + 1)}>
                      <ChevronRight className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </Card>

        {listModal && (
          <Modal isOpen={!!listModal} onClose={() => setListModal(null)} title={listModal.title}
            footer={<Button variant="outline" onClick={() => setListModal(null)}>Close</Button>}>
            <ul className="space-y-1.5">
              {listModal.items.map((item: any) => (
                <li key={item.id || item.code} className="text-sm font-mono text-gray-700">{item.code}</li>
              ))}
            </ul>
          </Modal>
        )}

        <ComponentFormModal
          isOpen={showModal}
          onClose={() => { setShowModal(false); setEditingComponent(null) }}
          component={editingComponent}
          applications={applications}
          defaultApplicationId={contextApplicationId || undefined}
          onSaved={() => {
            loadComponents(search, currentPage)
            setShowModal(false)
            setEditingComponent(null)
          }}
        />
      </div>
    </Layout>
  )
}
