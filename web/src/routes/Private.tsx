import type { ReactElement } from 'react'

import { useAuth } from '@/lib/auth'

import { LandingPage } from './LandingPage'

/**
 * Renders a screen only for a signed-in visitor, and the landing page
 * otherwise.
 *
 * It branches rather than redirecting, which is the rule the route table
 * already follows: the URL never changes on account of who is looking, so a
 * deep link to somebody's character survives being shared with someone who
 * has not signed in yet -- they see the mark, sign in from the header, and the
 * same route renders its real content.
 *
 * RootGate already keeps the signed-out visitor inside LandingShell, so this
 * is about the content rather than the chrome. Without it a signed-out deep
 * link would mount a screen that immediately fires an authenticated fetch and
 * renders somebody an error where an invitation belongs.
 */
export function Private({ children }: { children: ReactElement }): ReactElement {
  const { status } = useAuth()
  return status === 'authenticated' ? children : <LandingPage />
}
