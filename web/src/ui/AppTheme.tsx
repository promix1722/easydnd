import { MantineProvider } from '@mantine/core'
import type { ReactNode } from 'react'

import { cssVariables, theme } from './theme'

// The *layered* build of Mantine's stylesheet, rather than the plain one, and
// the reason is `./app.css` below. `styles.layer.css` wraps every rule in
// `@layer mantine`, and an unlayered rule beats a layered one whatever the two
// selectors weigh -- so the control sizes in `app.css` land without a
// specificity contest, an `!important`, or a dependency on which file the
// bundler happens to emit first. The rules inside are otherwise identical.
import '@mantine/core/styles.layer.css'
// Carousel ships its own stylesheet; it is not part of core's. Imported here
// rather than in the one screen that uses it, because src/ui is where the
// design system is assembled and a feature importing Mantine CSS would be the
// same leak the layer check exists to stop. Layered for the same reason as
// core's, so that the two cannot end up on different sides of the cascade.
import '@mantine/carousel/styles.layer.css'
// The app's own, and its only one. Last, so it is unambiguous that it is the
// one doing the overriding. See the file for what it does and why it is here
// rather than expressed as theme values.
import './app.css'

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
    <MantineProvider
      theme={theme}
      cssVariablesResolver={cssVariables}
      defaultColorScheme="auto"
      {...(env ? { env } : {})}
    >
      {children}
    </MantineProvider>
  )
}
