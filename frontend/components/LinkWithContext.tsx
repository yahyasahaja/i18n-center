'use client'

import NextLink from 'next/link'
import { useAppContext } from '@/context/AppContext'

export interface LinkWithContextProps
  extends Omit<React.ComponentProps<typeof NextLink>, 'href'> {
  /** Path to navigate to (context params are added automatically) */
  href: string
  /** Extra query params to add (e.g. { edit: 'id' }) */
  extraParams?: Record<string, string>
}

/**
 * Next.js Link that preserves sidebar context (application_id, stage) in the URL.
 * Use for all in-app links so the sidebar state is never reset on navigation.
 */
export function LinkWithContext({
  href,
  extraParams,
  ...rest
}: LinkWithContextProps) {
  const { buildHref } = useAppContext()
  return <NextLink href={buildHref(href, extraParams)} {...rest} />
}
