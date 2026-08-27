import i18next, { type i18n as I18n } from 'i18next'
import { initReactI18next } from 'react-i18next'

import en from '@locales/en.json'
import ru from '@locales/ru.json'

import { DEFAULT_LOCALE, LOCALES, type Locale, localeOf } from './locales'

/**
 * Where the language a visitor chose is remembered.
 *
 * sessionStorage, not localStorage, and not the account: a language is this
 * visit's business. The consequence is worth knowing rather than discovering
 * -- a second tab autodetects again rather than inheriting the choice -- and
 * it is the same trade `features/groups/inviteToken.ts` makes for an
 * invitation. Moving to localStorage is a change to this one line.
 */
const STORAGE_KEY = 'easydnd.locale'

/**
 * The language this visit should open in.
 *
 * Hand-rolled rather than `i18next-browser-languagedetector`, which was here
 * first and was wrong in two ways that both matter. It leaves `i18n.language`
 * as the raw tag, so a browser asking for `ru-RU` produced a locale of
 * "ru-RU" -- resources still resolved, but every consumer had to normalise it
 * again. And it reads `sessionStorage` unguarded, so a private-mode browser
 * that refuses storage outright threw before the app rendered at all. Fifteen
 * lines here answer both, and the try/catch is the same one
 * `features/groups/inviteToken.ts` has needed all along.
 */
function detect(): Locale {
  const stored = read()
  if (stored !== null) return stored

  const asked = typeof navigator === 'undefined' ? [] : (navigator.languages ?? [navigator.language])
  for (const tag of asked) {
    const locale = localeOf(tag ?? '')
    if (locale !== null) return locale
  }
  return DEFAULT_LOCALE
}

/** What this visit chose, if a previous render of it wrote one down. */
function read(): Locale | null {
  try {
    return localeOf(window.sessionStorage.getItem(STORAGE_KEY) ?? '')
  } catch {
    // Storage refused. The choice is lost for this visit, which is a shame;
    // failing to render is not an option.
    return null
  }
}

/** Remembers the choice, if the browser will hold it. */
export function remember(locale: Locale): void {
  try {
    window.sessionStorage.setItem(STORAGE_KEY, locale)
  } catch {
    // As above: nothing else to do, and nothing worth breaking over.
  }
}

/**
 * Builds an i18next instance.
 *
 * **Never the `i18next` default export, and never `i18next.init()`.** The test
 * suite shares one module registry per worker (`isolate: false` in
 * vite.config.ts) and `npm run lint:layers` bans `vi.mock` for the same
 * reason: a language set on a module-level singleton by one test file is
 * inherited by every file that runs after it, in whatever order they happened
 * to run. A fresh instance handed down through `I18nextProvider` has no such
 * reach, which is why `LocaleProvider` takes one as an optional prop and the
 * test renderer passes its own.
 *
 * @param locale the language to start in. Omit to autodetect.
 */
export function createI18n(locale?: Locale): I18n {
  const instance = i18next.createInstance()

  void instance.use(initReactI18next).init({
    resources: {
      en: { translation: en },
      ru: { translation: ru },
    },
    lng: locale ?? detect(),
    // Per key, not per file: a Russian catalogue that has translated a button
    // and not the paragraph beside it shows the button in Russian and the
    // paragraph in English. That partial state is what a growing locale
    // actually looks like, so it is the case that has to work well -- and it
    // is the same rule the Go catalogue applies to SRD prose.
    fallbackLng: DEFAULT_LOCALE,
    supportedLngs: [...LOCALES],
    // The keys in web/locales/*.json are flat and dotted. Without these two,
    // i18next reads "auth.login.title" as a path into a nested object, finds
    // nothing, and silently renders the key -- which looks like a missing
    // translation rather than a misconfiguration.
    keySeparator: false,
    nsSeparator: false,
    interpolation: {
      // React escapes everything it renders already. Leaving i18next's own
      // escaping on double-encodes the apostrophes this app's copy is full of.
      escapeValue: false,
    },
    react: {
      // Nothing in this client renders a translation containing markup, and
      // leaving the parser on means every string is scanned for tags.
      transSupportBasicHtmlNodes: false,
    },
  })

  return instance
}
