import { Link, Outlet, useLocation } from 'react-router'

import { AppShell, Group, NavLink } from '@/ui'

import { AccountActions } from './AccountActions'
import { HEADER_HEIGHT } from './chrome'
import { activeNavPath, NAV_ITEMS } from './nav'
import { Wordmark } from './Wordmark'

/**
 * Wide-screen chrome: a persistent navbar beside the content.
 *
 * Persistent, and with nothing to collapse it. There was a burger, defaulting
 * to open -- so what it actually drew was a close cross sitting to the left of
 * the mark, before the app's own name, reading as a way to dismiss something
 * rather than as a menu. A wide screen has room for a 240px navbar next to the
 * content and no reason to hide it, so the control that hid it went. The
 * viewport that genuinely cannot spare the width does not use this shell at
 * all -- see ./MobileShell.tsx.
 */
export function DesktopShell() {
  const { pathname } = useLocation()

  return (
    <AppShell
      header={{ height: HEADER_HEIGHT }}
      navbar={{ width: 240, breakpoint: 'never' }}
      padding="lg"
    >
      <AppShell.Header>
        <Group h="100%" px="md" gap="sm">
          <Wordmark />
          {/* The account lives in the corner that names the account, beside
              the control that ends the session -- not in the navbar, which
              lists the parts of the app rather than who is using it. Shared
              with the phone header rather than written twice here: see
              ./AccountActions.tsx, which is also where the reasoning for two
              icons over a name and a button lives. */}
          <AccountActions />
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="xs">
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.to}
            component={Link}
            to={item.to}
            label={item.label}
            active={activeNavPath(pathname) === item.to}
          />
        ))}
      </AppShell.Navbar>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
