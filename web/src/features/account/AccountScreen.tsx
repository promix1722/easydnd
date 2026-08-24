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
 *
 * The second rule is about what appears at all: **a section is drawn when it
 * has something to show or something to do**, and otherwise not at all. An
 * account reached only through Google has no passkeys and can never gain one,
 * so a "Passkeys" heading over a sentence explaining its own emptiness is a
 * heading over nothing. A card with a Connect button in it, on the other hand,
 * earns its place with no rows at all -- connecting is the only recovery this
 * design has, so the offer is the content.
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

      {/* Nothing here adds a passkey -- an account's passkeys are the ones it
          was created with -- so an empty card would be a heading with no rows
          under it and no button to change that. It carries no count either:
          the rows are immediately below it and each already wears a badge of
          its own, which a second one in the heading only competes with. */}
      {user.credentials.length > 0 ? (
        <Card withBorder padding="md">
          <Stack gap="sm">
            <Title order={4}>Passkeys</Title>

            {user.credentials.map((credential) => (
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
            ))}
          </Stack>
        </Card>
      ) : null}

      {/* Drawn whenever there is a provider connected or one left to connect,
          which is not the same test as "has identities": a passkey-only
          account has nothing to list and is exactly the account that most
          needs the offer. Only a deployment that configured no provider at all
          loses the card, because then there is neither.

          This heading does keep its count, and for the one state the rows
          cannot express: zero connected with a Connect button beside it says
          the second way in is still missing. */}
      {user.identities.length > 0 || unlinked.length > 0 ? (
        <Card withBorder padding="md">
          <Stack gap="sm">
            <Group justify="space-between">
              <Title order={4}>Connected accounts</Title>
              <Badge variant="light">{user.identities.length}</Badge>
            </Group>

            {user.identities.map((identity) => {
              const name =
                providers.find((p) => p.id === identity.provider)?.name ?? identity.provider
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
      ) : null}
    </Stack>
  )
}

/** Dates arrive as RFC 3339 and are only ever shown, never compared. */
function formatDate(value: string): string {
  const at = new Date(value)
  return Number.isNaN(at.getTime()) ? 'recently' : at.toLocaleDateString()
}
