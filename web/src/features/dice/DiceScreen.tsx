import { D20Roll } from '@/ui'

/**
 * A page that is a die and nothing else.
 *
 * This was a full-screen dialog first, and the dialog was wrong twice over. It
 * covered the header, so the menu you opened the die from was unreachable
 * until you dismissed it; and once it was cut back to sit *below* the header
 * it still needed a close button of its own, which is a second way out of a
 * place you can already leave through the menu. A page needs neither: the
 * chrome is simply there, every section is one press away, and leaving is
 * navigating rather than dismissing.
 *
 * No heading, no crumb and no caption, which is the same argument the die
 * itself makes -- the camera looks straight down, so the number that landed is
 * the one facing you, and a page whose entire content is a die does not need a
 * line of text explaining that. `ui/Page` is deliberately not used for that
 * reason: it exists to put a breadcrumb and a title above a screen, and this
 * screen wants neither.
 */

/**
 * Fills what the header leaves, the way `routes/LandingPage.tsx` does.
 *
 * Every term is `AppShell`'s own custom property, so changing the header
 * height in `shell/MobileShell.tsx` resizes this with it and no number is
 * repeated across the two files. Wrapped in `calc(...)` rather than left as a
 * bare `max(...)` because Mantine's `rem()` passes a length through untouched
 * only when it begins `calc(` or `clamp(` -- a bare `max(` is split on its
 * spaces and converted into nonsense.
 */
const FILL_MAIN =
  'calc(max(320px, 100dvh' +
  ' - var(--app-shell-header-offset, 0rem)' +
  ' - var(--app-shell-padding, 0rem) * 2))'

export function DiceScreen() {
  return (
    <div style={{ height: FILL_MAIN, overflow: 'hidden' }}>
      <D20Roll />
    </div>
  )
}
