import { Outlet } from 'react-router'

import { AppShell, Group } from '@/ui'

import { SignInActions } from './SignInActions'
import { Wordmark } from './Wordmark'

/**
 * Logged-out chrome: a wordmark, and the way in.
 *
 * No navbar and no tab bar, because there is nowhere to navigate to yet --
 * every section of the app is somebody's, and nobody is signed in. The sign-in
 * controls sit top right, where the signed-in header keeps "Sign out", so the
 * control that matters is in the same corner on both sides of the boundary.
 */
export function LandingShell() {
  return (
    <AppShell header={{ height: 56 }} padding="lg">
      <AppShell.Header>
        <Group h="100%" px="md" gap="sm">
          <Wordmark />
          <Group gap="sm" ml="auto">
            <SignInActions />
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
