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
export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/', label: 'Characters', shortLabel: 'Party' },
]
