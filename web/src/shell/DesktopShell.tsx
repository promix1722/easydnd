import { Link, Outlet, useLocation } from 'react-router'

import { useAuth } from '@/lib/auth'
import { AppShell, Burger, Button, Group, NavLink, Text, useDisclosure } from '@/ui'

import { NAV_ITEMS } from './nav'
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
            {user ? (
              <Text size="sm" c="dimmed">
                {user.display_name}
              </Text>
            ) : null}
            {/* The account lives in the corner that names the account, beside
                the control that ends the session -- not in the navbar, which
                lists the parts of the app rather than who is using it. */}
            <Button component={Link} to="/account" variant="subtle">
              Account
            </Button>
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
            active={pathname === item.to}
          />
        ))}
      </AppShell.Navbar>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
