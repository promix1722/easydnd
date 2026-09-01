import { ScrollArea, Tabs } from '@mantine/core'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { CSSProperties, ReactNode } from 'react'

export interface TabRowTab {
  value: string
  label: string
}

export interface TabRowProps {
  tabs: readonly TabRowTab[]
  value: string
  onChange: (value: string) => void
  /** The active tab's contents. */
  children?: ReactNode
}

/**
 * A row of tabs that scrolls sideways when it does not fit.
 *
 * A primitive rather than a call-site arrangement because six tabs of a
 * character sheet -- "Traits and features", "Resources and gear" -- need 657px
 * and a phone has 369 of them. Mantine's `Tabs.List` offers `grow` and
 * `justify`; neither is a strip that scrolls.
 *
 * There used to be an **actions** slot against the end of the strip, and the
 * build screen's Finish button was its only caller. Both moved: Finish acts on
 * the character rather than on the tabs, so it belongs in `ui/Page`'s `actions`
 * -- where the sheet's own button already was -- and a button competing with
 * the strip for a 390px line is a strip that cannot do the one thing it is for.
 * The slot went with the caller rather than being kept as a hole nothing fills,
 * the way `ColumnsSection`'s `aside` did; see docs/web.md.
 *
 * It is a responsive primitive whose **two renderings are the same markup**.
 * `ModalSheet`, `Columns`, `SectionDeck` and `TabDeck` genuinely swap components at the
 * breakpoint; this one does not need to. A `ScrollArea type="never"` is inert
 * at a width the content fits in, so the desktop rendering is the mobile one
 * with nothing to scroll -- which means there is no second tree to keep
 * working, and a test at one width is a real test of the other.
 *
 * The active tab is brought into view by setting `scrollLeft` rather than by
 * `scrollIntoView`. The latter scrolls every scrollable ancestor, so selecting
 * a tab would drag the document as well as the strip -- and jsdom does not
 * implement it, which would make the behaviour untestable on top of wrong.
 *
 * **The edges fade where there is more, and the tab under the fade stays
 * visible.** This is the second answer to the question, and the first one was
 * backwards. It used to hide the cut tab *outright*, for its whole measured
 * width, on the argument that a legible fragment reads as a broken word rather
 * than as "there is more this way". What that missed is where the strip
 * actually rests: on a tab's own left edge, so the tab straddling the far edge
 * is cut by however much of it happens to fit -- 56px of "Traits and features"
 * on the sheet -- and hiding the cut hid *the entire next tab*. The strip then
 * ended in clean space after `Proficiencies` and read as four tabs, which is
 * exactly what it was reported as. The fragment was never the problem; it was
 * the only evidence there was more.
 *
 * So the fragment is kept and the fade is a constant. A tab dissolving into the
 * edge is the ordinary signal every scrolling strip uses, and it cannot be read
 * as a typo because it is visibly not finished. `FADE` is long enough that no
 * part of it sits at the 80-90% opacity that made a short ramp read as a whole
 * word.
 *
 * Only the ends that have something behind them fade, which is why it is still
 * state: the same strip on a wide screen fits, and a fade over nothing is a
 * smudge on the first tab. But what it measures now is one boolean per end --
 * whether the scroller has anything left that way -- rather than the geometry
 * of whichever tab lies across the edge. `across`, the per-tab spans and the
 * mid-drag cap they needed are gone with the rule that wanted them.
 *
 * That measurement is the one thing here the suite cannot press. jsdom computes
 * no layout, so `scrollWidth` and `clientWidth` are both 0 and the mask is
 * always absent -- the same bargain `SectionDeck.test.tsx` records about which
 * slide is showing. What the tests do hold is that the absence is identical at
 * both viewports.
 */
