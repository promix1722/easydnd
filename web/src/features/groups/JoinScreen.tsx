import { useNavigate } from 'react-router'

import { acceptInvite, previewInvite } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Alert, Button, Group, Loader, Stack, Text, Title } from '@/ui'

import { clearInviteToken } from './inviteToken'
import { ROLE_LABELS } from './roles'

export interface JoinScreenProps {
  /**
   * The invitation, already recovered from the fragment or from where it was
   * saved before signing in. It arrives as a prop because the route has to
   * capture it before deciding whether this screen is rendered at all -- see
   * routes/JoinRoute.tsx.
   */
  token: string
}

/**
 * The screen a signed-in visitor sees on an invitation link: whose group,
 * which rank, and one button.
 */
export function JoinScreen({ token }: JoinScreenProps) {
  const navigate = useNavigate()
  const accept = useAction(acceptInvite)

  const { data, error, loading } = useResource(`invite:${token}`, (signal) => {
    if (token === '') return Promise.reject(new Error('no invitation'))
    return previewInvite(token, signal)
  })

  async function join() {
    const joined = await accept.run(token)
    if (joined === null) return
    clearInviteToken()
    // Replacing the entry is also what scrubs the token out of the address
    // bar: there is nothing left to leak once this resolves.
    await navigate(`/groups/${joined.id}`, { replace: true })
  }

  async function decline() {
    // Forgotten on the way out, so that a saved invitation cannot resurface
    // days later on a route that had nothing to do with it.
    clearInviteToken()
    await navigate('/groups')
  }

  if (token === '') {
    return (
      <Alert color="red" title="No invitation">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">
            That link is missing its invitation. Ask whoever sent it for a fresh one.
          </Text>
          <Button variant="light" onClick={() => void navigate('/groups')}>
            Your groups
          </Button>
        </Stack>
      </Alert>
    )
  }

  if (loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Checking the invitation...
        </Text>
      </Group>
    )
  }

  if (error !== null || data === null) {
    return (
      <Alert color="red" title="That invitation is not usable">
        <Stack gap="xs" align="flex-start">
          {/* Whatever the server said. An expired link is a 400 and never a
              401, so being here does not mean anybody has been signed out. */}
          <Text size="sm">{error ?? 'It may have expired. Ask for a fresh link.'}</Text>
          <Button variant="light" onClick={() => void decline()}>
            Your groups
          </Button>
        </Stack>
      </Alert>
    )
  }

  return (
    <Stack gap="md">
      <div>
        <Title order={2}>{data.group_name}</Title>
        <Text c="dimmed" size="sm">
          {data.invited_by !== undefined && data.invited_by !== ''
            ? `${data.invited_by} invited you to join as a ${ROLE_LABELS[data.role].toLowerCase()}.`
            : `You have been invited to join as a ${ROLE_LABELS[data.role].toLowerCase()}.`}
        </Text>
      </div>

      {data.already_member ? (
        <Stack gap="xs" align="flex-start">
          <Text size="sm">You are already in this group.</Text>
          <Button onClick={() => void navigate(`/groups/${data.group_id}`, { replace: true })}>
            Open it
          </Button>
        </Stack>
      ) : (
        <>
          {accept.error !== null && (
            <Alert color="red" title="Could not join">
              {accept.error}
            </Alert>
          )}
          <Group gap="xs">
            <Button loading={accept.pending} onClick={() => void join()}>
              Join group
            </Button>
            <Button variant="default" onClick={() => void decline()}>
              Not now
            </Button>
          </Group>
        </>
      )}
    </Stack>
  )
}
