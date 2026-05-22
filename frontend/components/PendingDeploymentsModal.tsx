'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card } from '@/components/ui/Card'
import { Rocket, ArrowRight, Inbox, Loader2 } from 'lucide-react'
import toast from 'react-hot-toast'
import { applicationApi, componentApi } from '@/services/api'

// Narrow shape — the modal only displays id + code; not worth importing the
// full Component type from the store.
type ComponentRow = { id: string; code: string }

// What "pending" means depends on which environment the sidebar is set to:
//   draft       → rows where stage_completed == 'draft' (ready to promote to staging)
//   staging     → rows where stage_completed == 'staging' (ready to promote to production)
//   production  → nothing — already at the terminal stage
//
// Keeping the rule data-driven so the modal copy and the filter both read
// from one source.
const PROMOTION_TARGET: Record<string, string | null> = {
  draft: 'staging',
  staging: 'production',
  production: null,
}
import { useAppContext } from '@/context/AppContext'

interface PendingDeploymentsModalProps {
  isOpen: boolean
  onClose: () => void
  applicationId: string
  // Fired after a successful deploy so the parent can refetch its pending
  // list, cache, etc.
  onDeployed?: () => void
}

type PendingDeploy = {
  locale: string
  stage_completed: string
  next_stage: string
}

