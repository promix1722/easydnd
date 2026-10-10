import { useState } from 'react'
import { Navigate, useLocation } from 'react-router'

import { ReceiveScreen } from '@/features/characters'
import {
  captureInviteToken, clearInviteToken, COPY_LINK_STASH_KEY, InvitePrompt, JoinScreen, kindOfLink, readInviteToken, tokenOfLink,
} from '@/features/groups'
import type { InvitationKind } from '@/features/groups'
import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import { Button, Group, Stack, Text, TextInput, Title } from '@/ui'

interface Invitation { kind: InvitationKind; token: string }

/**
 * The invitation this visit arrived with, saved before anything can navigate
 * away, or the one saved by an earlier visit that left to sign in.
 *
 * Each kind keeps its own key, as it always has: the screen that accepts one
 * forgets its own, and a pending group invitation and a pending copy link
 * must not overwrite each other.
 */
const keyOf = (kind: InvitationKind) => kind === 'character' ? COPY_LINK_STASH_KEY : undefined

function arrive(hint: InvitationKind | undefined): Invitation | null {
  if (window.location.hash.length > 1) {
    // The address it was sent as said which kind; failing that the token does.
    const kind = hint ?? kindOfLink(window.location.hash)
    return { kind, token: captureInviteToken(keyOf(kind)) }
  }
  for (const kind of hint ? [hint] : ['group', 'character'] as const) {
    const token = readInviteToken(keyOf(kind))
    if (token !== '') return { kind, token }
  }
  return null
}

/**
 * Where an invitation link lands: the two addresses links are sent as,
 * `/groups/join#<token>` and `/characters/receive#<token>`. It saves the token
 * and moves on to `/invitations`, so that every invitation is accepted on one
 * page with one name in the menu. The fragment rides along and so does the
 * kind the address named -- in router state, which a sign-in round trip loses
 * and the saved token's own key then answers for.
 */
export function InvitationLink({ kind }: { kind: InvitationKind }) {
  useState(() => captureInviteToken(keyOf(kind)))
  return <Navigate to={{ pathname: '/invitations', hash: window.location.hash }} state={{ kind }} replace />
}

/**
 * The one page every invitation is accepted on: `/invitations`.
 *
 * It is for everybody, the way HomeRoute is `/` for everybody, and exists
 * instead of a `<Private>` wrapper for one reason: the invitation has to be
 * saved *before* the branch. `Private` renders the landing page to a
 * signed-out visitor, so the screen underneath never mounts -- and an
 * invitation link is precisely the deep link that arrives at a stranger who is
 * about to leave this page to sign in. The capture is in a `useState`
 * initialiser rather than an effect because it has to happen before anything
 * can navigate away, and it is idempotent, which is what makes it safe during
 * render.
 *
 * Reached with no invitation -- from the menu -- it asks for one: a field to
 * paste the link into. An installed app has no address bar and is not what a
 * link in a messenger opens, so without this the only way to follow an
 * invitation from the app was not to use the app.
 */
export function InvitationRoute() {
  const t = useT()
  const { status } = useAuth()
  const hint = (useLocation().state as { kind?: InvitationKind } | null)?.kind
  const [arrived, setArrived] = useState(() => arrive(hint))
  const [link, setLink] = useState('')
  const [pasted, setPasted] = useState<Invitation | null>(null)
  const invitation = arrived ?? pasted

  if (status !== 'authenticated') {
    return <InvitePrompt character={invitation?.kind === 'character'} hasToken={invitation !== null} />
  }
  if (invitation !== null) {
    return (
      <Stack gap="md" align="flex-start">
        {invitation.kind === 'character' ? <ReceiveScreen token={invitation.token} /> : <JoinScreen token={invitation.token} />}
        {/* Not the one meant, or one already dealt with: forgotten, and back to the field. Without this an
            invitation saved by an earlier visit would sit on the menu's page for the rest of the session. */}
        <Button variant="subtle" onClick={() => { clearInviteToken(keyOf(invitation.kind)); setArrived(null); setPasted(null) }}>
          {t('invitations.again')}
        </Button>
      </Stack>
    )
  }
  return (
    <form onSubmit={(event) => { event.preventDefault(); setPasted({ kind: kindOfLink(link), token: tokenOfLink(link) }) }}>
      <Stack gap="md">
        <div>
          <Title order={2}>{t('invitations.title')}</Title>
          <Text c="dimmed" size="sm">{t('invitations.hint')}</Text>
        </div>
        <TextInput label={t('invitations.link')} value={link} autoComplete="off" autoCapitalize="none" spellCheck={false}
          onChange={(event) => setLink(event.currentTarget.value)} />
        <Group gap="xs">
          <Button type="submit" disabled={tokenOfLink(link) === ''}>{t('invitations.open')}</Button>
        </Group>
      </Stack>
    </form>
  )
}
