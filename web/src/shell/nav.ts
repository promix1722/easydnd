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

// Account is deliberately absent: it is a header control in the top right of
// both shells, next to the button that ends the session, rather than a section
// of the app -- the whole page is about the identity that header names.
//
// So is system status: `/status` is a deploy diagnostic, not a part of the app
// somebody navigates around, and it renders in the landing chrome for everyone
// rather than inside the signed-in shell. See routes/index.tsx.
export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/', label: 'Characters', shortLabel: 'Party' },
]