export function TabRow({ tabs, value, onChange, children }: TabRowProps) {
  const viewportRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef(new Map<string, HTMLButtonElement>())
  const [edges, setEdges] = useState<Edges>(NO_EDGES)

  /**
   * Which ends have something behind them, which is which ends fade.
   *
   * Two booleans off the scroller itself. The 1px slack on each comparison is
   * for fractional scroll offsets: a strip scrolled to its end can report a
   * `scrollLeft` a rounding error short of the arithmetic, and a fade that
   * never quite switches off is a permanent smudge on the last tab.
   */
  const measure = useCallback(() => {
    const viewport = viewportRef.current
    if (viewport === null) return

    setEdges((held) => {
      const next = {
        start: viewport.scrollLeft > 1,
        end: viewport.scrollLeft + viewport.clientWidth < viewport.scrollWidth - 1,
      }
      // The same object unless something changed: this runs on every frame of
      // a scroll, and a fresh object each time is a re-render each time.
      return held.start === next.start && held.end === next.end ? held : next
    })
  }, [])

  useEffect(() => {
    const viewport = viewportRef.current
    const tab = tabRefs.current.get(value)
    if (viewport === null || tab === undefined) return

    /*
     * The strip comes to rest on a tab's own left edge, in both directions.
     *
     * Scrolling forward used to stop as soon as the active tab's *right* edge
     * cleared the viewport, which is the least it could do and leaves whatever
     * tab happens to straddle the left edge cut in half -- "Proficienc|ies",
     * unreadable, and read by everybody as a bug rather than as a scroll
     * position. Landing on the tab's own left edge cannot: a boundary is where
     * a tab begins.
     *
     * The one place it still cannot win is the far end of a strip whose tabs do
     * not divide into the viewport -- the browser clamps there, and the clamp
     * is wherever it is. That is what the fade below is for.
     */
    const left = tab.offsetLeft
    const right = left + tab.offsetWidth
    if (left < viewport.scrollLeft || right > viewport.scrollLeft + viewport.clientWidth) {
      viewport.scrollLeft = left
    }
    measure()
    // `tabs.length` because a strip that gains or loses a tab is a different
    // strip: what overflows changes without `value` changing. Deliberately not
    // `tabs` itself, which is a fresh array on every render of every caller --
    // this would then run on every render and yank a finger's scroll back.
  }, [value, tabs.length, measure])

  return (
    <Tabs
      value={value}
      onChange={(next) => {
        if (next !== null) onChange(next)
      }}
    >
      <ScrollArea
        type="never"
        viewportRef={viewportRef}
        onScrollPositionChange={measure}
        style={{ minWidth: 0, ...maskFor(edges) }}
      >
        {/*
          `max-content`, because the rule under the tabs is the list's own and a
          list is otherwise as wide as the box it is in. Inside a scroller that
          is the *viewport* -- 369px against 657px of tabs -- so the tabs
          overflowed their own list and the underline stopped a third of the way
          along, which from a scrolled position reads as a stray dash beside the
          first tab you can see.
        */}
        <Tabs.List style={{ flexWrap: 'nowrap', width: 'max-content' }}>
          {tabs.map((tab) => (
            <Tabs.Tab
              key={tab.value}
              value={tab.value}
              ref={(element: HTMLButtonElement | null) => {
                if (element === null) tabRefs.current.delete(tab.value)
                else tabRefs.current.set(tab.value, element)
              }}
            >
              {tab.label}
            </Tabs.Tab>
          ))}
        </Tabs.List>
      </ScrollArea>

      <Tabs.Panel value={value} pt="md">
        {children}
      </Tabs.Panel>
    </Tabs>
  )
}

/** Whether each end has strip behind it, and so whether each end fades. */
interface Edges {
  start: boolean
  end: boolean
}

const NO_EDGES: Edges = { start: false, end: false }

/**
 * How far the strip dissolves at an end that has more behind it.
 *
 * Long enough that the fragment under it is visibly *going*, rather than a word
 * with its last letters greyed: a short ramp was tried at 24px and again at
 * 32px, and the far half of the fragment sat at 80-90% opacity and read as
 * finished text. Short enough that the fragment is still a fragment of
 * something -- the point of keeping it is that a reader can see a tab there.
 */
const FADE = 44

/** The mask for a strip with those ends, or no mask at all for a strip with neither. */
function maskFor({ start, end }: Edges): CSSProperties {
  if (!start && !end) return {}
  const from = start ? `transparent 0, #000 ${FADE}px` : '#000 0'
  const to = end ? `#000 calc(100% - ${FADE}px), transparent 100%` : '#000 100%'
  const mask = `linear-gradient(to right, ${from}, ${to})`
  // Both spellings: the unprefixed property is what every current engine reads,
  // and PostCSS is configured for Mantine's mixins rather than autoprefixing.
  return { maskImage: mask, WebkitMaskImage: mask }
}
