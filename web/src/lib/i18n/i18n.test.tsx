import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'

import i18next from 'i18next'

import { renderAt } from '@/test/render'

import { createI18n } from './instance'
import { localeOf } from './locales'
import { useLocale, useSetLocale } from './useLocale'
import { useT } from './useT'

import { Button, Text } from '@/ui'

/**
 * What this file pins is the *mechanism*, not the copy.
 *
 * Every other test in the suite renders in English and asserts on the words it
 * expects, which is what `src/test/render.tsx` pins the locale for. These are
 * the ones that would be silently wrong if the machinery underneath them
 * stopped working: a plural that picks the wrong form, a fallback that shows a
 * bare key instead of English, a detector that reads the wrong thing.
 */

describe('plural selection', () => {
  const en = createI18n('en')
  const ru = createI18n('ru')

  // English has two forms and Russian four. i18next picks between them with
  // Intl.PluralRules, so the catalogue only has to *have* the forms -- but a
  // key missing `_few`/`_many` in Russian is a silent fallback to English, and
  // that is the mistake worth catching.
  it('picks the English form by count', () => {
    expect(en.t('choice.language', { count: 1 })).toBe('1 more language')
    expect(en.t('choice.language', { count: 2 })).toBe('2 more languages')
    expect(en.t('choice.language', { count: 5 })).toBe('5 more languages')
  })

  it('offers Russian all four categories', () => {
    // Not an assertion about translated copy -- `choice.language` has no
    // Russian yet. It is an assertion that the *rules* for Russian are loaded,
    // which is what decides whether a translator's `_few` will ever be read.
    const rules = new Intl.PluralRules('ru')
    expect(rules.select(1)).toBe('one')
    expect(rules.select(2)).toBe('few')
    expect(rules.select(5)).toBe('many')
    expect(ru.language).toBe('ru')
  })
})

describe('interpolation', () => {
  const en = createI18n('en')

  it('substitutes named values', () => {
    expect(en.t('folders.newCharacterIn', { name: 'Retired' })).toBe('New character in Retired')
  })

  // React escapes what it renders, so i18next escaping as well would double-
  // encode the apostrophes this app's copy is full of.
  it('does not escape the value it was given', () => {
    expect(en.t('groups.deleteWarning', { name: "Ada's table" })).toContain("Ada's table")
  })
})

describe('fallback', () => {
  it('answers in Russian where there is a translation', () => {
    expect(createI18n('ru').t('section.characters')).toBe('Персонажи')
  })

  /**
   * The whole design rests on this: a partial locale is the normal state of a
   * growing one, and the alternative to falling back is showing a bare key.
   *
   * Against a fixture rather than the shipped catalogues, and deliberately.
   * The first version of this test asserted that a real key had no Russian --
   * which passed only while `ru.json` was incomplete and broke the day it was
   * finished. The property being guarded is the *merge*, not how much of it
   * has been translated, so the fixture is what makes the test honest.
   */
  it('falls back to English per key, not per file', async () => {
    const partial = i18next.createInstance()
    await partial.init({
      lng: 'ru',
      fallbackLng: 'en',
      keySeparator: false,
      nsSeparator: false,
      resources: {
        en: { translation: { translated: 'English one', untranslated: 'English two' } },
        ru: { translation: { translated: 'Русский один' } },
      },
    })

    expect(partial.t('translated')).toBe('Русский один')
    // Not the key, and not empty: the English word, in the middle of an
    // otherwise Russian screen.
    expect(partial.t('untranslated')).toBe('English two')
  })
})

describe('locale tags', () => {
  it('reads a regional tag as its language', () => {
    // Matches acceptedLanguages in internal/api/http/helpers/locale.go, so
    // both halves of the app answer "ru-RU" the same way.
    expect(localeOf('ru-RU')).toBe('ru')
    expect(localeOf('en-GB')).toBe('en')
  })

  it('is null for a language this client has no catalogue for', () => {
    expect(localeOf('fr')).toBeNull()
    expect(localeOf('')).toBeNull()
  })
})

/** A screen small enough to read, in whatever language it is handed. */
function Switcher() {
  const t = useT()
  const locale = useLocale()
  const setLocale = useSetLocale()

  return (
    <>
      <Text>{t('section.characters')}</Text>
      <Text data-testid="locale">{locale}</Text>
      <Button onClick={() => setLocale('ru')}>{t('page.retry')}</Button>
    </>
  )
}

describe('the provider', () => {
  it('renders in English by default, which every other test depends on', () => {
    renderAt('desktop', <Switcher />)

    expect(screen.getByText('Characters')).toBeInTheDocument()
    expect(screen.getByTestId('locale')).toHaveTextContent('en')
  })

  it('renders in Russian when asked for it', () => {
    renderAt('desktop', <Switcher />, 'ru')

    expect(screen.getByText('Персонажи')).toBeInTheDocument()
    expect(screen.getByTestId('locale')).toHaveTextContent('ru')
  })

  // The <html lang> is not something React renders, so the provider is the
  // only place it can be said -- and a screen reader picking the wrong voice
  // for a page is exactly the failure nobody notices in a test suite.
  it('tells the document which language it is in', () => {
    renderAt('desktop', <Switcher />, 'ru')

    expect(document.documentElement.lang).toBe('ru')
  })
})

describe('choosing a language on first load', () => {
  /** What the browser says it wants, for the length of one test. */
  function browserSpeaks(tag: string) {
    vi.stubGlobal('navigator', { ...window.navigator, language: tag, languages: [tag] })
  }

  it('autodetects from the browser', () => {
    browserSpeaks('ru-RU')

    expect(createI18n().language).toBe('ru')
  })

  it('falls back to English for a language it has no catalogue for', () => {
    browserSpeaks('fr-FR')

    expect(createI18n().language).toBe('en')
  })

  it('prefers what this visit chose over what the browser asks for', () => {
    browserSpeaks('en-GB')
    window.sessionStorage.setItem('easydnd.locale', 'ru')

    expect(createI18n().language).toBe('ru')
  })

  // A private-mode browser refuses storage outright. The choice is lost, which
  // is a shame; failing to render is not an option.
  it('still starts when storage is refused', () => {
    browserSpeaks('en')
    const denied = () => {
      throw new DOMException('denied', 'SecurityError')
    }
    vi.stubGlobal('sessionStorage', { getItem: denied, setItem: denied, removeItem: denied })

    expect(() => createI18n()).not.toThrow()
  })
})
