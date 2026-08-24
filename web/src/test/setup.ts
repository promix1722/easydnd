import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

import { resetViewport } from './viewport'

/**
 * jsdom has no layout engine, so it has no ResizeObserver either -- and
 * Mantine's ScrollArea constructs one on mount. Without this, every component
 * built on `ui/TabRow` throws before it renders a single tab, for a reason
 * that has nothing to do with what is being tested. Anything that opens a
 * `Select` fails the same way, from deep inside React's commit phase.
 *
 * It observes nothing and reports nothing, deliberately. A stub that invented
 * sizes would let a test assert a layout jsdom never computed.
 */
class NoResizeObserver implements ResizeObserver {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

globalThis.ResizeObserver ??= NoResizeObserver

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
