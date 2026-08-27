/**
 * Dates and numbers, in the language the app is in.
 *
 * `Date.prototype.toLocaleString()` was what this client used, and it is
 * subtly wrong the moment a language can be chosen: it follows the *browser's*
 * locale, so a visitor who switches easydnd to Russian on an English-configured
 * machine gets Russian captions above English dates. These take the app's
 * locale explicitly, which is the only one that is actually being displayed.
 *
 * The locale is a parameter rather than a hook, so these work in the pure
 * modules that need them -- `features/character/labels.ts` is handed one.
 */
export function formatDateTime(iso: string, locale: string): string {
  const when = new Date(iso)
  return Number.isNaN(when.getTime()) ? iso : when.toLocaleString(locale)
}

export function formatDate(iso: string, locale: string): string {
  const when = new Date(iso)
  return Number.isNaN(when.getTime()) ? iso : when.toLocaleDateString(locale)
}
