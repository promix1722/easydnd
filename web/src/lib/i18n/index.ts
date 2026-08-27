/**
 * Translation, and the language it is in.
 *
 * This directory is the **only** place in the client permitted to import
 * `i18next` or `react-i18next`; `npm run lint:layers` enforces it. The reason
 * is the one `@/ui` exists for: a vendor that every layer may reach for is a
 * vendor nothing can ever replace. Screens import `useT` from here.
 *
 * No user-facing word appears anywhere under `src/`. Every string lives in
 * `web/locales/en.json` and `web/locales/ru.json`, so translating this app
 * means editing two data files and never opening a component.
 */
export { LocaleProvider } from './LocaleProvider'
export { useLocale, useSetLocale } from './useLocale'
export { createI18n } from './instance'
export { DEFAULT_LOCALE, LOCALES, LOCALE_NAMES, localeOf } from './locales'
export type { Locale } from './locales'
export { useT } from './useT'
export type { Translate } from './useT'
export type { MessageKey } from './useT'
export { formatDate, formatDateTime } from './format'
