'use client'

import { useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Badge } from '@/components/ui/Badge'
import { Upload, Trash2, CheckCircle2, AlertCircle, Loader2, Plus } from 'lucide-react'
import toast from 'react-hot-toast'
import { applicationApi } from '@/services/api'

interface BootstrapModalProps {
  isOpen: boolean
  onClose: () => void
  applicationId: string
  defaultStage?: string
  // Fired after at least one file imports successfully so the parent can
  // refetch components / pending deploys.
  onImported?: () => void
}

type FileEntry = {
  id: string // local UID for React keys
  file: File
  locale: string // inferred from filename, user-editable
  data: unknown | null // parsed JSON, null until parse succeeds
  parseError: string | null
  status: 'pending' | 'uploading' | 'done' | 'failed'
  result: {
    components_created: number
    components_updated: number
    keys_imported: number
    flat_keys_in_common: number
    components: string[]
  } | null
  error: string | null
}

// inferLocaleFromName pulls a sensible locale code from the filename stem:
//   en.json           → "en"
//   id.json           → "id"
//   pt-br.json        → "pt-br"
//   translations.en.json → "en" (last segment before .json wins)
// Falls back to the bare stem when nothing matches.
function inferLocaleFromName(name: string): string {
  const stem = name.replace(/\.json$/i, '')
  // Take the last dot-segment of the stem so multi-part names like
  // `translations.en.json` resolve to `en`. Single-segment stems like
  // `en` resolve to themselves.
  const parts = stem.split('.')
  return parts[parts.length - 1].toLowerCase().trim()
}

