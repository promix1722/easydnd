import { useState } from 'react'

import { captureInviteToken, InvitePrompt, JoinScreen } from '@/features/groups'
import { useAuth } from '@/lib/auth'

/**
 * `/groups/join` for everybody, the way HomeRoute is `/` for everybody.
 *
 * It exists instead of a `<Private>` wrapper for one reason: the invitation
 * has to be saved *before* the branch. `Private` renders the landing page to a
 * signed-out visitor, so the screen underneath never mounts -- and an
 * invitation link is precisely the deep link that arrives at a stranger who is
 * about to leave this page to sign in. Whatever ran inside the screen would
 * run only for people who did not need it.
 *
 * The capture is in a `useState` initialiser rather than an effect because it
 * has to happen before anything can navigate away, and because it is
 * idempotent: reading a fragment and writing it to sessionStorage is safe to
 * do twice, which is what makes it safe during render.
 */
export function JoinRoute() {
  const { status } = useAuth()
  const [token] = useState(captureInviteToken)

  return status === 'authenticated' ? (
    <JoinScreen token={token} />
  ) : (
    <InvitePrompt hasToken={token !== ''} />
  )
}
