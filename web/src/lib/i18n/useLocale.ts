import { useTranslation } from 'react-i18next'

import { remember } from './instance'
import { DEFAULT_LOCALE, type Locale, localeOf } from './locales'

/**
 * Reading and changing the language.
 *
 * Beside `LocaleProvider` rather than inside it because oxlint's
 * `react/only-export-components` allows a component's module to export a
 * constant next to it but not a hook -- the same rule that put `pageState`
 * beside `Page` rather than in it.
 */

/** The language the app is currently in. */
export function useLocale(): Locale {
  const { i18n } = useTranslation()
  return localeOf(i18n.language) ?? DEFAULT_LOCALE
}

/**
 * Switching the language.
 *
 * Returns a setter rather than exposing `i18n` itself, so that a screen cannot
 * reach past this module into i18next's own API -- the same reason `@/ui`
 * re-exports Mantine rather than letting every feature import it.
 */
export function useSetLocale(): (locale: Locale) => void {
  const { i18n } = useTranslation()
  return (locale: Locale) => {
    remember(locale)
    void i18n.changeLanguage(locale)
  }
}