function makeId() {
  return `f_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}

export function BootstrapModal({
  isOpen,
  onClose,
  applicationId,
  defaultStage = 'draft',
  onImported,
}: BootstrapModalProps) {
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [stage, setStage] = useState(defaultStage)
  const [busy, setBusy] = useState(false)

  const reset = () => {
    setEntries([])
    setBusy(false)
  }

  const handleClose = () => {
    if (busy) return // prevent closing mid-upload
    reset()
    onClose()
  }

  // Reads + parses each newly selected file in parallel. Parse errors are
  // attached to the row so the user can see which file is malformed before
  // hitting Import — the import button is disabled when any row has an error.
  const ingestFiles = async (fileList: FileList | null) => {
    if (!fileList || fileList.length === 0) return
    const newEntries: FileEntry[] = await Promise.all(
      Array.from(fileList).map(async (file) => {
        const entry: FileEntry = {
          id: makeId(),
          file,
          locale: inferLocaleFromName(file.name),
          data: null,
          parseError: null,
          status: 'pending',
          result: null,
          error: null,
        }
        try {
          const text = await file.text()
          const parsed = JSON.parse(text) as unknown
          if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
            entry.parseError = 'Top-level value must be an object'
          } else {
            entry.data = parsed
          }
        } catch (err) {
          entry.parseError =
            err instanceof Error ? err.message : 'Could not parse JSON'
        }
        return entry
      }),
    )
    setEntries((prev) => [...prev, ...newEntries])
  }

  const removeEntry = (id: string) => {
    setEntries((prev) => prev.filter((e) => e.id !== id))
  }

  const updateLocale = (id: string, locale: string) => {
    setEntries((prev) =>
      prev.map((e) => (e.id === id ? { ...e, locale: locale.trim().toLowerCase() } : e)),
    )
  }

  // canImport: every row has a parsed object AND a non-blank locale AND no
  // duplicate locale within the batch. We block duplicates because the second
  // POST would clobber the first — surface that as a precondition, not a
  // server-side surprise.
  const trimmedLocales = entries.map((e) => e.locale.trim().toLowerCase())
  const hasDuplicateLocale = trimmedLocales.some(
    (l, i) => l && trimmedLocales.indexOf(l) !== i,
  )
  const hasParseError = entries.some((e) => !!e.parseError)
  const hasBlankLocale = entries.some((e) => !e.locale.trim())
  const canImport =
    entries.length > 0 && !hasParseError && !hasBlankLocale && !hasDuplicateLocale && !busy

  const runImport = async () => {
    setBusy(true)
    let success = 0
    // Sequential not parallel — the bootstrap endpoint mutates components and
    // back-to-back imports for different locales benefit from a stable cache
    // state per request. Also avoids hammering OpenAI's per-component setup
    // path if the backend ever fans out work eagerly.
    for (const e of entries) {
      if (e.status === 'done') continue
      if (!e.data) continue
      setEntries((prev) =>
        prev.map((x) =>
          x.id === e.id ? { ...x, status: 'uploading', error: null } : x,
        ),
      )
      try {
        const res = await applicationApi.bootstrap(
          applicationId,
          e.data as Record<string, unknown>,
          e.locale,
          stage,
        )
        setEntries((prev) =>
          prev.map((x) =>
            x.id === e.id ? { ...x, status: 'done', result: res } : x,
          ),
        )
        success++
      } catch (err: unknown) {
        const ex = err as { response?: { data?: { error?: string } }; message?: string }
        const message = ex.response?.data?.error || ex.message || 'Import failed'
        setEntries((prev) =>
          prev.map((x) =>
            x.id === e.id ? { ...x, status: 'failed', error: message } : x,
          ),
        )
      }
    }
    setBusy(false)
    if (success > 0) {
      toast.success(
        success === entries.length
          ? `Imported ${success} locale${success > 1 ? 's' : ''}`
          : `Imported ${success}/${entries.length} locales (see errors below)`,
      )
      onImported?.()
    } else {
      toast.error('Nothing imported — see errors below')
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Bootstrap from JSON"
      size="large"
      footer={
        <>
          <Button variant="outline" onClick={handleClose} disabled={busy}>
            {entries.some((e) => e.status === 'done') ? 'Close' : 'Cancel'}
          </Button>
          <Button
            variant="primary"
            onClick={runImport}
            disabled={!canImport}
            isLoading={busy}
          >
            <Upload className="w-4 h-4 mr-2" />
            Import {entries.length > 0 ? `${entries.length} file${entries.length > 1 ? 's' : ''}` : ''}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-xs text-blue-900">
          <strong className="font-semibold">How this works:</strong> Each JSON
          file becomes one locale&apos;s draft (or chosen stage) translations.
          The top-level object keys become <em>component codes</em>; flat
          string keys at the root collapse into a synthetic{' '}
          <code className="font-mono">common</code> component. The filename
          stem (<code className="font-mono">en.json</code> →{' '}
          <code className="font-mono">en</code>) seeds the locale, but you can
          edit each row.
        </div>

        <div className="flex items-center gap-3">
          <div className="flex-1">
            <label className="block text-xs font-medium text-gray-500 mb-1">
              Stage (applied to all files)
            </label>
            <Select
              value={stage}
              onChange={(e) => setStage(e.target.value)}
              disabled={busy}
              options={[
                { value: 'draft', label: 'Draft' },
                { value: 'staging', label: 'Staging' },
                { value: 'production', label: 'Production' },
              ]}
            />
          </div>
        </div>

        <div>
          <label
            htmlFor="bootstrap-files"
            className={[
              'flex items-center justify-center gap-2 rounded-md border-2 border-dashed border-gray-300 px-4 py-6 text-sm text-gray-600',
              busy ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:bg-gray-50 hover:border-primary-400',
            ].join(' ')}
          >
            <Plus className="w-4 h-4" />
            Click to pick JSON files (you can select multiple)
            <input
              id="bootstrap-files"
              type="file"
              accept=".json,application/json"
              multiple
              className="hidden"
              disabled={busy}
              onChange={(e) => {
                ingestFiles(e.target.files)
                // Reset so picking the same file twice re-fires onChange.
                e.target.value = ''
              }}
            />
          </label>
        </div>

        {entries.length > 0 && (
          <div className="overflow-hidden border border-gray-200 rounded-md">
            <table className="min-w-full divide-y divide-gray-200 text-sm">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-3 py-2 text-left font-medium text-gray-700">File</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-700 w-32">Locale</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-700">Status</th>
                  <th className="px-3 py-2 w-10" aria-label="remove" />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 bg-white">
                {entries.map((e) => {
                  const dup =
                    e.locale &&
                    trimmedLocales.filter((l) => l === e.locale).length > 1
                  return (
                    <tr key={e.id}>
                      <td className="px-3 py-2 align-top">
                        <div className="font-mono text-xs text-gray-800">{e.file.name}</div>
                        <div className="text-xs text-gray-500">
                          {(e.file.size / 1024).toFixed(1)} KB
                        </div>
                      </td>
                      <td className="px-3 py-2 align-top">
                        <Input
                          value={e.locale}
                          onChange={(ev) => updateLocale(e.id, ev.target.value)}
                          disabled={busy || e.status === 'done'}
                          className={dup ? 'border-red-300' : ''}
                          placeholder="en"
                        />
                        {dup && (
                          <div className="text-xs text-red-600 mt-0.5">Duplicate</div>
                        )}
                      </td>
                      <td className="px-3 py-2 align-top">
                        {e.parseError && (
                          <div className="flex items-start gap-1.5 text-xs text-red-700">
                            <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" />
                            <span>{e.parseError}</span>
                          </div>
                        )}
                        {!e.parseError && e.status === 'pending' && (
                          <Badge variant="secondary" size="sm">Ready</Badge>
                        )}
                        {e.status === 'uploading' && (
                          <div className="flex items-center gap-1.5 text-xs text-blue-700">
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                            Uploading…
                          </div>
                        )}
                        {e.status === 'done' && e.result && (
                          <div className="flex flex-col gap-0.5 text-xs">
                            <div className="flex items-center gap-1.5 text-green-700">
                              <CheckCircle2 className="w-3.5 h-3.5" />
                              <span>
                                <strong>{e.result.components_created}</strong> created,{' '}
                                <strong>{e.result.components_updated}</strong> updated
                              </span>
                            </div>
                            <span className="text-gray-500 pl-5">
                              {e.result.keys_imported} keys total
                              {e.result.flat_keys_in_common > 0 && (
                                <> · {e.result.flat_keys_in_common} flat in <code className="font-mono">common</code></>
                              )}
                            </span>
                          </div>
                        )}
                        {e.status === 'failed' && (
                          <div className="flex items-start gap-1.5 text-xs text-red-700">
                            <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" />
                            <span>{e.error}</span>
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-2 align-top text-right">
                        {e.status !== 'uploading' && (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            disabled={busy}
                            onClick={() => removeEntry(e.id)}
                            aria-label="Remove file"
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Modal>
  )
}
