import { useEffect, useState } from 'react'

import type { EmblaCarouselType } from 'embla-carousel'

/**
 * How long one wheel gesture owns the carousel, in milliseconds.
 *
 * A wheel is not one event. A trackpad flick and a momentum-scrolling mouse
 * both fire dozens in a row, so a handler that moved a slide per event would
 * cross the whole carousel on one push. This is the window in which the rest of
 * a gesture is read and thrown away -- long enough to cover the momentum tail,
 * short enough that a deliberate second push lands.
 */
const GESTURE_MS = 400

/** Where an arrow key means "move the caret", not "move the carousel". */
const TYPING = new Set(['INPUT', 'TEXTAREA', 'SELECT'])

/**
 * Arrow keys and the wheel, for a carousel that fills the page it is on.
 *
 * Mantine gives a carousel a swipe and a pair of arrow buttons, and its
 * indicators answer the arrow keys once one of them has focus. What none of
 * that covers is the visitor who has neither touched the page nor tabbed into
 * it: on a laptop, the two obvious ways to move a full-page carousel are the
 * arrow keys and the wheel, and both did nothing.
 *
 * **The wheel is only ever borrowed.** It is taken on the axis the gesture is
 * actually on, and only when the page has nowhere to scroll on that axis -- so
 * a sideways trackpad swipe moves the carousel when nothing scrolls
 * horizontally, and a plain mouse wheel moves it when the page already fits.
 * The moment the page has scrolling of its own to do -- a short landscape phone
 * where the carousel hits its 320px floor and the page grows past the viewport
 * -- the wheel goes straight back to the page. A carousel that ate the wheel
 * unconditionally would be a page you cannot scroll.
 *
 * **The keys are only borrowed too.** Focus inside the carousel is left alone,
 * because Mantine's indicators are a roving tabindex that already answers
 * arrows there and handling them twice would move two slides at once; and a
 * field being typed in keeps its own arrows, which is what `TYPING` is for.
 *
 * Bound to the window rather than the carousel, because "the carousel is what
 * this page is" is the case it exists for -- requiring focus first would be the
 * behaviour that is already missing. The effect runs only once there is a
 * carousel to drive, so a page without one binds nothing.
 *
 * Returned as props to spread rather than taking an instance, so a caller never
 * has to name `EmblaCarouselType`: only `ui/` may import the engine, and
 * `routes/LandingPage.tsx` is where this is used.
 */
export function useCarouselGestures(): {
  getEmblaApi: (embla: EmblaCarouselType) => void
} {
  const [embla, setEmbla] = useState<EmblaCarouselType | null>(null)

  useEffect(() => {
    if (embla === null) return
    const root = embla.rootNode()
    // Not state: nothing renders from it, and a re-render per wheel event is
    // the one thing a gesture handler must not cost.
    let settled = 0

    const onWheel = (event: WheelEvent) => {
      const doc = document.documentElement
      const sideways = Math.abs(event.deltaX) > Math.abs(event.deltaY)
      // The +1 is for fractional layout: a page that fits can still report a
      // scroll height a rounding error taller than its client height.
      const pageScrolls = sideways
        ? doc.scrollWidth > doc.clientWidth + 1
        : doc.scrollHeight > doc.clientHeight + 1
      if (pageScrolls) return

      const delta = sideways ? event.deltaX : event.deltaY
      if (delta === 0) return

      // Before the cooldown, not after it: the tail of a gesture we already
      // answered still belongs to us, and letting it through would be a
      // sideways swipe triggering the browser's back-navigation.
      event.preventDefault()
      if (event.timeStamp - settled < GESTURE_MS) return
      settled = event.timeStamp

      if (delta > 0) embla.scrollNext()
      else embla.scrollPrev()
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
      const active = document.activeElement
      if (active !== null && root.contains(active)) return
      if (active instanceof HTMLElement && (active.isContentEditable || TYPING.has(active.tagName)))
        return

      event.preventDefault()
      if (event.key === 'ArrowRight') embla.scrollNext()
      else embla.scrollPrev()
    }

    // `passive: false` because the whole point is to be able to preventDefault;
    // a wheel listener is passive by default on a root target.
    root.addEventListener('wheel', onWheel, { passive: false })
    window.addEventListener('keydown', onKeyDown)
    return () => {
      root.removeEventListener('wheel', onWheel)
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [embla])

  return { getEmblaApi: setEmbla }
}
