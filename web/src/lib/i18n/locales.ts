/**
 * The languages this client ships, and the one it falls back to.
 *
 * Mirrors internal/domain/rules/locale.go, which is the authority: the server
 * resolves SRD prose into one of these and answers `GET /v1/catalog` with the
 * list it actually holds. This constant exists so the switcher can be drawn
 * before that round trip has happened, not to be a second opinion about it --
 * `getManifest()` is what says which locales a deployment really has.
 */
export const LOCALES = ['en', 'ru'] as const

export type Locale = (typeof LOCALES)[number]

/**
 * English, because SRD 5.1 is published in it and it is the only locale
 * guaranteed to be complete. Every missing key in every other locale resolves
 * here -- per key, not per file, so a translated button beside an untranslated
 * paragraph is a working state rather than a broken one.
 */
export const DEFAULT_LOCALE: Locale = 'en'

/** The name each language calls itself. Never translated -- that is the point. */
export const LOCALE_NAMES: Record<Locale, string> = {
  en: 'English',
  ru: 'Русский',
}

/** Narrows a language tag to one this client has a catalogue for. */
function isLocale(tag: string): tag is Locale {
  return (LOCALES as readonly string[]).includes(tag)
}

/**
 * The locale a tag asks for, or null.
 *
 * "ru-RU" resolves to "ru": a region is a preference about dates and currency,
 * and this client has one catalogue per language. Matching on the base tag is
 * also what `acceptedLanguages` does on the Go side, so a browser sending
 * "ru-RU" gets the same answer from both halves of the app.
 */
export function localeOf(tag: string): Locale | null {
  const base = tag.trim().toLowerCase().split('-')[0] ?? ''
  return isLocale(base) ? base : null
}
