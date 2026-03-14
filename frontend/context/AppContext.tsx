'use client'

/**
 * AppContext provides application_id and stage (synced with URL) and context-aware navigation.
 *
 * For any in-app navigation (links, buttons) use:
 * - push(path) or push(path, { extraParams: { ... } }) to preserve sidebar state
 * - <LinkWithContext href="..."> for Next.js Link
 * - buildHref(path) when you need the URL string (e.g. for redirects that should keep context)
 *
 * Use the raw useRouter() only for auth redirects (e.g. to /login or /dashboard) where
 * context params are not needed.
 */
import React, { createContext, useCallback, useContext, useMemo } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'

const STAGES = ['draft', 'staging', 'production'] as const
export type Stage = (typeof STAGES)[number]

export function isValidStage(s: string): s is Stage {
  return STAGES.includes(s as Stage)
}

/** Options for context-aware push/replace (preserves application_id & stage in URL) */
export interface NavigateOptions {
  scroll?: boolean
  /** Extra query params to add/override (e.g. { edit: 'id' }) */
  extraParams?: Record<string, string>
}

export interface AppContextValue {
  applicationId: string | null
  stage: Stage
  setApplicationId: (id: string | null) => void
  setStage: (stage: Stage) => void
  updateParams: (updates: { application_id?: string | null; stage?: string; locale?: string }) => void
  /** Build href with current application_id and stage (for links and programmatic nav) */
  buildHref: (path: string, extraParams?: Record<string, string>) => string
  /** Navigate and preserve sidebar context. Use for all in-app links/buttons. */
  push: (path: string, options?: NavigateOptions) => void
  /** Same as push but replace history entry */
  replace: (path: string, options?: NavigateOptions) => void
}

const AppContext = createContext<AppContextValue | null>(null)

export function AppContextProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()

  const applicationId = searchParams.get('application_id') || null
  const stageParam = searchParams.get('stage') || 'draft'
  const stage: Stage = isValidStage(stageParam) ? stageParam : 'draft'

  const updateParams = useCallback(
    (updates: { application_id?: string | null; stage?: string; locale?: string }) => {
      const params = new URLSearchParams(searchParams.toString())
      if ('application_id' in updates) {
        if (updates.application_id) params.set('application_id', updates.application_id)
        else params.delete('application_id')
      }
      if (updates.stage !== undefined) params.set('stage', updates.stage)
      if (updates.locale !== undefined) {
        if (updates.locale) params.set('locale', updates.locale)
        else params.delete('locale')
      }
      const q = params.toString()
      router.replace(q ? `${pathname}?${q}` : pathname, { scroll: false })
    },
    [pathname, router, searchParams]
  )

  const setApplicationId = useCallback(
    (id: string | null) => updateParams({ application_id: id }),
    [updateParams]
  )

  const setStage = useCallback(
    (s: Stage) => updateParams({ stage: s }),
    [updateParams]
  )

  const buildHref = useCallback(
    (path: string, extraParams?: Record<string, string>): string => {
      const [base, existingQuery] = path.split('?')
      const params = new URLSearchParams(existingQuery ?? '')
      if (applicationId) params.set('application_id', applicationId)
      params.set('stage', stage)
      if (extraParams) {
        Object.entries(extraParams).forEach(([k, v]) => params.set(k, v))
      }
      const q = params.toString()
      return q ? `${base}?${q}` : base
    },
    [applicationId, stage]
  )

  const push = useCallback(
    (path: string, options?: NavigateOptions) => {
      const url = buildHref(path, options?.extraParams)
      router.push(url, { scroll: options?.scroll ?? true })
    },
    [router, buildHref]
  )

  const replace = useCallback(
    (path: string, options?: NavigateOptions) => {
      const url = buildHref(path, options?.extraParams)
      router.replace(url, { scroll: options?.scroll ?? true })
    },
    [router, buildHref]
  )

  const value = useMemo<AppContextValue>(
    () => ({
      applicationId,
      stage,
      setApplicationId,
      setStage,
      updateParams,
      buildHref,
      push,
      replace,
    }),
    [applicationId, stage, setApplicationId, setStage, updateParams, buildHref, push, replace]
  )

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}

export function useAppContext(): AppContextValue {
  const ctx = useContext(AppContext)
  if (!ctx) throw new Error('useAppContext must be used within AppContextProvider')
  return ctx
}
