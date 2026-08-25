import { MantineProvider } from '@mantine/core'
import type { ReactNode } from 'react'

import { theme } from './theme'

import '@mantine/core/styles.css'
// Carousel ships its own stylesheet; it is not part of core's. Imported here
// rather than in the one screen that uses it, because src/ui is where the
// design system is assembled and a feature importing Mantine CSS would be the
// same leak the layer check exists to stop.
import '@mantine/carousel/styles.css'

/**
 * Wraps the app in the design system. Exists so that `main.tsx` -- which sits
 * outside `src/ui` -- does not have to import Mantine to bootstrap it.
 *
 * `env` is Mantine's own escape hatch for jsdom, and the test render helper is
 * the only caller that sets it. Without it a popover-backed control -- Select,
 * Menu -- opens and then hides itself again: Mantine hides a dropdown whose
 * anchor is not visible, and in a environment that lays nothing out no anchor
 * ever is. It is a prop rather than a check on import.meta.env so that the
 * production bundle cannot reach the branch at all.
 */
export function AppTheme({ children, env }: { children: ReactNode; env?: 'test' }) {
  return (
    // Spread rather than pass through: exactOptionalPropertyTypes means
    // `env={undefined}` is not the same as omitting it.
    <MantineProvider theme={theme} defaultColorScheme="auto" {...(env ? { env } : {})}>
      {children}
    </MantineProvider>
  )
}
