import type { PointerEvent as ReactPointerEvent } from 'react'

/**
 * The three bits of arithmetic and DOM lookup a pointer drag needs.
 *
 * Their own module for two reasons. `ScoreAssignment.tsx` exports a component
 * and nothing else, which is what keeps fast refresh working -- and these are
 * the parts of the gesture a test can actually reach: jsdom has no layout, so
 * `document.elementFromPoint` is missing there and the drag itself has to be
 * driven with a stub. See `AbilityScoresForm.test.tsx`.
 */

/** How far a press has to travel before it is a drag rather than a tap. */
export const DRAG_THRESHOLD = 8

/** The attribute that makes an ability findable under a finger. */
export const TARGET = 'data-ability'

/** Where a pointer is, in the coordinates `elementFromPoint` reads. */
export interface Point {
  x: number
  y: number
}

export interface Drag {
  /** The place in the pool being moved. */
  place: number
  from: Point
  at: Point
  /** The ability under the pointer, once this is a drag at all. */
  over: string | null
  moved: boolean
}

export function point(event: ReactPointerEvent<HTMLElement>): Point {
  return { x: event.clientX, y: event.clientY }
}

/** How far the pointer has come. */
export function travelled(from: Point, to: Point): number {
  return Math.hypot(to.x - from.x, to.y - from.y)
}

/**
 * The ability under the pointer, if any.
 *
 * `elementFromPoint` is missing in jsdom, which is why it is called through an
 * optional call rather than assumed: the suite presses this surface with real
 * clicks, and a drag it cannot express should not take the tests down with it.
 */
export function abilityUnder(at: Point): string | null {
  const under = document.elementFromPoint?.(at.x, at.y)
  return under?.closest(`[${TARGET}]`)?.getAttribute(TARGET) ?? null
}

/** What `ScoreAssignment` spreads onto anything a number can be dragged from. */
export interface DragHandlers {
  onPointerDown?: (event: ReactPointerEvent<HTMLElement>) => void
  onPointerMove?: (event: ReactPointerEvent<HTMLElement>) => void
  onPointerUp?: () => void
  onPointerCancel?: () => void
}
