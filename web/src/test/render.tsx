import { render, type RenderResult } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'

import { createI18n, LocaleProvider, type Locale } from '@/lib/i18n'
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
 *
 * **The language is pinned to English, and every assertion in this suite
 * depends on it.** Around six hundred queries match visible copy --
 * `getByText('Wednesday Night')`, `getByRole('button', { name: 'New group' })`
 * -- and they went on passing unchanged when the captions moved into
 * `web/locales/`, because this is the one place that decides which catalogue
 * they are drawn from. A test that wants Russian passes `locale`.
 *
 * The instance is built per render rather than shared. That is not tidiness:
 * the suite runs with `isolate: false`, so a language set on a shared instance
 * by one file would be inherited by every file that ran after it, in whatever
 * order they happened to run. See `createI18n`.
 */
export function renderAt(
  viewport: Viewport,
  ui: ReactElement,
  locale: Locale = 'en',
): RenderAtResult {
  setViewport(viewport)
  const i18n = createI18n(locale)
  const wrap = (node: ReactNode) => (
    <LocaleProvider i18n={i18n}>
      <AppTheme env="test">{node}</AppTheme>
    </LocaleProvider>
  )
  const result = render(wrap(ui))
  return {
    ...result,
    rerender: (next: ReactNode) => {
      result.rerender(wrap(next))
    },
  }
}
