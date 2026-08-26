import { Link, Outlet, useLocation } from 'react-router'

import {
  AppShell,
  Button,
  Group,
  IconCheck,
  IconChevronDown,
  Menu,
  SECTIONS,
  sectionFor,
} from '@/ui'

import { AccountActions } from './AccountActions'
import { HEADER_HEIGHT } from './chrome'
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

  // Shared with the desktop navbar: see sectionFor in ui/sections.ts.
  const active = sectionFor(pathname)

  /*
   * What the trigger reads.
   *
   * The desktop navbar can leave every entry unlit -- on `/account`, on a 404
   * -- because the list is still on screen saying where you could go. This has
   * no such luxury: it is one control, it is the only thing naming the current
   * place, and a button with no label is a button nobody presses. So the
   * fallback is the word for what the control *is*.
   *
   * It fires far less often than it used to. `sectionFor` now knows that a
   * character sheet belongs to Characters, where the old `activeNavPath`
   * matched on the link target alone and so answered "nowhere" for every
   * detail page under `/`.
   */
  const label = active?.label ?? 'Menu'

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
                // The section's own glyph, which is also what the desktop
                // navbar draws beside this label. It carries more weight here
                // than it does there: this control is the only thing on a
                // phone naming where you are, and `ui/Page` now drops the
                // section crumb below `md` precisely because this says it --
                // so the glyph is the section's mark on the page, not
                // decoration. Absent on a path in no section, where the label
                // falls back to "Menu" and there is no glyph to draw.
                {...(active ? { leftSection: <active.icon size={16} /> } : {})}
                rightSection={<IconChevronDown size={16} />}
              >
                {label}
              </Button>
            </Menu.Target>
            {/* Real links rather than an onChange that navigates: the desktop
                navbar's entries are links, and a section should be the same
                kind of thing to a browser on both. */}
            <Menu.Dropdown>
              {SECTIONS.map((section) => (
                <Menu.Item
                  key={section.to}
                  component={Link}
                  to={section.to}
                  aria-current={active?.to === section.to ? 'page' : undefined}
                  leftSection={<section.icon size={16} />}
                  // The tick moved to the right when the sections got glyphs.
                  // It used to sit on the left, drawn but invisible on every
                  // inactive row, purely to stop the labels shuffling sideways
                  // as you moved between sections -- a job the section's own
                  // glyph now does, on every row, while also saying something.
                  // Still hidden rather than absent, for the same alignment
                  // reason it was before.
                  rightSection={
                    <IconCheck
                      size={16}
                      style={{ visibility: active?.to === section.to ? 'visible' : 'hidden' }}
                    />
                  }
                >
                  {section.label}
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
