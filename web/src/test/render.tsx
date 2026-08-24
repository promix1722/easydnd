import { render, type RenderResult } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'

import { AppTheme } from '@/ui'

import { setViewport, type Viewport } from './viewport'

export interface RenderAtResult extends Omit<RenderResult, 'rerender'> {
  /**
   * Re-renders inside the same provider.
   *
   * Testing Library's own `rerender` replaces the whole tree, wrapper
   * included, which drops the theme provider and fails with an error about a
   * missing MantineProvider several frames deep. Wrapping it here means a
   * test that changes props reads the way it should.
   */
  rerender: (ui: ReactNode) => void
}

/**
 * Renders a component inside the design system at a chosen viewport. Every
 * responsive component should be exercised at both, because "works on my
 * 1440px screen" is not a claim this app can afford.
 */
export function renderAt(viewport: Viewport, ui: ReactElement): RenderAtResult {
  setViewport(viewport)
  const result = render(<AppTheme>{ui}</AppTheme>)
  return {
    ...result,
    rerender: (next: ReactNode) => {
      result.rerender(<AppTheme>{next}</AppTheme>)
    },
  }
}
