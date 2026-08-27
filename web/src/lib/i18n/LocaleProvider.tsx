import type { i18n as I18n } from 'i18next'
import { type ReactNode, useEffect, useMemo } from 'react'
import { I18nextProvider } from 'react-i18next'

import { resetCatalogCache } from '../api/catalog'
import { setRequestLocale } from '../api/locale'

import { createI18n } from './instance'
import { useLocale } from './useLocale'

/**
 * Puts a language around the app.
 *
 * It sits outside the router and beside `AuthProvider`, for the same reason
 * that one does: which language the chrome is in is not a fact about any
 * route, and a screen that had to wait for it would flash English first.
 *
 * `i18n` is a prop rather than a module-level default because the suite runs
 * without isolation -- see `createI18n`. `src/test/render.tsx` passes an
 * instance pinned to English, which is what keeps six hundred assertions
 * written against English copy passing unchanged.
 */
export function LocaleProvider({ i18n, children }: { i18n?: I18n; children: ReactNode }) {
  const instance = useMemo(() => i18n ?? createI18n(), [i18n])

  return (
    <I18nextProvider i18n={instance}>
      <LocaleEffects />
      {children}
    </I18nextProvider>
  )
}

/**
 * The three things that have to happen outside React when the language changes.
 *
 * A component rather than a hook inside the provider above, because it has to
 * be *inside* `I18nextProvider` to read the instance from context -- the
 * provider's own body runs before that context exists.
 */
function LocaleEffects() {
  const locale = useLocale()

  useEffect(() => {
    // The document's language, for screen readers and hyphenation. Nothing in
    // React renders the <html> element, so this is the only place it can be
    // said.
    document.documentElement.lang = locale

    // The server resolves SRD prose per request and cannot read this client's
    // mind; `?locale=` is how it is told. See src/lib/api/locale.ts.
    setRequestLocale(locale)

    // The compendium already fetched is in the old language. The cache key
    // carries the locale so nothing *wrong* is served, but the abandoned copy
    // would otherwise be held for the life of the tab.
    resetCatalogCache()
  }, [locale])

  return null
}
