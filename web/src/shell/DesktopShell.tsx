import { Link, Outlet, useLocation } from 'react-router'

import { useT } from '@/lib/i18n'

import {
  AppShell,
  CHROME_INSET,
  Divider,
  Group,
  IconChevronLeft,
  IconChevronRight,
  NavLink,
  ROW_HEIGHT,
  SECTIONS,
  sectionFor,
  Tooltip,
  useDisclosure,
} from '@/ui'

import { AccountActions } from './AccountActions'
import { HEADER_HEIGHT } from './chrome'
import { Wordmark } from './Wordmark'

/** The id the rail's control points `aria-controls` at. */
const NAVBAR_ID = 'app-navbar'

/** Open: room for a glyph and its word. Collapsed: room for the glyph. */
const NAVBAR_WIDTH = 240
const RAIL_WIDTH = 64

/*
 * One row height, in both states -- and the same one the page's heading uses.
 *
 * Pinned rather than left to the content, because a row is 41px with a word in
 * it and 34px without one, so collapsing shortened every row and the list
 * jumped upward, the third section moving 14px while you were looking at it.
 *
 * It comes from `@/theme` rather than living here because `ui/Page` needs the
 * same number: a section is named twice on screen at once, once in this navbar
 * and again as the heading of the page it opened, and sharing the row height
 * (and `CHROME_INSET`) is what puts the two on one line.
 */

/**
 * How a row is drawn, open or narrowed.
 *
 * The radius is set here because Mantine's `NavLink` ships with none, and a
 * square-cornered highlight is barely noticeable at 219px wide but reads as a
 * cramped box at 43px. `md` is the app's default radius, so the rail's active
 * item is the same shape as the menu's, just narrower.
 */
function rowStyles(opened: boolean) {
  return {
    root: {
      borderRadius: 'var(--mantine-radius-md)',
      minHeight: ROW_HEIGHT,
      ...(opened ? {} : { justifyContent: 'center' }),
    },
    // The label is removed rather than hidden so it cannot be read out on the
    // rail; each link keeps its name through `aria-label` instead.
    ...(opened ? {} : { section: { marginInlineEnd: 0 }, body: { display: 'none' } }),
  }
}

/**
 * Wide-screen chrome: a navbar that narrows to a rail of glyphs.
 *
 * This file used to argue that a wide screen has room for a 240px navbar and
 * no reason to give it up, and that was true while the content was whatever
 * width the window gave it. It stopped being true when every page got capped
 * at `CONTENT_MAX_WIDTH`: on a 1280px laptop, 240 of navbar plus 1024 of
 * content plus the shell's own padding does not fit, and the thing that gives
 * way is the table you were reading.
 *
 * **It narrows rather than disappears, and that is what fixes the control.**
 * The problem with a menu that vanishes is not the hiding, it is that the way
 * back has nowhere to live: put it in the header and it crowds the wordmark
 * (which is what the first attempt did, and what a `Burger` defaulting to open
 * did before that -- it drew a close cross, left of the mark, before the app's
 * own name). A rail always has a foot, so the control has one address in both
 * states, sitting at the bottom of the thing it resizes, pointing the way it
 * will move. It costs 64px of the 240 it gives back; what it buys is a
 * navigation that never leaves the screen and never has to be found again.
 *
 * The section glyphs earn their keep twice over here: on the rail they are the
 * whole of the navigation, so each keeps its name in a tooltip and in its
 * accessible name -- see `ui/sections.ts`, where the glyphs are chosen.
 *
 * **The state is not remembered.** There is no `localStorage` anywhere in this
 * client, and the sheet's "Hide untrained" toggle is already deliberately
 * unpersisted; a second unpersisted toggle is consistent, where a persisted one
 * would be this app's first stored preference and would have to earn that. It
 * would also be a *setting* that no page lists, which is how you get a bug
 * report reading "the menu is gone" from somebody who collapsed it once a month
 * ago. Reading it back at mount would mean painting the navbar wide and
 * snapping it narrow on every load.
 *
 * A note for whoever reaches for `AppShell`'s `collapsed` prop: it is read only
 * when Mantine considers the layout to be in its desktop mode, and
 * `breakpoint: 'never'` opts out of having modes -- so `collapsed` silently
 * does nothing here. Resizing `width` is what this shell does instead, and it
 * needs no breakpoint at all.
 */
