import { useState } from 'react'

import { useAuth } from '@/lib/auth'
import { Alert, Badge, Button, Card, Group, Stack, Text, Title } from '@/ui'

/**
 * Everything about how this account is reached.
 *
 * It exists because linking needs somewhere to live: connecting and
 * disconnecting a provider is an inventory of the ways in, and an inventory
 * needs a page.
 *
 * The rule the whole screen is arranged around: an account must always keep at
 * least one way in. The server enforces it -- this only makes it visible
 * before the click rather than after, by refusing to offer the last one up for
 * removal.
 */
export function AccountScreen() {
  const { user, providers, linkProvider, unlinkProvider, busy, error } = useAuth()
  const [confirming, setConfirming] = useState<string | null>(null)

  if (!user) return null

  // A guest has no account: nothing to inventory, nothing to connect, and
  // nothing that would survive the session anyway. Offering "Connect Google"
  // here would be offering to link a provider to a record that does not
  // exist -- the server would refuse it, and the honest answer is to say so
  // before the click rather than after.
  if (user.anonymous) {
    return (
      <Stack gap="lg">
        <div>
          <Title order={2}>Account</Title>
          <Text c="dimmed" size="sm">
            You are playing as a guest.
          </Text>
        </div>

        <Alert color="orange" title="There is no account to manage">
          A guest session has nothing behind it -- no passkey, no connected accounts, and no way to
          sign back in once it ends. To keep what you build, sign out and start again with a
          passkey or a connected account; you will be starting fresh.
        </Alert>
      </Stack>
    )
  }

  const methods = user.credentials.length + user.identities.length
  const unlinked = providers.filter(
    (provider) => !user.identities.some((identity) => identity.provider === provider.id),
  )

  return (
    <Stack gap="lg">
      <div>
        <Title order={2}>Account</Title>
        <Text c="dimmed" size="sm">
          Signed in as {user.display_name}.
        </Text>
      </div>

      {error ? (
        <Alert color="red" title="That did not work">
          {error}
        </Alert>
      ) : null}

      <Card withBorder padding="md">
        <Stack gap="sm">
          <Group justify="space-between">
            <Title order={4}>Passkeys</Title>
            <Badge variant="light">{user.credentials.length}</Badge>
          </Group>

          {user.credentials.length === 0 ? (
            <Text c="dimmed" size="sm">
              No passkey on this account. This account is reached through its connected accounts.
            </Text>
          ) : (
            user.credentials.map((credential) => (
              <Group key={credential.id} justify="space-between" wrap="nowrap">
                <Text size="sm" truncate>
                  Added {formatDate(credential.created_at)}
                </Text>
                {/* A passkey that does not sync is a single point of failure
                    for an account with no recovery path, and the only honest
                    thing to do is say so. */}
                <Badge color={credential.backed_up ? 'blue' : 'orange'} variant="light">
                  {credential.backed_up ? 'Synced' : 'This device only'}
                </Badge>
              </Group>
            ))
          )}
        </Stack>
      </Card>

      <Card withBorder padding="md">
        <Stack gap="sm">
          <Group justify="space-between">
            <Title order={4}>Connected accounts</Title>
            <Badge variant="light">{user.identities.length}</Badge>
          </Group>

          {user.identities.length === 0 && unlinked.length === 0 ? (
            <Text c="dimmed" size="sm">
              This deployment offers no external sign-in.
            </Text>
          ) : null}

          {user.identities.map((identity) => {
            const name = providers.find((p) => p.id === identity.provider)?.name ?? identity.provider
            const last = methods === 1
            const key = `${identity.provider}:${identity.subject}`

            return (
              <Group key={key} justify="space-between" wrap="nowrap">
                <div>
                  <Text size="sm">{name}</Text>
                  {identity.email ? (
                    <Text c="dimmed" size="xs">
                      {identity.email}
                    </Text>
                  ) : null}
                </div>

                {confirming === key ? (
                  <Group gap="xs" wrap="nowrap">
                    <Button variant="subtle" onClick={() => setConfirming(null)}>
                      Cancel
                    </Button>
                    <Button
                      color="red"
                      loading={busy}
                      onClick={() => {
                        void unlinkProvider(identity.provider, identity.subject).then((ok) => {
                          if (ok) setConfirming(null)
                        })
                      }}
                    >
                      Disconnect
                    </Button>
                  </Group>
                ) : (
                  <Button
                    variant="default"
                    disabled={busy || last}
                    // Disabled rather than hidden, with the reason attached:
                    // a control that vanishes reads as a bug, and the reason
                    // is the thing worth knowing.
                    title={last ? 'This is the only way left to sign in' : undefined}
                    onClick={() => setConfirming(key)}
                  >
                    Disconnect
                  </Button>
                )}
              </Group>
            )
          })}

          {unlinked.length > 0 ? (
            <Group>
              {unlinked.map((provider) => (
                <Button
                  key={provider.id}
                  variant="default"
                  disabled={busy}
                  onClick={() => linkProvider(provider.id)}
                >
                  Connect {provider.name}
                </Button>
              ))}
            </Group>
          ) : null}
        </Stack>
      </Card>
    </Stack>
  )
}

/** Dates arrive as RFC 3339 and are only ever shown, never compared. */
function formatDate(value: string): string {
  const at = new Date(value)
  return Number.isNaN(at.getTime()) ? 'recently' : at.toLocaleDateString()
}
