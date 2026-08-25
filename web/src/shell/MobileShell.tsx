import { Link, Outlet, useLocation } from 'react-router'

import { AppShell, Button, Group, IconCheck, IconChevronDown, Menu } from '@/ui'

import { AccountActions } from './AccountActions'
import { HEADER_HEIGHT } from './chrome'
import { activeNavPath, NAV_ITEMS, navLabel } from './nav'
import { Wordmark } from './Wordmark'

/**
 * Narrow-screen chrome: one row, holding everything.
 *
 * This replaced a header plus a thumb-reachable bottom tab bar, and the
 * argument it replaced is worth keeping in view because it was a good one: the
 * top of a phone screen is the hardest place to reach one-handed, which is how
 * this app gets used at a table. What outweighed it is that the two rows cost
 * 108px of a 390x844 screen before any content was drawn, and the header's own
 * contents -- a mark, the word "easydnd", an account name and a button reading
 * "End guest session" -- had already outgrown the width they shared. A
 * dropdown collapses every section into one control that costs the same
 * whether there are two of them or six, which a tab bar does not.
 *
 * So the row is: the mark, the section you are in, and the account. The word
 * "easydnd" is gone from it -- see `./Wordmark.tsx` -- and the account is two
 * icons -- see `./AccountActions.tsx`.
 *
 * `AppShell.Footer` is now unused here. That matters beyond this file: the
 * landing chrome's footer carries the SRD 5.1 attribution's way in, and the
 * reason the signed-in phone chrome could not have one was that the tab bar
 * owned the only slot. It no longer does. Nothing fills it yet -- see
 * docs/licensing.md#known-gaps, where that gap is recorded as open.
 */
export function MobileShell() {
  const { pathname } = useLocation()

  // Shared with the desktop navbar: see activeNavPath in nav.ts.
  const active = activeNavPath(pathname)

  return (
    <AppShell header={{ height: HEADER_HEIGHT }} padding="sm">
      <AppShell.Header>
        <Group h="100%" px="md" gap={4} wrap="nowrap">
          <Wordmark caption={false} />

          <Menu position="bottom-start" withinPortal>
            <Menu.Target>
              {/* The one deliberate override of the theme's xs Button, and the
                  reason is a tap target rather than taste: this is the whole of
                  the app's navigation on a phone, and 30px is under every
                  guideline there is. No aria-label -- the visible text is the
                  name, and Menu.Target supplies aria-haspopup and
                  aria-expanded on its own. */}
              <Button
                variant="subtle"
                size="sm"
                px="xs"
                rightSection={<IconChevronDown size={16} />}
              >
                {navLabel(pathname)}
              </Button>
            </Menu.Target>
            {/* Real links rather than an onChange that navigates: the desktop
                navbar's entries are links, and a section should be the same
                kind of thing to a browser on both. */}
            <Menu.Dropdown>
              {NAV_ITEMS.map((item) => (
                <Menu.Item
                  key={item.to}
                  component={Link}
                  to={item.to}
                  aria-current={active === item.to ? 'page' : undefined}
                  // The tick is hidden rather than absent so every row keeps
                  // the same left edge -- labels that shuffle sideways as you
                  // move between sections read as a different list each time.
                  leftSection={
                    <IconCheck
                      size={16}
                      style={{ visibility: active === item.to ? 'visible' : 'hidden' }}
                    />
                  }
                >
                  {item.label}
                </Menu.Item>
              ))}
            </Menu.Dropdown>
          </Menu>

          <AccountActions />
        </Group>
      </AppShell.Header>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
