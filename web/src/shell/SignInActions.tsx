import { Link, useLocation } from 'react-router'

import { useAuth } from '@/lib/auth'
import { Button } from '@/ui'

/**
 * The identity control in the top right of the landing header: the way in, or
 * the way back for somebody who is already inside.
 *
 * One button, and it navigates rather than acting: the ways into this app cost
 * different things -- one keeps nothing, one leaves for a provider -- and
 * choosing between them needs room to say so, which a header corner does not
 * have. That belongs to /login, and this is the link to it -- so shell/ needs
 * to know nothing about ceremonies or passkeys.
 *
 * It lives in shell/ rather than features/ because the header is chrome and
 * shell/ may not import a feature -- the same reason the signed-in "Sign out"
 * button is built in DesktopShell. It sits in that button's corner, so the
 * control that matters is in the same place on both sides of the boundary; the
 * size the two share is the theme's, not this call site's.
 */
export function SignInActions() {
  const location = useLocation()
  const { status } = useAuth()

  // Nothing to offer on the page this button leads to.
  if (location.pathname === '/login') return null

  // This chrome is normally the logged-out one, but /status wears it for
  // everybody -- so somebody who is already signed in needs the way back to
  // the app rather than an invitation to sign in again.
  if (status === 'authenticated') {
    return (
      <Button component={Link} to="/" variant="subtle">
        Back to easydnd
      </Button>
    )
  }

  // Mid-check or offline: we do not know which of the two to offer, and
  // guessing "sign in" would tell a signed-in visitor they are not.
  if (status !== 'anonymous') return null

  return (
    // The current location rides along so that signing in returns the visitor
    // to the deep link they arrived on. This is what replaces the old "the URL
    // never changes" property, now that the way in is a page of its own.
    <Button component={Link} to="/login" state={{ from: location }}>
      Log in
    </Button>
  )
}
