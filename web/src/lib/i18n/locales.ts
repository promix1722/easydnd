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

/**
 * A flag beside each name in the switcher's menu.
 *
 * A language is not a country and the mapping is a lie in the general case --
 * English is not Britain's alone -- but with two entries in a dropdown the flag
 * is a colour to aim at rather than a claim, and the name beside it is what
 * actually says which language it is.
 *
 * ponytail: emoji, not artwork. Windows ships no flag glyphs, so Chrome there
 * draws the two-letter code instead of a picture -- legible, and still the
 * right pair of letters. Swap for inline SVG only if that becomes a complaint.
 */
export const LOCALE_FLAGS: Record<Locale, string> = {
  en: '\u{1F1EC}\u{1F1E7}',
  ru: '\u{1F1F7}\u{1F1FA}',
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
