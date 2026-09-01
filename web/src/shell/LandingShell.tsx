import { Outlet } from 'react-router'

import { AppShell, Group, PAGE_BACKDROP } from '@/ui'

import { HEADER_BOX, SAFE_BOTTOM, SAFE_TOP } from './chrome'
import { LandingFooter } from './LandingFooter'
import { SignInActions } from './SignInActions'
import { Wordmark } from './Wordmark'

/**
 * Logged-out chrome: a wordmark, and the way in.
 *
 * No navbar and no tab bar, because there is nowhere to navigate to yet --
 * every section of the app is somebody's, and nobody is signed in. The sign-in
 * controls sit top right, where the signed-in header keeps "Sign out", so the
 * control that matters is in the same corner on both sides of the boundary.
 *
 * There is a footer, which the signed-in shells do not have. It carries the
 * SRD 5.1 attribution's way in, and the audience that needs to read a licence
 * notice before trusting an app with a character is the one that has not signed
 * in yet -- see `./LandingFooter.tsx`. `MobileShell` could not have it anyway:
 * it spends its one `AppShell.Footer` slot on the tab bar.
 *
 * The footer's height is declared here rather than inside `LandingFooter`,
 * because `AppShell` is what publishes it as `--app-shell-footer-offset` -- and
 * `routes/LandingPage.tsx` sizes itself against that property rather than
 * against a number repeated in two files.
 */
export function LandingShell() {
  return (
    <AppShell
      header={{ height: HEADER_BOX }}
      footer={{ height: `calc(48px + ${SAFE_BOTTOM})` }}
      padding="lg"
      // The bars grow into the strips the hardware covers and paint there;
      // their contents stay where they were. See ./chrome.ts.
      styles={{ header: { paddingTop: SAFE_TOP }, footer: { paddingBottom: SAFE_BOTTOM } }}
    >
      <AppShell.Header>
        <Group h="100%" px="md" gap="sm">
          <Wordmark />
          <Group gap="sm" ml="auto">
            <SignInActions />
          </Group>
        </Group>
      </AppShell.Header>

      {/* The picture behind every page this chrome wraps, the landing carousel
          included -- see ui/backdrop.ts. The carousel used to be the exception,
          on the grounds that it is three photographs already and a washed-out
          fourth showing between two panels is the page arguing with itself. It
          is not an exception any more: one ground under every page is worth more
          than the argument, and at 88% wash what shows through the slide gap is
          texture rather than a picture. */}
      <AppShell.Main style={PAGE_BACKDROP}>
        <Outlet />
      </AppShell.Main>

      <AppShell.Footer>
        <LandingFooter />
      </AppShell.Footer>
    </AppShell>
  )
}
