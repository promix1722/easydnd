import { MantineProvider } from '@mantine/core'
import type { ReactNode } from 'react'

import { theme } from './theme'

import '@mantine/core/styles.css'

/**
 * Wraps the app in the design system. Exists so that `main.tsx` -- which sits
 * outside `src/ui` -- does not have to import Mantine to bootstrap it.
 */
export function AppTheme({ children }: { children: ReactNode }) {
  return (
    <MantineProvider theme={theme} defaultColorScheme="auto">
      {children}
    </MantineProvider>
  )
}
