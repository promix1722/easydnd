/**
 * The navigation table, declared once and consumed by both shells. Keeping it
 * out of the shell components is what stops the desktop navbar and the mobile
 * dropdown from drifting apart as sections are added.
 */
export interface NavItem {
  /** Route path, also the menu item's value. */
  to: string
  label: string
}

// Account is deliberately absent: it is reached from the top right of both
// shells, where a profile icon links to it next to the one that ends the
// session, rather than being a section of the app -- the whole page is about
// the identity that header names, which is why the way in sits beside the way
// out rather than in this list.
//
// So is system status: `/status` is a deploy diagnostic, not a part of the app
// somebody navigates around, and it renders in the landing chrome for everyone
// rather than inside the signed-in shell. See routes/index.tsx.
//
// And so is `/legal`, on the same grounds and for a different reason: it is a
// document rather than a section, it renders in the landing chrome for
// everyone, and it is reached from the footer that chrome carries. See
// shell/LandingFooter.tsx.
export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/', label: 'Characters' },
  { to: '/groups', label: 'Groups' },
  // A placeholder section: the page behind it says so. See
  // routes/GamesPlaceholder.tsx.
  { to: '/games', label: 'Games' },
]

/**
 * Which section a path belongs to, or null.
 *
 * A nested route keeps its parent section highlighted -- exact matching alone
 * blanks the navigation as soon as anybody opens a detail page. `/` is the
 * special case: as a prefix it matches everything, so it only ever matches
 * itself, and the last (most specific) match wins.
 *
 * It lives here rather than in a shell because both shells need it and they
 * had drifted: the tab bar handled nesting and the desktop navbar did not.
 */
export function activeNavPath(pathname: string): string | null {
  return (
    NAV_ITEMS.filter((item) => (item.to === '/' ? pathname === '/' : pathname.startsWith(item.to)))
      .map((item) => item.to)
      .at(-1) ?? null
  )
}

/**
 * What the mobile dropdown's trigger reads.
 *
 * The desktop navbar can leave every entry unlit when `activeNavPath` returns
 * null -- on a character sheet, on `/account`, on a 404 -- because the list is
 * still on screen saying where you could go. The dropdown has no such luxury:
 * it is one control, it is the only thing naming the current place, and a
 * button with no label is a button nobody presses. So the fallback is the word
 * for what the control *is* rather than a guess at where you are; lighting
 * Characters on `/characters/:id` would mean widening `activeNavPath`, and it
 * is deliberately narrow -- see its note about `/` swallowing everything.
 */
export function navLabel(pathname: string): string {
  const active = activeNavPath(pathname)
  return NAV_ITEMS.find((item) => item.to === active)?.label ?? 'Menu'
}
