import { useState } from 'react'

import { ReceiveScreen } from '@/features/characters'
import { captureInviteToken, COPY_LINK_STASH_KEY, InvitePrompt } from '@/features/groups'
import { useAuth } from '@/lib/auth'

/**
 * `/characters/receive` for everybody: JoinRoute for a copy link.
 *
 * The same shape for the same reason -- the token has to be saved before the
 * branch, because a signed-out visitor is about to leave this page to sign in.
 */
export function ReceiveRoute() {
  const { status } = useAuth()
  const [token] = useState(() => captureInviteToken(COPY_LINK_STASH_KEY))

  return status === 'authenticated' ? (
    <ReceiveScreen token={token} />
  ) : (
    <InvitePrompt character hasToken={token !== ''} />
  )
}