// PendingDeploymentsModal — groups by locale and surfaces a one-click "promote
// every component in this locale to next stage" action.
//
// Why grouped by locale (and not by component or flat):
//   * Deploys are atomic per (application, locale). The
//     POST /applications/:id/deploy-locale endpoint takes one locale and
//     promotes every component in lockstep — that's the contract, and the
//     existing handler wraps it in a sqlx tx for atomicity. So the natural
//     unit of work for users IS the locale.
//   * Components inside a locale share the deploy decision: when you ship
//     `id` from draft to staging, every component goes together. Grouping
//     by locale therefore matches the mental model people already have.
//   * We list the components under each locale as a visibility aid — "what
//     am I about to deploy?" — and provide per-component navigation so
//     authors can spot-check a draft before clicking Deploy.
export function PendingDeploymentsModal({
  isOpen,
  onClose,
  applicationId,
  onDeployed,
}: PendingDeploymentsModalProps) {
  const router = useRouter()
  const { buildHref, stage: sidebarStage } = useAppContext()
  // What stage do we promote *from*? When the sidebar Environment is `draft`,
  // the user expects the modal to show "what's still in draft that I could
  // ship to staging." When the sidebar is `production`, there's nothing to
  // promote past production, so the modal renders an empty-state.
  const promotionTargetStage = PROMOTION_TARGET[sidebarStage] // 'staging' | 'production' | null
  const [pending, setPending] = useState<PendingDeploy[]>([])
  const [components, setComponents] = useState<ComponentRow[]>([])
  const [loading, setLoading] = useState(false)
  const [deployingLocale, setDeployingLocale] = useState<string | null>(null)

  // Load both the locale-level pending list and the full component list when
  // the modal opens. Component data drives the per-locale row table —
  // skipping it means we'd just say "deploy locale X" without showing what's
  // inside, which loses the visibility benefit.
  useEffect(() => {
    if (!isOpen || !applicationId) return
    let cancelled = false
    const load = async () => {
      setLoading(true)
      try {
        const [pendingRes, componentsRes] = await Promise.all([
          applicationApi.getPendingDeploys(applicationId),
          componentApi.getAll({ applicationId, pageSize: 1000 }),
        ])
        if (cancelled) return
        setPending(pendingRes.pending_deploys || [])
        setComponents(componentsRes.data || [])
      } catch {
        // Errors here are non-fatal for the page — they show as empty state.
        if (!cancelled) {
          setPending([])
          setComponents([])
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [isOpen, applicationId])

  const deployLocale = async (locale: string, nextStage: string) => {
    const ok = window.confirm(
      `Deploy every component in "${locale.toUpperCase()}" from its current stage to "${nextStage}"?\n\n` +
        `This is atomic: either every component promotes or none of them do.`,
    )
    if (!ok) return
    try {
      setDeployingLocale(locale)
      await applicationApi.deployLocale(applicationId, locale)
      toast.success(`Deployed ${locale.toUpperCase()} → ${nextStage}`)
      // Refresh both the pending list (some rows may now be resolved) and
      // the parent's view.
      const refreshed = await applicationApi.getPendingDeploys(applicationId)
      setPending(refreshed.pending_deploys || [])
      onDeployed?.()
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string; detail?: string } } }
      toast.error(e.response?.data?.error || e.response?.data?.detail || 'Deploy failed')
    } finally {
      setDeployingLocale(null)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Pending deployments"
      size="large"
      footer={
        <Button variant="outline" onClick={onClose}>
          Close
        </Button>
      }
    >
      {(() => {
        // Filter by sidebar stage: only locales whose stage_completed matches
        // the current sidebar Environment are eligible for promotion. The
        // server returns every non-production row; we narrow client-side.
        const scoped = promotionTargetStage
          ? pending.filter((p) => p.stage_completed === sidebarStage)
          : []

        if (loading) {
          return (
            <div className="flex items-center justify-center py-12 text-gray-500">
              <Loader2 className="w-5 h-5 animate-spin mr-2" />
              Loading pending deployments…
            </div>
          )
        }

        if (!promotionTargetStage) {
          // Sidebar is on Production. Nothing further to promote.
          return (
            <div className="rounded-md border border-dashed border-gray-300 p-8 text-center">
              <Inbox className="mx-auto h-10 w-10 text-gray-400" />
              <p className="mt-2 text-sm font-medium text-gray-700">
                Production is the terminal stage
              </p>
              <p className="mt-1 text-xs text-gray-500">
                Switch the Environment selector in the sidebar to <strong>Draft</strong> or{' '}
                <strong>Staging</strong> to see what could be promoted next.
              </p>
            </div>
          )
        }

        if (scoped.length === 0) {
          return (
            <div className="rounded-md border border-dashed border-gray-300 p-8 text-center">
              <Inbox className="mx-auto h-10 w-10 text-gray-400" />
              <p className="mt-2 text-sm font-medium text-gray-700">
                No locales sitting at <strong>{sidebarStage}</strong>
              </p>
              <p className="mt-1 text-xs text-gray-500">
                Nothing to promote to {promotionTargetStage}. Saves on the{' '}
                {sidebarStage} stage will show up here once they&apos;re ready.
              </p>
            </div>
          )
        }

        return (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Each row is a locale sitting at <strong>{sidebarStage}</strong>.
            Deploy promotes <em>every component</em> for that locale to{' '}
            <strong>{promotionTargetStage}</strong> in one atomic transaction —
            perfect for "ship the {promotionTargetStage} cut for{' '}
            {scoped[0]?.locale.toUpperCase()}" releases. Switch the sidebar
            Environment to see promotions for a different stage.
          </p>

          {scoped.map((p) => (
            <Card key={p.locale}>
              <div className="flex items-start justify-between gap-3 mb-3">
                <div>
                  <div className="flex items-center gap-2">
                    <Badge variant="info">{p.locale.toUpperCase()}</Badge>
                    <span className="text-sm text-gray-700">
                      currently at <strong>{p.stage_completed}</strong>
                    </span>
                    <ArrowRight className="w-4 h-4 text-gray-400" />
                    <Badge variant="warning">{p.next_stage}</Badge>
                  </div>
                  <p className="text-xs text-gray-500 mt-1">
                    {components.length} component{components.length === 1 ? '' : 's'} will move
                    together.
                  </p>
                </div>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => deployLocale(p.locale, p.next_stage)}
                  isLoading={deployingLocale === p.locale}
                  disabled={!!deployingLocale}
                >
                  <Rocket className="w-4 h-4 mr-2" />
                  Deploy all to {p.next_stage}
                </Button>
              </div>

              {components.length > 0 && (
                <div className="border-t border-gray-100 pt-3">
                  <p className="text-xs font-medium text-gray-500 mb-2">
                    Components in this deploy:
                  </p>
                  <div className="flex flex-wrap gap-1.5">
                    {components.slice(0, 12).map((c) => (
                      <button
                        key={c.id}
                        type="button"
                        className="inline-flex items-center gap-1 rounded bg-gray-100 hover:bg-primary-50 hover:text-primary-700 text-gray-700 text-xs px-2 py-1 font-mono transition-colors"
                        onClick={() => {
                          // buildHref carries sidebar app/stage context; we
                          // append locale + stage so the editor opens scoped
                          // to the exact slice the user is about to ship.
                          router.push(
                            buildHref(`/components/${c.id}/translations`, {
                              locale: p.locale,
                              stage: p.stage_completed,
                            }),
                          )
                          onClose()
                        }}
                        title={`Open ${c.code} at ${p.locale}/${p.stage_completed}`}
                      >
                        {c.code}
                        <ArrowRight className="w-3 h-3" />
                      </button>
                    ))}
                    {components.length > 12 && (
                      <span className="text-xs text-gray-500 px-2 py-1">
                        + {components.length - 12} more
                      </span>
                    )}
                  </div>
                </div>
              )}
            </Card>
          ))}
        </div>
        )
      })()}
    </Modal>
  )
}
