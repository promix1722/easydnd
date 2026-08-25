import { Link, Outlet, useLocation } from 'react-router'

import { useAuth } from '@/lib/auth'
import { Anchor, AppShell, Burger, Button, Group, NavLink, useDisclosure } from '@/ui'

import { activeNavPath, NAV_ITEMS } from './nav'
import { Wordmark } from './Wordmark'

/**
 * Wide-screen chrome: a persistent navbar beside the content.
 */
export function DesktopShell() {
  const [opened, { toggle }] = useDisclosure(true)
  const { pathname } = useLocation()
  const { user, signOut } = useAuth()

  return (
    <AppShell
      header={{ height: 56 }}
      navbar={{ width: 240, breakpoint: 'never', collapsed: { desktop: !opened } }}
      padding="lg"
    >
      <AppShell.Header>
        <Group h="100%" px="md" gap="sm">
          <Burger opened={opened} onClick={toggle} size="sm" aria-label="Toggle navigation" />
          <Wordmark />
          <Group gap="sm" ml="auto">
            {/* The account lives in the corner that names the account, beside
                the control that ends the session -- not in the navbar, which
                lists the parts of the app rather than who is using it. The
                name *is* the link: naming whose session this is and offering
                the way into it are one job, and a button labelled "Account"
                beside the account's own name was that sentence said twice.
                A session with nothing to show under a name falls back to the
                word, since a link with no label is a link nobody can find. */}
            {user ? (
              <Anchor component={Link} to="/account" size="sm" c="dimmed">
                {user.display_name.trim() || 'Account'}
              </Anchor>
            ) : null}
            {/* A guest's session is the only copy of their work, so the
                button that ends it says what it ends rather than borrowing
                the word for leaving an account you can come back to. */}
            <Button variant="subtle" onClick={() => void signOut()}>
              {user?.anonymous ? 'End guest session' : 'Sign out'}
            </Button>
          </Group>
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
