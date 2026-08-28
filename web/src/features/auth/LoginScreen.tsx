import { useState } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'

import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import { isPasskeySupported } from '@/lib/webauthn'
import { Alert, Button, Card, Group, Stack, Text, Title } from '@/ui'

/**
 * The way in, on a page of its own.
 *
 * They are not variations on one flow: a passkey asks the browser a question, a
 * Google sign-in leaves for Google, and a guest session asks nobody anything. A
 * header popover could hold two of those; laying them out where they can be
 * read and compared is what the page buys, and it is also the only place with
 * room to say what each one costs.
 *
 * There is no "create an account" card, and that is the point. Signing in with
 * a passkey and signing up with one are the same press, because the browser
 * will not tell a page whether a passkey exists for it -- so asking the visitor
 * to choose would be asking a question the platform refuses to answer, and a
 * wrong answer strands whichever of the two they are. The button tries to sign
 * in and makes an account when there was nothing to sign in with. The only text
 * this app ever asked for went with that choice; the server names the account.
 *
 * Google comes first when it is offered. It is the only option that both keeps
 * your characters and works on a browser with no WebAuthn, which makes it the
 * one to reach for when you do not already know what a passkey is.
 *
 * Guest sits last because it is the only one that keeps nothing. It stays on
 * the page without WebAuthn, so a browser that lacks passkeys still leads
 * somewhere.
 *
 * A browser with no WebAuthn is told nothing about it. The passkey card simply
 * is not there, and what is left -- a provider, a guest -- is what that browser
 * can actually do: an alert explaining an option nobody was offered is a page
 * apologising for itself before it has been asked anything.
 */
export function LoginScreen() {
  const t = useT()
  const { status, signInOrRegister, signInAsGuest, signInWith, providers, busy, error } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const passkeys = isPasskeySupported()
  // Which button was pressed. `busy` is one flag for every flow -- there is
  // only ever one attempt in the air -- so a spinner bound to it directly spun
  // on all three at once, and the guest button appeared to be signing in while
  // the passkey picker was open. The flag says an attempt is running; this says
  // whose, and the two together are what a button needs to know.
  const [pressed, setPressed] = useState<'passkey' | 'guest' | null>(null)

  // Whoever sent us here recorded where they were, so signing in returns them
  // to the deep link they arrived on rather than dropping them at the root.
  //
  // The whole location, not just its path. A query string or a fragment is
  // part of where somebody was, and an invitation link is entirely fragment --
  // returning them to a bare `/groups/join` would land them on the one screen
  // that cannot work without it.
  const from = returnTo(location.state)

  // Already signed in: this page has nothing to offer, and leaving it reachable
  // would mean a "Log in" screen rendered inside the signed-in shell.
  if (status === 'authenticated') return <Navigate to="/" replace />

  const attempt = (which: 'passkey' | 'guest', run: () => Promise<boolean>) => {
    setPressed(which)
    void run().then((ok) => {
      if (ok) void navigate(from, { replace: true })
    })
  }

  return (
    <Stack gap="lg" maw={560} mx="auto" py="xl">
      <Title order={2}>{t('login.title')}</Title>

      {/* One alert for every flow. Whichever attempt failed most recently is
          the one worth showing, and the provider only keeps that one. */}
      {error ? (
        <Alert color="red" title={t('login.failed')}>
          {error}
        </Alert>
      ) : null}

      {/* First, and one card per provider. It signs in and signs up in a
          single press: the provider has already established who this is, so
          asking which of the two was meant would be a question with no
          content. Nothing renders when the deployment configured none -- a
          button for a provider that is not there would be a dead end. */}
      {providers.map((provider) => (
        <Card key={provider.id} withBorder padding="md">
          <Stack gap="sm">
            <div>
              <Title order={4}>{t('login.provider.title', { provider: provider.name })}</Title>
              <Text c="dimmed" size="sm">
                {t('login.provider.detail', { provider: provider.name })}
              </Text>
            </div>
            <Group>
              {/* No `loading`: this leaves the page rather than resolving, so
                  a spinner here would spin until the browser navigated away
                  and then come back on a fresh mount. */}
              <Button disabled={busy} onClick={() => signInWith(provider.id)}>
                {t('login.provider.title', { provider: provider.name })}
              </Button>
            </Group>
          </Stack>
        </Card>
      ))}

      {/* One button for both halves of an account's life. The copy carries the
          whole of the mitigation for the one rough edge: a cancelled picker is
          indistinguishable from an empty one, so cancelling is followed by an
          offer to make a passkey. Saying so before the press turns a surprise
          into something expected, and costs a first-time visitor nothing -- a
          confirmation dialog would charge every one of them a click to save a
          returning visitor a single Escape. */}
      {passkeys ? (
        <Card withBorder padding="md">
          <Stack gap="sm">
            <div>
              <Title order={4}>{t('login.passkey.title')}</Title>
              <Text c="dimmed" size="sm">
                {t('login.passkey.detail')}
              </Text>
            </div>
            <Group>
              <Button
                loading={busy && pressed === 'passkey'}
                disabled={busy && pressed !== 'passkey'}
                onClick={() => attempt('passkey', signInOrRegister)}
              >
                {t('login.passkey.action')}
              </Button>
            </Group>
          </Stack>
        </Card>
      ) : null}

      <Card withBorder padding="md">
        <Stack gap="sm">
          <div>
            <Title order={4}>{t('login.guest.title')}</Title>
            <Text c="dimmed" size="sm">
              {t('login.guest.detail')}
            </Text>
          </div>

          <Group>
            <Button
              variant="default"
              loading={busy && pressed === 'guest'}
              disabled={busy && pressed !== 'guest'}
              onClick={() => attempt('guest', signInAsGuest)}
            >
              {t('login.guest.action')}
            </Button>
          </Group>
        </Stack>
      </Card>
    </Stack>
  )
}

/**
 * Where to send somebody after they sign in.
 *
 * Rebuilt from the parts rather than taken whole, so that a `state` shaped by
 * something other than a react-router location -- a hand-written link, an
 * older build's history entry -- cannot navigate anywhere unexpected. Anything
 * unrecognisable falls back to the root.
 */
function returnTo(state: unknown): string {
  const from = (state as { from?: { pathname?: string; search?: string; hash?: string } } | null)
    ?.from
  if (from?.pathname === undefined || !from.pathname.startsWith('/')) return '/'
  return `${from.pathname}${from.search ?? ''}${from.hash ?? ''}`
}
