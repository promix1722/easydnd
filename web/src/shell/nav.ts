/**
 * The navigation table, declared once and consumed by both shells. Keeping it
 * out of the shell components is what stops the desktop navbar and the mobile
 * tab bar from drifting apart as sections are added.
 */
export interface NavItem {
  /** Route path, also the tab value. */
  to: string
  label: string
  /** Short form for the mobile tab bar, where space is measured in characters. */
  shortLabel?: string
}

// Account is deliberately absent: it is reached from the top right of both
// shells, where the signed-in name links to it next to the button that ends
// the session, rather than being a section of the app -- the whole page is
// about the identity that header names, which is why the name is the link.
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
  { to: '/', label: 'Characters', shortLabel: 'Party' },
  { to: '/groups', label: 'Groups' },
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
