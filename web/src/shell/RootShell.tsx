import { useIsDesktop } from '@/ui'

import { DesktopShell } from './DesktopShell'
import { MobileShell } from './MobileShell'

/**
 * Picks the chrome. This is the app's only top-level viewport branch: below
 * here, screens are viewport-agnostic and use the responsive primitives from
 * `@/ui` or Mantine's responsive props.
 *
 * Two shells rather than one adaptive shell because the desktop navbar and the
 * mobile tab bar share no markup -- only the route table in ./nav.ts, which is
 * exactly the part that must not diverge.
 */
export function RootShell() {
  return useIsDesktop() ? <DesktopShell /> : <MobileShell />
}
