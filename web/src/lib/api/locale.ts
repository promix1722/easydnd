/**
 * Which language this client is asking the API to answer in.
 *
 * The server negotiates per request -- `?locale=` first, then
 * `Accept-Language`, then English (internal/api/http/helpers/locale.go) -- and
 * the query parameter exists precisely because a page cannot rewrite the
 * header its own browser sent. So a language the visitor *chose* has to travel
 * as a parameter, and `request()` appends it to every call.
 *
 * It is module state rather than an argument because `request` is a plain
 * function with something like sixty call sites, none of which have any
 * business knowing about language. The cost of that is real and is paid in
 * `src/test/setup.ts`: the suite shares one module registry per worker
 * (`isolate: false`), so a locale left set by one test file would be inherited
 * by every file that ran after it. `resetRequestLocale` is on that afterEach
 * list for exactly this reason.
 */
import { DEFAULT_LOCALE, type Locale } from '../i18n/locales'

let current: Locale = DEFAULT_LOCALE

/** The locale every subsequent request will ask for. */
export function requestLocale(): Locale {
  return current
}

/** Points subsequent requests at a language. Called by the locale provider. */
export function setRequestLocale(locale: Locale): void {
  current = locale
}

/** Back to English. For `src/test/setup.ts`. */
export function resetRequestLocale(): void {
  current = DEFAULT_LOCALE
}
