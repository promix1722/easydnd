import { useAuth } from '@/lib/auth'
import { Alert, Text } from '@/ui'

/**
 * Tells a guest that nothing they build is being kept.
 *
 * A guest session has no account behind it and no way to sign back in, so the
 * one thing worth saying up front is that this is a session and not a place
 * where work accumulates. An account holder gets no banner: the state of their
 * ways in lives on the account screen, where the controls that change it are.
 */
export function PasskeyNotice() {
  const { user } = useAuth()
  if (!user?.anonymous) return null

  return (
    <Alert color="orange" title="You are playing as a guest">
      <Text size="sm">
        Nothing here is saved. These characters live in this session only -- there is no account
        behind it and no way to sign back in. To keep what you build, sign out and create an
        account with a passkey; you will be starting fresh.
      </Text>
    </Alert>
  )
}
