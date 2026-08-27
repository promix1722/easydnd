import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { DESKTOP_MEDIA_QUERY } from '@/theme/tokens'

/**
 * What `app.css` still has to get right, checked as text rather than as styles.
 *
 * The file holds one rule -- a 16px font on a phone's fields, because iOS
 * Safari zooms below that -- and the thing that can go silently wrong is its
 * *boundary*. Written a pixel off the breakpoint `useIsDesktop` reads, nothing
 * would look broken and one width would quietly disagree with itself.
 *
 * Read as text on purpose, and not as a shortcut around a better test:
 * `vite.config.ts` runs the suite with CSS off, and jsdom evaluates no `@media`
 * at all, so **no test here can assert a rendered size**. Turning `css: true`
 * on to try would cost the `isolate: false` speed the whole suite is built
 * around.
 *
 * It is the same shape as `make data/srd/check`: not a test of behaviour, a
 * test that two artefacts which must agree still do.
 */

/**
 * Read with `node:fs`, and lazily.
 *
 * Not `import css from './app.css?raw'`, which is the obvious way and returns
 * an empty string here: the suite runs with CSS off, so Vite stubs every `.css`
 * specifier before `?raw` gets a chance -- and an empty string would make every
 * assertion below fail for a reason that has nothing to do with the files.
 *
 * Nor `new URL('./app.css', import.meta.url)`, which is the other obvious way
 * and throws: Vite reads that exact pattern as an *asset* reference and
 * rewrites it to a bundled URL, so what reaches `fileURLToPath` is not a
 * `file:` URL at all. Taking the directory apart by hand leaves the string
 * alone.
 */
function appCss(): string {
  const here = dirname(fileURLToPath(import.meta.url))
  const source = readFileSync(join(here, 'app.css'), 'utf8')
  // A path that resolved to nothing would otherwise pass the `not.toContain`
  // case and fail the others with a confusing message.
  expect(source.length, 'app.css read as empty').toBeGreaterThan(0)
  // Comments out, because that file explains itself at length and names every
  // variable it sets while doing so. Both assertions below caught the prose
  // rather than the rules on the first run -- the `!important` one tripped on
  // the sentence telling the next reader not to write one.
  return source.replace(/\/\*[\s\S]*?\*\//g, '')
}

describe('theme.ts and app.css', () => {
  it('turns on below the one breakpoint the rest of the app switches at', () => {
    // The rule is negated -- `not all and (min-width: ...)` -- because it
    // applies to phones, and Mantine's own `smaller-than` mixin would have
    // written 61.9375em instead: a pixel away from where `useIsDesktop`
    // changes its mind, close enough that nothing would look wrong and one
    // width would quietly disagree with itself. So the breakpoint itself is
    // what has to match, byte for byte.
    expect(appCss()).toContain(DESKTOP_MEDIA_QUERY)
  })

  it('never reaches for !important, because the cascade layer makes it unnecessary', () => {
    // `ui/AppTheme.tsx` imports Mantine's `styles.layer.css`, so every rule
    // here already outranks every Mantine rule. An `!important` appearing in
    // this file means somebody was fighting a fight they had already won, and
    // is the first sign the layered import was swapped back.
    expect(appCss()).not.toContain('!important')
  })
})
