'use client'

import React, { useState, useEffect } from 'react'
import { useSearchParams, useRouter, usePathname } from 'next/navigation'
import { Button } from './ui/Button'
import { Card } from './ui/Card'
import { Badge } from './ui/Badge'
import { Modal } from './ui/Modal'
import { CodeEditor } from './CodeEditor'
import { DiffView } from './DiffView'
import { Save, RotateCcw, Download, Upload, Languages, Zap, GitCompare, History } from 'lucide-react'
import toast from 'react-hot-toast'
import { translationApi, exportApi, importApi } from '@/services/api'

interface TranslationEditorProps {
  componentId: string
  componentName: string
  applicationId: string
  enabledLanguages: string[]
  defaultLocale: string
}

const STAGE_VALUES = ['draft', 'staging', 'production']

export const TranslationEditor: React.FC<TranslationEditorProps> = ({
  componentId,
  componentName,
  applicationId,
  enabledLanguages,
  defaultLocale,
}) => {
  const searchParams = useSearchParams()
  const router = useRouter()
  const pathname = usePathname()

  const urlLocale = searchParams.get('locale')
  const urlStage = searchParams.get('stage')
  const initialLocale = urlLocale && enabledLanguages.includes(urlLocale) ? urlLocale : (defaultLocale || 'en')
  const initialStage = urlStage && STAGE_VALUES.includes(urlStage) ? urlStage : 'draft'

  const [selectedLocale, setSelectedLocale] = useState(initialLocale)
  const [selectedStage, setSelectedStage] = useState(initialStage)
  const [translationData, setTranslationData] = useState<Record<string, any>>({})
  const [jsonText, setJsonText] = useState('{}')
  const [originalJsonText, setOriginalJsonText] = useState('{}')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [showDiff, setShowDiff] = useState(false)
  const [showDeployDiff, setShowDeployDiff] = useState(false)
  const [version1Data, setVersion1Data] = useState<Record<string, any> | null>(null)
  const [version2Data, setVersion2Data] = useState<Record<string, any> | null>(null)
  const [deployFromStage, setDeployFromStage] = useState<string>('')
  const [deployToStage, setDeployToStage] = useState<string>('')
  const [deployDraftStagingHasChanges, setDeployDraftStagingHasChanges] = useState<boolean | null>(null)
  const [deployStagingProductionHasChanges, setDeployStagingProductionHasChanges] = useState<boolean | null>(null)
  const [deployingAllLocales, setDeployingAllLocales] = useState(false)
  const [deployCheckKey, setDeployCheckKey] = useState(0)
  const [showSaveDiffModal, setShowSaveDiffModal] = useState(false)
  const [showRevertDiffModal, setShowRevertDiffModal] = useState(false)
  const [showVersionHistoryModal, setShowVersionHistoryModal] = useState(false)
  const [versionHistoryList, setVersionHistoryList] = useState<Array<{ version: number; data: Record<string, any>; created_at: string }>>([])
  const [versionHistoryDiffVersion, setVersionHistoryDiffVersion] = useState<number | null>(null)

  // Check if there are unsaved changes
  const hasUnsavedChanges = () => {
    try {
      const current = JSON.stringify(JSON.parse(jsonText), null, 2)
      const original = JSON.stringify(JSON.parse(originalJsonText), null, 2)
      return current !== original
    } catch {
      // If JSON is invalid, consider it as changed
      return jsonText !== originalJsonText
    }
  }

  // Confirm before resetting changes
  const confirmBeforeReset = (action: () => void, message: string = 'You have unsaved changes. Are you sure you want to continue?') => {
    if (hasUnsavedChanges()) {
      if (!window.confirm(message)) {
        return false
      }
    }
    action()
    return true
  }

  const stages = [
    { value: 'draft', label: 'Draft', color: 'warning' as const },
    { value: 'staging', label: 'Staging', color: 'info' as const },
    { value: 'production', label: 'Production', color: 'success' as const },
  ]

  // Sync state from URL when query string changes (e.g. browser back/forward)
  useEffect(() => {
    const l = searchParams.get('locale')
    const s = searchParams.get('stage')
    if (l && enabledLanguages.includes(l)) setSelectedLocale(l)
    if (s && STAGE_VALUES.includes(s)) setSelectedStage(s)
  }, [searchParams, enabledLanguages])

  // On mount, if URL has no locale/stage, push current state so URL stays in sync
  useEffect(() => {
    if (!searchParams.get('locale') || !searchParams.get('stage')) {
      updateQueryString(selectedLocale, selectedStage)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const updateQueryString = (locale: string, stage: string) => {
    const params = new URLSearchParams(searchParams.toString())
    params.set('locale', locale)
    params.set('stage', stage)
    router.replace(`${pathname}?${params.toString()}`, { scroll: false })
  }

  useEffect(() => {
    // Reset original text when component/locale/stage changes
    // Skip confirm on initial load or when changing locale/stage (user already confirmed if needed)
    loadTranslation(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [componentId, selectedLocale, selectedStage])

  // Check if there are changes to deploy (source stage !== target stage) for current locale
  useEffect(() => {
    if (!componentId || !selectedLocale) return
    const checkDeployAvailability = async () => {
      try {
        if (selectedStage === 'draft') {
          const [draftRes, stagingRes] = await Promise.all([
            translationApi.get(componentId, selectedLocale, 'draft').catch(() => ({ data: {} })),
            translationApi.get(componentId, selectedLocale, 'staging').catch(() => ({ data: {} })),
          ])
          const draftStr = JSON.stringify(draftRes.data || {})
          const stagingStr = JSON.stringify(stagingRes.data || {})
          setDeployDraftStagingHasChanges(draftStr !== stagingStr)
          setDeployStagingProductionHasChanges(null)
        } else if (selectedStage === 'staging') {
          const [stagingRes, prodRes] = await Promise.all([
            translationApi.get(componentId, selectedLocale, 'staging').catch(() => ({ data: {} })),
            translationApi.get(componentId, selectedLocale, 'production').catch(() => ({ data: {} })),
          ])
          const stagingStr = JSON.stringify(stagingRes.data || {})
          const prodStr = JSON.stringify(prodRes.data || {})
          setDeployStagingProductionHasChanges(stagingStr !== prodStr)
          setDeployDraftStagingHasChanges(null)
        } else {
          setDeployDraftStagingHasChanges(null)
          setDeployStagingProductionHasChanges(null)
        }
      } catch {
        setDeployDraftStagingHasChanges(null)
        setDeployStagingProductionHasChanges(null)
      }
    }
    checkDeployAvailability()
  }, [componentId, selectedLocale, selectedStage, originalJsonText, deployCheckKey])

  const loadTranslation = async (skipConfirm = false) => {
    if (!skipConfirm && hasUnsavedChanges()) {
      if (!window.confirm('You have unsaved changes. Are you sure you want to load a different translation? Your changes will be lost.')) {
        return
      }
    }

    setLoading(true)
    try {
      const data = await translationApi.get(componentId, selectedLocale, selectedStage)
      const translation = data.data || {}
      setTranslationData(translation)
      const jsonString = JSON.stringify(translation, null, 2)
      setJsonText(jsonString)
      setOriginalJsonText(jsonString)
      setJsonError(null)
    } catch (error: any) {
      if (error.response?.status === 404) {
        setTranslationData({})
        const emptyJson = '{}'
        setJsonText(emptyJson)
        setOriginalJsonText(emptyJson)
      } else {
        toast.error('Failed to load translation')
      }
    } finally {
      setLoading(false)
    }
  }

  const handleSaveClick = () => {
    const validation = validateJSON(jsonText)
    if (!validation.valid) {
      setJsonError(validation.error || 'Invalid JSON')
      toast.error(validation.error || 'Please fix JSON errors before saving.')
      return
    }
    try {
      JSON.parse(jsonText)
      setJsonError(null)
    } catch {
      setJsonError('Invalid JSON. Please fix syntax errors before saving.')
      toast.error('Invalid JSON. Please fix syntax errors before saving.')
      return
    }
    setVersion1Data(JSON.parse(originalJsonText) || {})
    setVersion2Data(JSON.parse(jsonText) || {})
    setShowSaveDiffModal(true)
  }

  const confirmSaveFromDiff = async () => {
    setShowSaveDiffModal(false)
    let dataToSave: Record<string, any>
    try {
      dataToSave = JSON.parse(jsonText)
    } catch {
      return
    }
    setSaving(true)
    try {
      await translationApi.save(componentId, selectedLocale, selectedStage, dataToSave)
      setTranslationData(dataToSave)
      setOriginalJsonText(JSON.stringify(dataToSave, null, 2))
      toast.success('Translation saved successfully')
      setDeployCheckKey((k) => k + 1)
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to save translation')
    } finally {
      setSaving(false)
    }
  }

  const handleRevertClick = async () => {
    if (hasUnsavedChanges()) {
      toast.error('Save or discard your changes before reverting')
      return
    }
    try {
      const comparison = await translationApi.compare(componentId, selectedLocale, selectedStage)
      if (!comparison.version1 || !comparison.version2) {
        toast.error('No previous version to revert to')
        return
      }
      setVersion1Data(comparison.version1.data || {})
      setVersion2Data(comparison.version2.data || {})
      setShowRevertDiffModal(true)
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to load comparison for revert')
    }
  }

  const confirmRevertFromDiff = async () => {
    setShowRevertDiffModal(false)
    try {
      await translationApi.revert(componentId, selectedLocale, selectedStage)
      toast.success('Translation reverted')
      loadTranslation(true)
      setDeployCheckKey((k) => k + 1)
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to revert translation')
    }
  }

  const loadVersionComparison = async () => {
    try {
      const comparison = await translationApi.compare(componentId, selectedLocale, selectedStage)
      setVersion1Data(comparison.version1?.data || {})
      setVersion2Data(comparison.version2?.data || {})
      setShowDiff(true)
    } catch (error: any) {
      toast.error('Failed to load version comparison')
    }
  }

  const openVersionHistoryModal = async () => {
    try {
      const res = await translationApi.listVersions(componentId, selectedLocale, selectedStage)
      setVersionHistoryList(res.versions || [])
      setVersionHistoryDiffVersion(null)
      setShowVersionHistoryModal(true)
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to load version history')
    }
  }

  const handleDeploy = async (fromStage: string, toStage: string) => {
    // Show diff before deploying
    try {
      // Get source stage data
      const sourceData = await translationApi.get(componentId, selectedLocale, fromStage)
      // Get target stage data (if exists)
      let targetData: any = { data: {} }
      try {
        targetData = await translationApi.get(componentId, selectedLocale, toStage)
      } catch (e) {
        // Target doesn't exist yet, that's fine
      }

      setVersion1Data(targetData.data || {})
      setVersion2Data(sourceData.data || {})
      setDeployFromStage(fromStage)
      setDeployToStage(toStage)
      setShowDeployDiff(true)
    } catch (error: any) {
      toast.error('Failed to load comparison data')
    }
  }

  const confirmDeploy = async () => {
    try {
      await translationApi.deploy(componentId, selectedLocale, deployFromStage, deployToStage)
      toast.success(`Deployed to ${deployToStage}`)
      setShowDeployDiff(false)
      if (selectedStage === deployToStage) {
        loadTranslation(true)
      }
      setDeployCheckKey((k) => k + 1)
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to deploy')
    }
  }

  const handleDeployAllLocales = async (fromStage: string, toStage: string) => {
    if (hasUnsavedChanges()) {
      toast.error('Save changes first before deploying')
      return
    }
    const confirmMsg = `Deploy ${fromStage} → ${toStage} for all locales (${enabledLanguages.join(', ')})?`
    if (!window.confirm(confirmMsg)) return
    setDeployingAllLocales(true)
    try {
      for (const locale of enabledLanguages) {
        try {
          await translationApi.deploy(componentId, locale, fromStage, toStage)
        } catch (err: any) {
          toast.error(`Deploy failed for ${locale}: ${err.response?.data?.error || err.message}`)
        }
      }
      toast.success(`Deploy ${fromStage} → ${toStage} completed for all locales`)
      setDeployCheckKey((k) => k + 1)
      if (selectedStage === toStage) {
        loadTranslation(true)
      }
    } finally {
      setDeployingAllLocales(false)
    }
  }

  const handleAutoTranslate = async (targetLocale: string) => {
    if (hasUnsavedChanges()) {
      toast.error('Save changes first before translate')
      return
    }
    try {
      setLoading(true)
      await translationApi.autoTranslate(
        componentId,
        selectedLocale,
        targetLocale,
        selectedStage
      )
      toast.success(`Translated to ${targetLocale}`)
      if (targetLocale === selectedLocale) {
        loadTranslation()
      }
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to translate')
    } finally {
      setLoading(false)
    }
  }

  const handleBackfill = async () => {
    if (hasUnsavedChanges()) {
      toast.error('Save changes first before translate')
      return
    }
    const missingLocales = enabledLanguages.filter((lang) => lang !== selectedLocale)
    if (missingLocales.length === 0) {
      toast.error('All languages are already translated')
      return
    }

    if (!confirm(`Backfill translations for ${missingLocales.join(', ')}?`)) return

    try {
      setLoading(true)
      await translationApi.backfill(componentId, selectedLocale, missingLocales, selectedStage)
      toast.success('Backfill completed')
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to backfill')
    } finally {
      setLoading(false)
    }
  }

  const handleExport = async () => {
    try {
      const blob = await exportApi.exportComponent(componentId, selectedLocale, selectedStage)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${componentName}_${selectedLocale}_${selectedStage}.json`
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
      toast.success('Exported successfully')
    } catch (error: any) {
      toast.error('Failed to export')
    }
  }

  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    if (hasUnsavedChanges()) {
      if (!window.confirm('You have unsaved changes. Importing will replace them. Are you sure you want to continue?')) {
        // Reset the file input
        event.target.value = ''
        return
      }
    }

    try {
      const text = await file.text()
      const data = JSON.parse(text)
      setTranslationData(data)
      const jsonString = JSON.stringify(data, null, 2)
      setJsonText(jsonString)
      setOriginalJsonText(jsonString) // Update original after import
      setJsonError(null)
      toast.success('File imported. Click Save to apply changes.')
    } catch (error) {
      toast.error('Invalid JSON file')
    } finally {
      // Reset the file input
      event.target.value = ''
    }
  }

  // Validate JSON and check for duplicate keys
  const validateJSON = (jsonString: string): { valid: boolean; error: string | null } => {
    try {
      // First check if it's valid JSON syntax
      JSON.parse(jsonString)

      // Check for duplicate keys in the raw string
      // JSON.parse() accepts duplicates but we want to catch them
      const findDuplicateKeys = (str: string): string[] => {
        const duplicates: string[] = []

        // Simple approach: find all top-level keys in the main object
        // This handles the most common case of duplicate keys at root level
        const trimmed = str.trim()
        if (!trimmed.startsWith('{')) {
          return duplicates
        }

        // Find the main object content (between first { and matching })
        let depth = 0
        let startIdx = -1
        let inString = false
        let escapeNext = false

        for (let i = 0; i < trimmed.length; i++) {
          const char = trimmed[i]

          if (escapeNext) {
            escapeNext = false
            continue
          }

          if (char === '\\') {
            escapeNext = true
            continue
          }

          if (char === '"' && !escapeNext) {
            inString = !inString
            continue
          }

          if (!inString) {
            if (char === '{') {
              depth++
              if (depth === 1) {
                startIdx = i + 1
              }
            } else if (char === '}') {
              depth--
              if (depth === 0 && startIdx >= 0) {
                // Extract object content
                const content = trimmed.substring(startIdx, i)
                // Find all keys in this content
                const keyRegex = /"([^"]+)":/g
                const keys: string[] = []
                let keyMatch

                while ((keyMatch = keyRegex.exec(content)) !== null) {
                  const key = keyMatch[1]
                  if (keys.includes(key) && !duplicates.includes(key)) {
                    duplicates.push(key)
                  } else {
                    keys.push(key)
                  }
                }
                break
              }
            }
          }
        }

        return duplicates
      }

      const duplicateKeys = findDuplicateKeys(jsonString)
      if (duplicateKeys.length > 0) {
        return {
          valid: false,
          error: `Duplicate keys found: ${duplicateKeys.join(', ')}`
        }
      }

      return { valid: true, error: null }
    } catch (error: any) {
      return {
        valid: false,
        error: `Invalid JSON syntax: ${error.message}`
      }
    }
  }

  const handleJsonChange = (value: string) => {
    setJsonText(value)
    // Validate JSON and check for duplicates
    const validation = validateJSON(value)
    if (validation.valid) {
      try {
        const parsed = JSON.parse(value)
        setTranslationData(parsed)
        setJsonError(null)
      } catch {
        // Should not happen if validation passed, but just in case
        setJsonError('Invalid JSON')
      }
    } else {
      setJsonError(validation.error || 'Invalid JSON')
    }
  }

  const currentStage = stages.find((s) => s.value === selectedStage)

  return (
    <div className="space-y-6">
      {/* Header Controls */}
      <Card>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold text-gray-900">{componentName}</h2>
            <div className="flex items-center space-x-2">
              <Button variant="outline" onClick={handleExport} size="sm">
                <Download className="w-4 h-4 mr-2" />
                Export
              </Button>
              <label className="cursor-pointer">
                <span className="inline-flex items-center px-3 py-1.5 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500">
                  <Upload className="w-4 h-4 mr-2" />
                  Import
                </span>
                <input
                  type="file"
                  accept=".json"
                  onChange={handleImport}
                  className="hidden"
                />
              </label>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Locale
              </label>
              <select
                value={selectedLocale}
                onChange={(e) => {
                  if (hasUnsavedChanges()) {
                    if (!window.confirm('You have unsaved changes. Changing locale will discard them. Are you sure you want to continue?')) {
                      return
                    }
                  }
                  const next = e.target.value
                  setSelectedLocale(next)
                  updateQueryString(next, selectedStage)
                }}
                className="block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 bg-white text-gray-900 focus:outline-none focus:ring-primary-500 focus:border-primary-500"
              >
                {enabledLanguages.map((lang) => (
                  <option key={lang} value={lang}>
                    {lang.toUpperCase()}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Stage
              </label>
              <div className="flex items-center space-x-2">
                <select
                  value={selectedStage}
                  onChange={(e) => {
                    if (hasUnsavedChanges()) {
                      if (!window.confirm('You have unsaved changes. Changing stage will discard them. Are you sure you want to continue?')) {
                        return
                      }
                    }
                    const next = e.target.value
                    setSelectedStage(next)
                    updateQueryString(selectedLocale, next)
                  }}
                  className="block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 bg-white text-gray-900 focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                >
                  {stages.map((stage) => (
                    <option key={stage.value} value={stage.value}>
                      {stage.label}
                    </option>
                  ))}
                </select>
                {currentStage && (
                  <Badge variant={currentStage.color}>{currentStage.label}</Badge>
                )}
              </div>
            </div>

            <div className="flex flex-wrap items-end gap-2 min-w-0 md:min-w-[280px]">
              <Button
                variant="primary"
                onClick={handleSaveClick}
                isLoading={saving}
                disabled={!!jsonError || !hasUnsavedChanges()}
                size="sm"
                title={jsonError ? 'Please fix JSON errors before saving' : !hasUnsavedChanges() ? 'No changes to save' : ''}
              >
                <Save className="w-4 h-4 mr-2 shrink-0" />
                Save
              </Button>
              <Button variant="outline" onClick={handleRevertClick} size="sm">
                <RotateCcw className="w-4 h-4 mr-2 shrink-0" />
                Revert
              </Button>
              <Button variant="outline" onClick={loadVersionComparison} size="sm">
                <GitCompare className="w-4 h-4 mr-2 shrink-0" />
                Compare
              </Button>
              <Button variant="outline" onClick={openVersionHistoryModal} size="sm" title="Version history">
                <History className="w-4 h-4 mr-2 shrink-0" />
                <span className="whitespace-nowrap">Version history</span>
              </Button>
            </div>
          </div>

          {/* Auto-translate Actions */}
          <div className="flex items-center space-x-2 pt-2 border-t">
            <span className="text-sm text-gray-600">Translate:</span>
            {enabledLanguages
              .filter((lang) => lang !== selectedLocale)
              .map((lang) => (
                <Button
                  key={lang}
                  variant="outline"
                  size="sm"
                  onClick={() => handleAutoTranslate(lang)}
                  isLoading={loading}
                >
                  <Languages className="w-4 h-4 mr-1" />
                  {lang.toUpperCase()}
                </Button>
              ))}
            <Button
              variant="primary"
              size="sm"
              onClick={handleBackfill}
              isLoading={loading}
            >
              <Zap className="w-4 h-4 mr-2" />
              Backfill All
            </Button>
          </div>

          {/* Deployment Actions */}
          <div className="flex flex-wrap items-center gap-2 pt-2 border-t">
            <span className="text-sm text-gray-600">Deploy:</span>
            {selectedStage === 'draft' && (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeploy('draft', 'staging')}
                  disabled={deployDraftStagingHasChanges === false}
                  title={deployDraftStagingHasChanges === false ? 'No changes to deploy (draft and staging are identical)' : ''}
                >
                  Draft → Staging
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeployAllLocales('draft', 'staging')}
                  disabled={deployDraftStagingHasChanges === false}
                  isLoading={deployingAllLocales}
                  title={deployDraftStagingHasChanges === false ? 'No changes to deploy' : 'Deploy draft to staging for all locales'}
                >
                  Draft → Staging (all locales)
                </Button>
              </>
            )}
            {selectedStage === 'staging' && (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeploy('staging', 'production')}
                  disabled={deployStagingProductionHasChanges === false}
                  title={deployStagingProductionHasChanges === false ? 'No changes to deploy (staging and production are identical)' : ''}
                >
                  Staging → Production
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeployAllLocales('staging', 'production')}
                  disabled={deployStagingProductionHasChanges === false}
                  isLoading={deployingAllLocales}
                  title={deployStagingProductionHasChanges === false ? 'No changes to deploy' : 'Deploy staging to production for all locales'}
                >
                  Staging → Production (all locales)
                </Button>
              </>
            )}
          </div>
        </div>
      </Card>

      {/* JSON Editor */}
      <Card title="Translation Data">
        {loading ? (
          <div className="flex items-center justify-center h-96">
            <div className="text-center">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
              <p className="mt-4 text-gray-600">Loading translation data...</p>
            </div>
          </div>
        ) : (
          <>
            <CodeEditor
              value={jsonText}
              onChange={(value) => handleJsonChange(value || '{}')}
              language="json"
              height="600px"
            />
            {jsonError && (
              <p className="mt-2 text-sm text-red-600">
                Please fix JSON syntax errors before saving
              </p>
            )}
          </>
        )}
      </Card>

      {/* Version Comparison Modal (current vs previous) */}
      <Modal
        isOpen={showDiff}
        onClose={() => setShowDiff(false)}
        title="Version comparison (current vs previous)"
        size="large"
      >
        <DiffView
          oldValue={JSON.stringify(version1Data, null, 2)}
          newValue={JSON.stringify(version2Data, null, 2)}
        />
      </Modal>

      {/* Deploy Comparison Modal */}
      <Modal
        isOpen={showDeployDiff}
        onClose={() => setShowDeployDiff(false)}
        title={`Deploy Comparison: ${deployFromStage} → ${deployToStage}`}
        size="large"
      >
        <div className="space-y-4">
          <DiffView
            oldValue={JSON.stringify(version1Data, null, 2)}
            newValue={JSON.stringify(version2Data, null, 2)}
            title={`Changes from ${deployFromStage} to ${deployToStage}`}
          />
          <div className="flex justify-end space-x-2 pt-4 border-t">
            <Button variant="outline" onClick={() => setShowDeployDiff(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={confirmDeploy}>
              Confirm Deploy
            </Button>
          </div>
        </div>
      </Modal>

      {/* Save diff modal: show before saving */}
      <Modal
        isOpen={showSaveDiffModal}
        onClose={() => setShowSaveDiffModal(false)}
        title="Review changes before save"
        size="large"
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">Left: current saved. Right: your edits (will be saved).</p>
          <DiffView
            oldValue={JSON.stringify(version1Data, null, 2)}
            newValue={JSON.stringify(version2Data, null, 2)}
          />
          <div className="flex justify-end space-x-2 pt-4 border-t">
            <Button variant="outline" onClick={() => setShowSaveDiffModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={confirmSaveFromDiff}>
              Confirm Save
            </Button>
          </div>
        </div>
      </Modal>

      {/* Revert diff modal: show before reverting */}
      <Modal
        isOpen={showRevertDiffModal}
        onClose={() => setShowRevertDiffModal(false)}
        title="Review revert"
        size="large"
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">Left: current version. Right: previous version (revert will restore this).</p>
          <DiffView
            oldValue={JSON.stringify(version1Data, null, 2)}
            newValue={JSON.stringify(version2Data, null, 2)}
          />
          <div className="flex justify-end space-x-2 pt-4 border-t">
            <Button variant="outline" onClick={() => setShowRevertDiffModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={confirmRevertFromDiff}>
              Confirm Revert
            </Button>
          </div>
        </div>
      </Modal>

      {/* Version history modal */}
      <Modal
        isOpen={showVersionHistoryModal}
        onClose={() => setShowVersionHistoryModal(false)}
        title="Version history"
        size="large"
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">Versions (newest first). Click &quot;View changes&quot; to see diff from previous version.</p>
          <div className="max-h-80 overflow-y-auto border rounded p-2 space-y-2">
            {versionHistoryList.length === 0 && <p className="text-gray-500 text-sm">No versions yet.</p>}
            {versionHistoryList.map((v, idx) => (
              <div key={v.version} className="flex items-center justify-between py-2 border-b border-gray-100 last:border-0">
                <span className="text-sm font-medium">Version {v.version}</span>
                <span className="text-xs text-gray-500">
                  {v.created_at ? new Date(v.created_at).toLocaleString() : '—'}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={idx >= versionHistoryList.length - 1}
                  onClick={() => setVersionHistoryDiffVersion(v.version)}
                >
                  View changes
                </Button>
              </div>
            ))}
          </div>
          {versionHistoryDiffVersion != null && (() => {
            const v = versionHistoryList.find((x) => x.version === versionHistoryDiffVersion)
            const prev = versionHistoryList.find((x) => x.version === versionHistoryDiffVersion - 1)
            if (!v || !prev) return null
            return (
              <div className="border-t pt-4">
                <h4 className="text-sm font-medium mb-2">Version {v.version} vs {prev.version}</h4>
                <DiffView
                  oldValue={JSON.stringify(prev.data, null, 2)}
                  newValue={JSON.stringify(v.data, null, 2)}
                />
              </div>
            )
          })()}
        </div>
      </Modal>
    </div>
  )
}

