import '@testing-library/jest-dom/vitest'
import { afterEach, vi } from 'vitest'
import { cleanup } from '@testing-library/react'

import { configure } from '@testing-library/react'

import { resetCatalogCache } from '@/lib/api'
import { resetRequestLocale } from '@/lib/api/locale'
import { resetViewport } from './viewport'

/**
 * Five seconds rather than the default one.
 *
 * This buys nothing on a passing run and is not a speed setting: waitFor runs
 * its callback once, synchronously, before it ever waits (wait-for.js:84), so
 * an assertion that is going to pass resolves on that first check whatever the
 * timeout says. What the number decides is how long a *starved* test waits
 * before giving up -- and a one-second budget is short enough that a loaded CI
 * runner fails tests that have nothing wrong with them. That is the shape of
 * the intermittent `web checks` failure that has blocked tag deploys: under CPU
 * pressure the first to go were BuildScreen and AbilityScoresForm, the two
 * files that drive the most interactions.
 *
 * The cost is paid only by genuine failures, which now take up to five seconds
 * to report instead of one.
 */
configure({ asyncUtilTimeout: 5000 })

/**
 * jsdom has no layout engine, so it ships neither observer the UI depends on --
 * and several different things here want them. Mantine's ScrollArea constructs
 * a ResizeObserver on mount, so every component built on `ui/TabRow` throws
 * before it renders a single tab, and anything that opens a `Select` fails the
 * same way from deep inside React's commit phase; embla, which drives every
 * carousel here, constructs both observers the moment one mounts, so any test
 * that renders the landing page or a character sheet dies on a missing global.
 * None of those failures has anything to do with what is being tested.
 *
 * They observe nothing and report nothing, deliberately. A stub that invented
 * sizes would let a test assert a layout jsdom never computed -- so a carousel
 * is asserted on its markup, the slides being present and named and in order,
 * and never on which one is scrolled into view.
 *
 * `??=` rather than a bare assignment, so a future jsdom that ships either one
 * for real wins over the stub.
 */
class NoResizeObserver implements ResizeObserver {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

class NoIntersectionObserver implements IntersectionObserver {
  readonly root = null
  readonly rootMargin = ''
  readonly scrollMargin = ''
  readonly thresholds: readonly number[] = []
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
  takeRecords(): IntersectionObserverEntry[] {
    return []
  }
}

globalThis.ResizeObserver ??= NoResizeObserver
globalThis.IntersectionObserver ??= NoIntersectionObserver

/**
 * The same gap, one layer up: Mantine's Select scrolls the highlighted option
 * into view as the dropdown opens, and jsdom has no such method to call.
 *
 * Making those dropdowns *visible* is a separate matter and is not here --
 * src/test/render.tsx renders the provider in Mantine's own test environment,
 * because Mantine hides a popover whose anchor it cannot see and in jsdom no
 * anchor ever is.
 */
Element.prototype.scrollIntoView ??= function scrollIntoView(): void {}

/**
 * Everything one test file can leave behind for the next one.
 *
 * This matters more than it looks, because the suite does not isolate test
 * files from each other -- see the `test` block in vite.config.ts. One module
 * registry and one jsdom are shared by every file a worker runs, which is most
 * of why the suite is fast and all of why this hook matters: it is the only
 * thing standing between that and a test passing because of what ran before it.
 *
 * The list is short because there is very little module-level mutable state in
 * src/: the catalogue's in-flight request cache, the viewport width, and which
 * language the API client is asking for. Add to it when you add to those.
 *
 * The language is the newest entry and the easiest to overlook, because it has
 * two homes. `src/lib/api/locale.ts` holds the one `request()` reads, and it is
 * reset here. The *displayed* language is not module state at all -- it lives
 * on an i18next instance that `src/test/render.tsx` creates per render and
 * pins to English -- which is deliberate, and is why a test that switches
 * language cannot leak one into the next file.
 */
afterEach(() => {
  cleanup()
  resetViewport()
  // Fifteen test files stub a global; fourteen of them unstub it. Doing it
  // here means a file that forgets cannot reach the next one.
  vi.unstubAllGlobals()
  resetCatalogCache()
  resetRequestLocale()
  // The language detector caches a choice here. A test that switches language
  // would otherwise hand it to the next file that autodetects.
  try {
    window.sessionStorage.clear()
  } catch {
    // A jsdom without storage is still a jsdom worth running tests in.
  }
})
