'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter, useParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { fetchApplications } from '@/store/slices/applicationSlice'
import { cmsApi, type CmsItem, type CmsLocalization, type CmsTemplateField } from '@/services/api'
import toast from 'react-hot-toast'
import { Save, ChevronRight, Languages, Upload, RotateCcw, History, ChevronsRight } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import { RichTextEditor } from '@/components/RichTextEditor'
import dynamic from 'next/dynamic'

const CodeEditor = dynamic(() => import('@/components/CodeEditor').then(m => ({ default: m.CodeEditor })), { ssr: false })

const STAGE_VALUES = ['draft', 'staging', 'production'] as const
type Stage = typeof STAGE_VALUES[number]

const STAGE_COLORS: Record<Stage, 'warning' | 'info' | 'success'> = {
  draft: 'warning',
  staging: 'info',
  production: 'success',
}

export default function CmsItemPage() {
  const router = useRouter()
  const params = useParams()
  const itemId = params.id as string
  const dispatch = useAppDispatch()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const { applications } = useAppSelector((state) => state.applications)

  const [item, setItem] = useState<CmsItem | null>(null)
  const [selectedLocale, setSelectedLocale] = useState('en')
  const [selectedStage, setSelectedStage] = useState<Stage>('draft')
  const [localization, setLocalization] = useState<CmsLocalization | null>(null)
  const [formData, setFormData] = useState<Record<string, any>>({})
  const [originalData, setOriginalData] = useState<Record<string, any>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [translating, setTranslating] = useState(false)
  const [translatingTo, setTranslatingTo] = useState<string | null>(null)
  const [enabledLocales, setEnabledLocales] = useState<string[]>([])
  const [showVersionModal, setShowVersionModal] = useState(false)
  const [versions, setVersions] = useState<CmsLocalization[]>([])
  const [backfilling, setBackfilling] = useState(false)
  const [backfillProgress, setBackfillProgress] = useState<Record<string, 'pending'|'completed'|'failed'>>({})
  const [showBackfillModal, setShowBackfillModal] = useState(false)

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) { router.replace('/login'); return }
    dispatch(fetchApplications())
  }, [router, dispatch])

  useEffect(() => {
    if (!itemId) return
    loadItem()
  }, [itemId]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (item && applications.length > 0) {
      const app = applications.find(a => a.id === item.application_id)
      if (app?.enabled_languages) {
        setEnabledLocales(app.enabled_languages)
        if (!app.enabled_languages.includes(selectedLocale)) {
          setSelectedLocale(app.enabled_languages[0] || 'en')
        }
      }
    }
  }, [item, applications]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (item) loadLocalization()
  }, [selectedLocale, selectedStage, item]) // eslint-disable-line react-hooks/exhaustive-deps

  const loadItem = async () => {
    setLoading(true)
    try {
      const data = await cmsApi.getItem(itemId)
      setItem(data)
    } catch {
      toast.error('CMS item not found')
      router.replace('/cms/items')
    } finally {
      setLoading(false)
    }
  }

  const loadLocalization = useCallback(async () => {
    if (!item) return
    try {
      const loc = await cmsApi.getLocalization(itemId, selectedLocale, selectedStage)
      setLocalization(loc)
      setFormData({ ...loc.data })
      setOriginalData({ ...loc.data })
    } catch {
      // No localization yet — start with empty form based on template fields
      const empty: Record<string, any> = {}
      item.template?.fields?.forEach(f => { empty[f.key] = f.value_type === 'json' ? {} : '' })
      setLocalization(null)
      setFormData(empty)
      setOriginalData(empty)
    }
  }, [item, itemId, selectedLocale, selectedStage])

  const hasUnsavedChanges = () => JSON.stringify(formData) !== JSON.stringify(originalData)

  const handleSave = async () => {
    setSaving(true)
    try {
      await cmsApi.saveLocalization(itemId, selectedLocale, selectedStage, formData)
      toast.success('Saved')
      loadLocalization()
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const handleTranslateFrom = async (sourceLocale: string) => {
    if (hasUnsavedChanges()) { toast.error('Save changes first'); return }
    if (!confirm(`Translate current locale (${selectedLocale.toUpperCase()}) from ${sourceLocale.toUpperCase()}? This will overwrite the current content.`)) return
    setTranslating(true)
    try {
      const { job_id } = await cmsApi.translateLocalization(itemId, sourceLocale, selectedLocale, selectedStage)
      toast.loading(`Translating from ${sourceLocale.toUpperCase()}…`, { id: job_id })
      // Poll job status
      let attempts = 0
      const poll = setInterval(async () => {
        attempts++
        if (attempts > 120) { clearInterval(poll); toast.error('Translation timed out', { id: job_id }); setTranslating(false); return }
        try {
          const job = await cmsApi.getCmsTranslateJobStatus(job_id)
          if (job.status === 'completed') {
            clearInterval(poll)
            toast.success(`Translated from ${sourceLocale.toUpperCase()}`, { id: job_id })
            loadLocalization()
            setTranslating(false)
          } else if (job.status === 'failed') {
            clearInterval(poll)
            toast.error(`Translation failed: ${job.error_message}`, { id: job_id })
            setTranslating(false)
          }
        } catch { clearInterval(poll); setTranslating(false) }
      }, 3000)
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to start translation')
      setTranslating(false)
    }
  }

  const handleTranslateTo = async (targetLocale: string) => {
    if (hasUnsavedChanges()) { toast.error('Save changes first'); return }
    if (!confirm(`Translate current ${selectedLocale.toUpperCase()} content → ${targetLocale.toUpperCase()}? This will overwrite the ${targetLocale.toUpperCase()} content.`)) return
    setTranslatingTo(targetLocale)
    try {
      const { job_id } = await cmsApi.translateLocalization(itemId, selectedLocale, targetLocale, selectedStage)
      toast.loading(`Translating to ${targetLocale.toUpperCase()}…`, { id: job_id })
      let attempts = 0
      const poll = setInterval(async () => {
        attempts++
        if (attempts > 120) { clearInterval(poll); toast.error('Translation timed out', { id: job_id }); setTranslatingTo(null); return }
        try {
          const job = await cmsApi.getCmsTranslateJobStatus(job_id)
          if (job.status === 'completed') {
            clearInterval(poll)
            toast.success(`Translated to ${targetLocale.toUpperCase()}`, { id: job_id })
            setTranslatingTo(null)
          } else if (job.status === 'failed') {
            clearInterval(poll)
            toast.error(`Translation failed: ${job.error_message}`, { id: job_id })
            setTranslatingTo(null)
          }
        } catch { clearInterval(poll); setTranslatingTo(null) }
      }, 3000)
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to start translation')
      setTranslatingTo(null)
    }
  }

  const handleDeploy = async (fromStage: Stage, toStage: Stage) => {
    if (!confirm(`Deploy ${selectedLocale.toUpperCase()} from ${fromStage} → ${toStage}?`)) return
    try {
      await cmsApi.deployLocalization(itemId, selectedLocale, fromStage, toStage)
      toast.success(`Deployed to ${toStage}`)
      loadLocalization()
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to deploy')
    }
  }

  const openVersionHistory = async () => {
    try {
      const data = await cmsApi.listVersions(itemId, selectedLocale, selectedStage)
      setVersions(data)
      setShowVersionModal(true)
    } catch { toast.error('Failed to load versions') }
  }

  const handleRevert = async (version: number) => {
    if (!confirm(`Revert to version ${version}? A new version will be created.`)) return
    try {
      await cmsApi.revertLocalization(itemId, selectedLocale, selectedStage, version)
      toast.success(`Reverted to version ${version}`)
      setShowVersionModal(false)
      loadLocalization()
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to revert')
    }
  }

  const handleBackfill = async () => {
    const targets = enabledLocales.filter(l => l !== selectedLocale)
    if (targets.length === 0) return
    if (hasUnsavedChanges()) { toast.error('Save changes first'); return }
    if (!confirm(`Translate ${selectedLocale.toUpperCase()} → all other locales (${targets.map(l => l.toUpperCase()).join(', ')})? Existing content will be overwritten.`)) return

    setBackfilling(true)
    const initial: Record<string, 'pending'|'completed'|'failed'> = {}
    targets.forEach(l => { initial[l] = 'pending' })
    setBackfillProgress(initial)
    setShowBackfillModal(true)

    try {
      const { job_ids } = await cmsApi.backfillLocalizations(itemId, selectedLocale, targets, selectedStage)

      // Poll all jobs in parallel
      const intervals: NodeJS.Timeout[] = []
      job_ids.forEach((jobId, idx) => {
        const locale = targets[idx]
        let attempts = 0
        const iv = setInterval(async () => {
          attempts++
          if (attempts > 120) {
            clearInterval(iv)
            setBackfillProgress(prev => ({ ...prev, [locale]: 'failed' }))
            return
          }
          try {
            const job = await cmsApi.getCmsTranslateJobStatus(jobId)
            if (job.status === 'completed') {
              clearInterval(iv)
              setBackfillProgress(prev => ({ ...prev, [locale]: 'completed' }))
            } else if (job.status === 'failed') {
              clearInterval(iv)
              setBackfillProgress(prev => ({ ...prev, [locale]: 'failed' }))
            }
          } catch { clearInterval(iv) }
        }, 3000)
        intervals.push(iv)
      })

      // When all done, mark backfill complete
      const checkDone = setInterval(() => {
        setBackfillProgress(prev => {
          const done = Object.values(prev).every(s => s !== 'pending')
          if (done) {
            clearInterval(checkDone)
            intervals.forEach(iv => clearInterval(iv))
            setBackfilling(false)
            const failed = Object.entries(prev).filter(([, s]) => s === 'failed').map(([l]) => l.toUpperCase())
            if (failed.length === 0) toast.success('All locales translated')
            else toast.error(`Failed: ${failed.join(', ')}`)
          }
          return prev
        })
      }, 1000)
    } catch (e: any) {
      toast.error(e.response?.data?.error || 'Failed to start backfill')
      setBackfilling(false)
    }
  }

  const renderFieldEditor = (field: CmsTemplateField) => {
    const value = formData[field.key]
    const onChange = (val: any) => setFormData(prev => ({ ...prev, [field.key]: val }))

    switch (field.value_type) {
      case 'text':
        return (
          <input
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500"
            value={value || ''}
            onChange={e => onChange(e.target.value)}
            placeholder={`Enter ${field.label.toLowerCase()}…`}
          />
        )
      case 'textarea':
        return (
          <textarea
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary-500 min-h-[80px]"
            value={value || ''}
            onChange={e => onChange(e.target.value)}
            placeholder={`Enter ${field.label.toLowerCase()}…`}
          />
        )
      case 'rich_text':
        return (
          <RichTextEditor
            value={value || ''}
            onChange={onChange}
            placeholder={`Enter ${field.label.toLowerCase()}…`}
          />
        )
      case 'json':
        return (
          <CodeEditor
            value={typeof value === 'string' ? value : JSON.stringify(value, null, 2) || '{}'}
            onChange={(v) => { try { onChange(JSON.parse(v ?? '')); } catch { onChange(v); } }}
            language="json"
            height="160px"
          />
        )
      default:
        return null
    }
  }

  if (!isAuthenticated) return null
  if (loading) return <Layout><div className="text-center py-8 text-gray-500">Loading…</div></Layout>
  if (!item) return null

  const canManage = user?.role === 'super_admin' || user?.role === 'operator'
  const fields = item.template?.fields || []

  return (
    <Layout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <button onClick={() => router.push('/cms/items')} className="hover:text-gray-900">CMS Content</button>
          <ChevronRight className="w-4 h-4" />
          <span className="text-gray-900 font-medium">{item.name}</span>
          <span className="font-mono text-xs">({item.identifier})</span>
        </div>

        {/* Locale + Stage selectors */}
        <div className="flex flex-wrap gap-4 items-center">
          {/* Locale tabs */}
          <div className="flex gap-1 flex-wrap">
            {enabledLocales.map(loc => (
              <button key={loc}
                className={`px-3 py-1.5 text-sm rounded-full border transition-colors ${selectedLocale === loc ? 'bg-primary-50 border-primary-500 text-primary-600 font-medium' : 'border-gray-200 text-gray-600 hover:bg-gray-50'}`}
                onClick={() => setSelectedLocale(loc)}>
                {loc.toUpperCase()}
              </button>
            ))}
          </div>
          {/* Stage tabs */}
          <div className="flex gap-1">
            {STAGE_VALUES.map((s, idx) => (
              <div key={s} className="flex items-center">
                <button
                  className={`px-3 py-1.5 text-sm rounded-md border transition-colors ${selectedStage === s ? 'bg-primary-50 border-primary-500 text-primary-600 font-medium' : 'border-gray-200 text-gray-600 hover:bg-gray-50'}`}
                  onClick={() => setSelectedStage(s)}>
                  {s.charAt(0).toUpperCase() + s.slice(1)}
                </button>
                {idx < STAGE_VALUES.length - 1 && <ChevronRight className="w-4 h-4 text-gray-300 mx-0.5" />}
              </div>
            ))}
          </div>
        </div>

        <Card>
          {/* Actions bar */}
          <div className="border-b border-gray-200 pb-3 mb-4 flex flex-wrap gap-2 items-center justify-between">
            <div className="flex gap-2">
              <Button variant="primary" size="sm" onClick={handleSave} isLoading={saving}
                disabled={!hasUnsavedChanges()}>
                <Save className="w-4 h-4 mr-1" />Save
              </Button>
              <Button variant="outline" size="sm" onClick={openVersionHistory}>
                <History className="w-4 h-4 mr-1" />Versions
              </Button>
            </div>
            {canManage && (
              <div className="flex gap-2 items-center">
                {selectedStage === 'draft' && (
                  <Button variant="outline" size="sm" onClick={() => handleDeploy('draft', 'staging')}>
                    <Upload className="w-4 h-4 mr-1" />Deploy to Staging
                  </Button>
                )}
                {selectedStage === 'staging' && (
                  <Button variant="outline" size="sm" onClick={() => handleDeploy('staging', 'production')}>
                    <Upload className="w-4 h-4 mr-1" />Deploy to Production
                  </Button>
                )}
              </div>
            )}
          </div>

          {/* Translate from / to rows */}
          {enabledLocales.filter(l => l !== selectedLocale).length > 0 && (
            <div className="space-y-2 mb-4 pb-3 border-b border-gray-200">
              {/* Translate from */}
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm text-gray-500 shrink-0 w-28">Translate from:</span>
                {enabledLocales.filter(l => l !== selectedLocale).map(loc => (
                  <Button key={loc} variant="outline" size="sm" onClick={() => handleTranslateFrom(loc)}
                    disabled={translating || !!translatingTo} isLoading={translating}>
                    <Languages className="w-4 h-4 mr-1" />{loc.toUpperCase()}
                  </Button>
                ))}
                {translating && <span className="text-xs text-gray-400 animate-pulse">Translating…</span>}
              </div>
              {/* Translate to */}
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm text-gray-500 shrink-0 w-28">Translate to:</span>
                {enabledLocales.filter(l => l !== selectedLocale).map(loc => (
                  <Button key={loc} variant="outline" size="sm" onClick={() => handleTranslateTo(loc)}
                    disabled={translating || !!translatingTo || backfilling} isLoading={translatingTo === loc}>
                    <Languages className="w-4 h-4 mr-1" />{loc.toUpperCase()}
                  </Button>
                ))}
                <Button variant="outline" size="sm" onClick={handleBackfill}
                  disabled={translating || !!translatingTo || backfilling} isLoading={backfilling}>
                  <ChevronsRight className="w-4 h-4 mr-1" />All
                </Button>
                {translatingTo && <span className="text-xs text-gray-400 animate-pulse">Translating to {translatingTo.toUpperCase()}…</span>}
              </div>
            </div>
          )}

          {/* Field editors */}
          {fields.length === 0 ? (
            <p className="text-gray-400 text-sm">This template has no fields defined.</p>
          ) : (
            <div className="space-y-6">
              {fields.map(field => (
                <div key={field.key}>
                  <div className="flex items-center gap-2 mb-1.5">
                    <label className="text-sm font-medium text-gray-700">{field.label}</label>
                    <span className="text-xs font-mono text-gray-400">{field.key}</span>
                    <Badge variant={field.value_type === 'rich_text' ? 'success' : field.value_type === 'json' ? 'warning' : 'secondary'} size="sm">
                      {field.value_type}
                    </Badge>
                    {field.required && <Badge variant="warning" size="sm">required</Badge>}
                  </div>
                  {renderFieldEditor(field)}
                </div>
              ))}
            </div>
          )}

          {localization && (
            <p className="text-xs text-gray-400 mt-4">
              Current: version {localization.version} · stage {localization.stage}
              {localization.source_locale ? ` · translated from ${localization.source_locale.toUpperCase()}` : ''}
            </p>
          )}
        </Card>

        {/* Backfill Progress Modal */}
        <Modal isOpen={showBackfillModal} onClose={() => { if (!backfilling) setShowBackfillModal(false) }}
          title={`Translating from ${selectedLocale.toUpperCase()} → All Locales`}
          footer={
            <Button variant="outline" onClick={() => setShowBackfillModal(false)} disabled={backfilling}>
              {backfilling ? 'In progress…' : 'Close'}
            </Button>
          }>
          <div className="space-y-2">
            {Object.entries(backfillProgress).map(([locale, status]) => (
              <div key={locale} className="flex items-center justify-between p-2.5 bg-gray-50 rounded-md">
                <span className="text-sm font-medium">{locale.toUpperCase()}</span>
                <Badge variant={status === 'completed' ? 'success' : status === 'failed' ? 'danger' : 'secondary'} size="sm">
                  {status === 'pending' ? '⏳ translating…' : status === 'completed' ? '✓ done' : '✕ failed'}
                </Badge>
              </div>
            ))}
          </div>
        </Modal>

        {/* Version History Modal */}
        <Modal isOpen={showVersionModal} onClose={() => setShowVersionModal(false)}
          title={`Version History — ${selectedLocale.toUpperCase()} / ${selectedStage}`}
          footer={<Button variant="outline" onClick={() => setShowVersionModal(false)}>Close</Button>}>
          {versions.length === 0 ? (
            <p className="text-gray-500 text-sm">No versions yet.</p>
          ) : (
            <div className="space-y-2 max-h-96 overflow-y-auto">
              {versions.map(v => (
                <div key={v.id} className="flex items-center justify-between p-3 bg-gray-50 rounded-md">
                  <div>
                    <span className="font-medium text-sm">v{v.version}</span>
                    <span className="text-xs text-gray-500 ml-2">
                      {new Date(v.created_at).toLocaleString()}
                    </span>
                    {v.source_locale && (
                      <span className="text-xs text-gray-400 ml-2">from {v.source_locale.toUpperCase()}</span>
                    )}
                  </div>
                  <Button variant="outline" size="sm" onClick={() => handleRevert(v.version)}
                    disabled={v.version === localization?.version}>
                    <RotateCcw className="w-3 h-3 mr-1" />Revert
                  </Button>
                </div>
              ))}
            </div>
          )}
        </Modal>
      </div>
    </Layout>
  )
}
