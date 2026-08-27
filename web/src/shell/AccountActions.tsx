import { Link } from 'react-router'

import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import { ActionIcon, Group, IconLogout, IconUserCircle, Tooltip } from '@/ui'

import { LocaleActions } from './LocaleActions'

/**
 * The identity controls in the top right of both signed-in headers: the way
 * into the account, and the way out of the session.
 *
 * One component rather than the same block in `DesktopShell` and
 * `MobileShell`, which is where it lived and where it had already been copied
 * once. The account is not a section -- see `./nav.ts` -- so it is not in the
 * navigation; it is who is looking, and it sits in the corner that says so.
 *
 * **Two icons rather than a name and a button.** The header used to spend its
 * width on the account's display name and a button reading "End guest
 * session", which on a 390px phone is most of the row. The cost is real and
 * worth naming: a sighted visitor can no longer read whose session this is at
 * a glance -- they hover the profile icon, or open the page it leads to, which
 * names the account at the top. What does not change is that the information
 * is still *there*: both controls carry the words they replaced as their
 * accessible name and as a tooltip, so a screen reader hears exactly what it
 * heard before.
 *
 * The guest branch survives for that reason. A guest's session is the only
 * copy of their work, and "End guest session" says what it ends rather than
 * borrowing the word for leaving an account you can come back to -- a
 * distinction an icon cannot draw, so the label carries it.
 */
export function AccountActions() {
  const { user, signOut } = useAuth()
  const t = useT()

  // "Account: Alice" rather than "Alice": a glyph named only with a person's
  // name says whose it is without saying what pressing it does, and this
  // control has no visible text left to answer that. A session with nothing to
  // show under a name falls back to the word alone, since a control with no
  // accessible name is a control nobody can find.
  const who = user?.display_name.trim()
  const accountLabel = who ? t('account.namedAction', { name: who }) : t('account.action')
  const signOutLabel = user?.anonymous ? t('auth.endGuestSession') : t('auth.signOut')

  return (
    // Pushed right together, so the header still ends in the way out whether
    // or not there is an account to link to.
    <Group gap="xs" ml="auto" wrap="nowrap">
      {/* Language first, then who is looking, then the way out. It is the one
          control here that is not about the account -- somebody signed out
          needs it too, which is why SignInActions draws it as well -- so it
          sits outside the pair rather than between them. */}
      <LocaleActions />
      {user === null ? null : (
        <Tooltip label={accountLabel} withArrow>
          <ActionIcon component={Link} to="/account" variant="subtle" aria-label={accountLabel}>
            <IconUserCircle size={20} />
          </ActionIcon>
        </Tooltip>
      )}
      <Tooltip label={signOutLabel} withArrow>
        <ActionIcon variant="subtle" aria-label={signOutLabel} onClick={() => void signOut()}>
          <IconLogout size={20} />
        </ActionIcon>
      </Tooltip>
    </Group>
  )
}
