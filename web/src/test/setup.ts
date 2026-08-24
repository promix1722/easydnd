import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

import { resetViewport } from './viewport'

/**
 * jsdom has no layout engine, so it has no ResizeObserver either -- and
 * Mantine's ScrollArea constructs one on mount. Without this, every component
 * built on `ui/TabRow` throws before it renders a single tab, for a reason
 * that has nothing to do with what is being tested.
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

afterEach(() => {
  cleanup()
  resetViewport()
})
