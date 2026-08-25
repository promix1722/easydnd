import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

import { resetViewport } from './viewport'

/**
 * jsdom has no layout engine, so it ships neither observer the UI depends on --
 * and several different things here want them. Mantine's ScrollArea constructs
 * a ResizeObserver on mount, so every component built on `ui/TabRow` throws
 * before it renders a single tab, and anything that opens a `Select` fails the
 * same way from deep inside React's commit phase; embla, which drives
 * `ui/Carousel`, constructs both observers the moment a carousel mounts, so
 * every test that renders the landing page dies on a missing global. None of
 * those failures has anything to do with what is being tested.
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

afterEach(() => {
  cleanup()
  resetViewport()
})
