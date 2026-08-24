import { Link, Outlet, useLocation, useNavigate } from 'react-router'

import { useAuth } from '@/lib/auth'
import { Anchor, AppShell, Button, Group, Tabs } from '@/ui'

import { NAV_ITEMS } from './nav'
import { Wordmark } from './Wordmark'

const TAB_BAR_HEIGHT = 56

/**
 * Narrow-screen chrome: a compact header and a thumb-reachable bottom tab bar.
 *
 * Bottom rather than top because the top of a phone screen is the hardest
 * place to reach one-handed, which is how this app gets used at a table.
 */
export function MobileShell() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { user, signOut } = useAuth()

  // A nested route keeps the parent section highlighted; exact match alone
  // would blank the tab bar as soon as anyone opened a detail page.
  const active =
    NAV_ITEMS.filter((item) => item.to === '/' ? pathname === '/' : pathname.startsWith(item.to))
      .map((item) => item.to)
      .at(-1) ?? null

  return (
    <AppShell header={{ height: 52 }} footer={{ height: TAB_BAR_HEIGHT }} padding="sm">
      <AppShell.Header>
        <Group h="100%" px="md">
          <Wordmark order={4} />
          {/* Top right rather than a tab: see nav.ts. The tab bar is for the
              parts of the app, and the account is not one of them. The pair
              is pushed right together, so the header still ends in the
              account whether or not there is a name to draw. */}
          <Group gap="sm" ml="auto">
            {/* See DesktopShell: the name is the link, so this header says
                whose session it is instead of spending its narrowest row on
                a button that repeats the word above the page it opens. */}
            {user ? (
              <Anchor component={Link} to="/account" size="sm" c="dimmed">
                {user.display_name.trim() || 'Account'}
              </Anchor>
            ) : null}
            {/* See DesktopShell: ending a guest session destroys the only copy
                of whatever they built, and the label should say so. */}
            <Button variant="subtle" onClick={() => void signOut()}>
              {user?.anonymous ? 'End guest session' : 'Sign out'}
            </Button>
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>

      <AppShell.Footer>
        <Tabs
          value={active}
          onChange={(value) => {
            if (value) void navigate(value)
          }}
          variant="default"
          h="100%"
        >
          <Tabs.List grow h="100%">
            {NAV_ITEMS.map((item) => (
              <Tabs.Tab key={item.to} value={item.to}>
                {item.shortLabel ?? item.label}
              </Tabs.Tab>
            ))}
          </Tabs.List>
        </Tabs>
      </AppShell.Footer>
    </AppShell>
  )
}
