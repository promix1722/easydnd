import { useState } from 'react'

import { useAuth } from '@/lib/auth'
import { formatDate, useLocale, useT } from '@/lib/i18n'
import type { Translate } from '@/lib/i18n'
import { Alert, Badge, Button, Card, Group, Page, Stack, Text, Title } from '@/ui'

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
 *
 * It wears `ui/Page` like every other screen behind the sign-in, and that was
 * a correction rather than the original: this page drew its own `Title` over
 * its own dimmed line, so its heading was a different size from the one on the
 * character list, started at a different height, and was capped by nothing at
 * all on a wide monitor. `/account` belongs to no section -- see
 * `ui/sections.ts` -- so `Page` gives it a heading and no breadcrumb, which is
 * what it should have. It passes `namedByChrome` with it: the phone's chrome
 * *does* have a word for this place, the account being a row in the menu that
 * selector opens, so below `md` the heading would be that word twice on a 390px
 * screen. It is dropped there exactly as a section root's is, and a desktop is
 * unchanged -- same heading, same 1024px cap as every other page.
 */
export function AccountScreen() {
  const t = useT()
  const locale = useLocale()
  const { user, providers, linkProvider, unlinkProvider, busy, error } = useAuth()
  const [confirming, setConfirming] = useState<string | null>(null)

  if (!user) return null

  // A guest has no account: nothing to inventory, nothing to connect, and
  // nothing that would survive the session anyway. Offering "Connect Google"
  // here would be offering to link a provider to a record that does not exist,
  // and the server would refuse it -- so the page draws none of it.
  //
  // What it does not do any more is explain itself. The subtitle says what this
  // session is, and an alert under it spelling out what a guest session lacks
  // was a page scolding somebody for the way they chose to come in, on the one
  // screen where there is nothing they can do about it.
  if (user.anonymous) {
    return <Page trail={trail(t)} namedByChrome subtitle={t('account.asGuest')} />
  }

  const methods = user.credentials.length + user.identities.length
  const unlinked = providers.filter(
    (provider) => !user.identities.some((identity) => identity.provider === provider.id),
  )

  return (
    <Page trail={trail(t)} namedByChrome>
      <Stack gap="lg">
        {error ? (
          <Alert color="red" title={t('group.actionFailed')}>
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
              <Title order={4}>{t('account.passkeys')}</Title>

              {user.credentials.map((credential) => (
                <Group key={credential.id} justify="space-between" wrap="nowrap">
                  <Text size="sm" truncate>
                    {t('account.added', { when: added(locale, credential.created_at, t) })}
                  </Text>
                  {/* A passkey that does not sync is a single point of failure
                      for an account with no recovery path, and the only honest
                      thing to do is say so. */}
                  <Badge color={credential.backed_up ? 'blue' : 'orange'} variant="light">
                    {credential.backed_up ? t('account.synced') : t('account.deviceOnly')}
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
                <Title order={4}>{t('account.connected')}</Title>
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
                          {t('common.cancel')}
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
                          {t('account.disconnect')}
                        </Button>
                      </Group>
                    ) : (
                      <Button
                        variant="default"
                        disabled={busy || last}
                        // Disabled rather than hidden, with the reason attached:
                        // a control that vanishes reads as a bug, and the reason
                        // is the thing worth knowing.
                        title={last ? t('account.onlyWayIn') : undefined}
                        onClick={() => setConfirming(key)}
                      >
                        {t('account.disconnect')}
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
                      {t('account.connect', { provider: provider.name })}
                    </Button>
                  ))}
                </Group>
              ) : null}
            </Stack>
          </Card>
        ) : null}
      </Stack>
    </Page>
  )
}

/**
 * The whole trail: one crumb, with no section above it.
 *
 * Hoisted out of the component because both returns draw it and a second
 * literal is a second thing that can come to read differently.
 */
function trail(t: Translate) {
  return [{ label: t('account.title') }]
}

/**
 * Dates arrive as RFC 3339 and are only ever shown, never compared.
 *
 * In the app's locale rather than the browser's: a visitor who switched
 * easydnd to Russian should not read Russian captions above English dates.
 */
function added(locale: string, value: string, t: Translate): string {
  const at = new Date(value)
  return Number.isNaN(at.getTime()) ? t('account.recently') : formatDate(value, locale)
}