export function DesktopShell() {
  const { pathname } = useLocation()
  const [opened, { toggle }] = useDisclosure(true)

  const t = useT()
  const active = sectionFor(pathname)
  const label = opened ? t('nav.collapse') : t('nav.expand')

  return (
    <AppShell
      header={{ height: HEADER_HEIGHT }}
      navbar={{ width: opened ? NAVBAR_WIDTH : RAIL_WIDTH, breakpoint: 'never' }}
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

      <AppShell.Navbar id={NAVBAR_ID} p={CHROME_INSET}>
        {SECTIONS.map((section) => {
          const link = (
            <NavLink
              key={section.to}
              component={Link}
              to={section.to}
              // On the rail the glyph is the whole control, so the section's
              // name has to survive as the link's accessible name -- otherwise
              // the navigation reads as three unlabelled links.
              aria-label={t(section.label)}
              {...(opened ? { label: t(section.label) } : {})}
              leftSection={<section.icon size={18} />}
              active={active?.to === section.to}
              styles={rowStyles(opened)}
            />
          )

          // A tooltip only where the word is gone. With the label on screen it
          // would be a second copy of what you are already reading.
          return opened ? (
            link
          ) : (
            <Tooltip key={section.to} label={t(section.label)} position="right" withArrow>
              {link}
            </Tooltip>
          )
        })}

        {/*
          Directly under the sections, not at the foot of the navbar.

          It sat at the bottom first, which is where a sidebar's chrome
          conventionally goes -- but this navbar is as tall as the window and
          holds three items, so the control ended up eight hundred pixels below
          the last thing anybody had looked at, and was missed. A rule separates
          it from the sections so it still reads as chrome rather than a fourth
          place to go.

          Drawn as a `NavLink` like the sections themselves, so it inherits
          their geometry exactly instead of being aligned to them by hand, and
          dimmed so it does not compete with the navigation above it.
        */}
        <Divider my="xs" />
        <ControlRow
          opened={opened}
          onToggle={toggle}
          controls={NAVBAR_ID}
          label={label}
        />
      </AppShell.Navbar>

      {/* The navbar's own top inset, so the first navbar row and the page's
          heading row start at the same y and the section's name lines up with
          itself across the divider. The sides and bottom keep the shell's
          roomier `lg`.

          The header offset has to be carried explicitly: Mantine builds this
          padding as `header-offset + shell-padding`, so overriding `pt` with a
          bare number drops the offset and slides the whole page up underneath
          the header. */}
      <AppShell.Main
        style={{
          paddingTop: `calc(var(--app-shell-header-offset, 0rem) + ${CHROME_INSET}px)`,
        }}
      >
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}

/** The control that resizes the navbar, drawn as one more row in it. */
function ControlRow({
  opened,
  onToggle,
  controls,
  label,
}: {
  opened: boolean
  onToggle: () => void
  controls: string
  label: string
}) {
  const t = useT()
  const row = (
    <NavLink
      component="button"
      onClick={onToggle}
      aria-label={label}
      aria-expanded={opened}
      aria-controls={controls}
      leftSection={opened ? <IconChevronLeft size={18} /> : <IconChevronRight size={18} />}
      {...(opened ? { label: t('nav.collapseShort') } : {})}
      styles={{
        ...rowStyles(opened),
        root: { ...rowStyles(opened).root, color: 'var(--mantine-color-dimmed)' },
      }}
    />
  )

  return opened ? (
    row
  ) : (
    <Tooltip label={label} position="right" withArrow>
      {row}
    </Tooltip>
  )
}
