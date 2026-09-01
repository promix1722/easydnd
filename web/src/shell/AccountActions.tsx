import { useNavigate } from 'react-router'

import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import { ActionIcon, Group, IconLogout, InstallButton, Tooltip } from '@/ui'

import { LocaleActions } from './LocaleActions'

/**
 * What is left in the top right of both signed-in headers: the language, and
 * the way out of the session.
 *
 * One component rather than the same block in `DesktopShell` and
 * `MobileShell`, which is where it lived and where it had already been copied
 * once.
 *
 * **The way in to the account is no longer here.** It was a third glyph in a
 * corner that already held two, saying nothing about itself; it is a row in the
 * navigation now -- the phone's dropdown and the desktop navbar both -- under
 * the rule that separates the sections from everything else. Signing *out*
 * stays in the corner, because it is not somewhere to go.
 *
 * **An icon rather than a button reading its own sentence.** The header used to
 * spend its width on the account's display name and a button reading "End guest
 * session", which on a 390px phone is most of the row. The cost is real and
 * worth naming: a sighted visitor cannot read whose session this is at a glance
 * -- they open the account page, which names it at the top. What does not
 * change is that the information is still *there*: the control carries the
 * words it replaced as its accessible name and as a tooltip, so a screen reader
 * hears exactly what it heard before.
 *
 * The guest branch survives for that reason. A guest's session is the only
 * copy of their work, and "End guest session" says what it ends rather than
 * borrowing the word for leaving an account you can come back to -- a
 * distinction an icon cannot draw, so the label carries it.
 */
export function AccountActions() {
  const { user, signOut } = useAuth()
  const navigate = useNavigate()
  const t = useT()

  /*
   * Out of the session, and off the page it happened on.
   *
   * Signing out used to leave the URL alone, which reads as a bug at every
   * address that is not `/`: the chrome swaps to the logged-out one underneath
   * you and the deep link you were on -- a sheet, a group, the account page --
   * either bounces through the gate or sits there naming a thing you can no
   * longer open. Landing on `/` is the honest answer, and it is the one address
   * that means something on both sides of the boundary: the carousel signed
   * out, the character list signed in.
   *
   * After the request rather than beside it, so the navigation happens once the
   * session is actually gone; `signOut` drops the local session even when the
   * request fails, so this is reached either way.
   */
  const endSession = async () => {
    await signOut()
    await navigate('/')
  }

  const signOutLabel = user?.anonymous ? t('auth.endGuestSession') : t('auth.signOut')

  return (
    // Pushed right together, so the header still ends in the way out whether
    // or not there is an account to link to.
    <Group gap="xs" ml="auto" wrap="nowrap">
      {/* The install offer, then the language, then the way out. The first
          two are the ones that are not about this session -- somebody signed
          out needs both, which is why SignInActions draws them as well.
          Usually nothing is drawn for the install: see ui/InstallAction.tsx. */}
      <InstallButton />
      <LocaleActions />
      <Tooltip label={signOutLabel} withArrow>
        <ActionIcon variant="subtle" aria-label={signOutLabel} onClick={() => void endSession()}>
          <IconLogout size={20} />
        </ActionIcon>
      </Tooltip>
    </Group>
  )
}
