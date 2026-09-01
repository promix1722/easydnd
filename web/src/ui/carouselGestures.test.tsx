import { useEffect } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from '@testing-library/react'

import { useCarouselGestures } from './carouselGestures'

/**
 * The hook drives an Embla instance and reads the document's scroll extent, and
 * neither is something jsdom supplies: there is no carousel here and every
 * element reports a zero-sized layout. So the instance is a pair of spies and
 * the extent is stubbed per test, which is what makes the *decisions* testable
 * -- which way it moved, and whether it moved at all -- without a real engine.
 *
 * `rootNode` returns a genuine element in the document, because two of the
 * branches are about where the event came from and where the focus is.
 */
function fakeEmbla(root: HTMLElement) {
  return {
    rootNode: () => root,
    scrollNext: vi.fn(),
    scrollPrev: vi.fn(),
  }
}

/**
 * Nodes this file put in the document by hand, to be taken out again.
 *
 * The suite runs with `isolate: false`, so `document.body` is shared with every
 * file that runs after this one -- and `render`'s own cleanup only removes what
 * React mounted, not a `<div>` appended here. A leftover `<button>` is a
 * `queryByRole('button')` somewhere else finding a control its own screen never
 * drew, which is exactly the failure `src/test/setup.ts` exists to prevent.
 */
const planted: Element[] = []

function plant<T extends Element>(node: T): T {
  document.body.append(node)
  planted.push(node)
  return node
}

/** Renders a component that hands the hook the fake, and returns the spies. */
function mount() {
  const root = document.createElement('div')
  root.innerHTML = '<button type="button">inside</button>'
  plant(root)
  const embla = fakeEmbla(root)

  function Harness() {
    const { getEmblaApi } = useCarouselGestures()
    // In an effect, which is when the real Carousel hands its engine over --
    // calling it during render is a render-phase setState that loops.
    useEffect(() => {
      getEmblaApi(embla as unknown as Parameters<typeof getEmblaApi>[0])
    }, [getEmblaApi])
    return null
  }

  render(<Harness />)
  return { embla, root }
}

/**
 * Pretends the page does or does not have somewhere to scroll.
 *
 * jsdom reports 0 for every extent, so without this the guard always reads
 * "the page fits" and the branch that hands the wheel back is never taken.
 */
function pageExtent({ scrollHeight = 0, scrollWidth = 0 }) {
  const doc = document.documentElement
  const spies = [
    vi.spyOn(doc, 'scrollHeight', 'get').mockReturnValue(scrollHeight),
    vi.spyOn(doc, 'scrollWidth', 'get').mockReturnValue(scrollWidth),
    vi.spyOn(doc, 'clientHeight', 'get').mockReturnValue(0),
    vi.spyOn(doc, 'clientWidth', 'get').mockReturnValue(0),
  ]
  return () => spies.forEach((spy) => spy.mockRestore())
}

let restore: (() => void) | null = null
afterEach(() => {
  restore?.()
  restore = null
  planted.forEach((node) => node.remove())
  planted.length = 0
})

function wheel(root: HTMLElement, init: WheelEventInit) {
  root.dispatchEvent(new WheelEvent('wheel', { bubbles: true, cancelable: true, ...init }))
}

describe('useCarouselGestures', () => {
  it('moves forward on a wheel down and back on a wheel up', () => {
    restore = pageExtent({})
    const { embla, root } = mount()

    wheel(root, { deltaY: 120 })
    expect(embla.scrollNext).toHaveBeenCalledTimes(1)
    expect(embla.scrollPrev).not.toHaveBeenCalled()

    // A second mount, because the first gesture holds the cooldown open --
    // which is the next test's subject, not this one's.
    const second = mount()
    wheel(second.root, { deltaY: -120 })
    expect(second.embla.scrollPrev).toHaveBeenCalledTimes(1)
  })

  // The tail of a flick is dozens of events. Without this the carousel crosses
  // every slide on one push, which is the whole reason the cooldown exists.
  it('reads one gesture as one slide', () => {
    restore = pageExtent({})
    const { embla, root } = mount()

    for (let i = 0; i < 8; i++) wheel(root, { deltaY: 120 })

    expect(embla.scrollNext).toHaveBeenCalledTimes(1)
  })

  // The guard that keeps this from being a page you cannot scroll.
  it('gives the wheel back when the page has somewhere to scroll', () => {
    restore = pageExtent({ scrollHeight: 2000 })
    const { embla, root } = mount()

    wheel(root, { deltaY: 120 })

    expect(embla.scrollNext).not.toHaveBeenCalled()
  })

  // A sideways swipe is judged on the horizontal extent, not the vertical one:
  // a tall page still owes the carousel its sideways gestures.
  it('takes a sideways swipe on a page that only scrolls down', () => {
    restore = pageExtent({ scrollHeight: 2000 })
    const { embla, root } = mount()

    wheel(root, { deltaX: 120, deltaY: 0 })

    expect(embla.scrollNext).toHaveBeenCalledTimes(1)
  })

  it('answers the arrow keys from anywhere on the page', () => {
    restore = pageExtent({})
    const { embla } = mount()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    expect(embla.scrollNext).toHaveBeenCalledTimes(1)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }))
    expect(embla.scrollPrev).toHaveBeenCalledTimes(1)
  })

  // Mantine's indicators answer arrows themselves with a roving tabindex, so
  // handling them again while focus is in there moves two slides per press.
  it('leaves the arrow keys alone while focus is inside the carousel', () => {
    restore = pageExtent({})
    const { embla, root } = mount()
    root.querySelector('button')?.focus()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))

    expect(embla.scrollNext).not.toHaveBeenCalled()
  })

  it('leaves the arrow keys alone in a field being typed in', () => {
    restore = pageExtent({})
    const { embla } = mount()
    const field = plant(document.createElement('input'))
    field.focus()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))

    expect(embla.scrollNext).not.toHaveBeenCalled()
  })
})
