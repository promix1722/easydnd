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
 * **The edges fade where there is more.** A strip that scrolls always rests
 * with a tab cut in half at one edge -- the last position on the sheet's own
 * strip leaves "Proficienc|ies" showing, because the seven tabs are 657px and
 * a viewport is 369, and no scroll offset makes both ends land on a boundary.
 * A hard cut reads as a broken word rather than as "there is more this way", so
 * the ends are masked, and only the ends that have something behind them. It is
 * measured rather than assumed, which is why it is state: the same strip on a
 * wide screen fits, and a fade over nothing is a smudge on the first tab.
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
   * How much of each end is covering a tab, which is how much of each end
   * fades.
   *
   * The fade is measured against the tab that is actually cut rather than set
   * to a constant, and that is the whole of why it works: a constant either
   * leaves a legible fragment ("Proficienc|ies", which is what a fixed 24 and
   * then a fixed 32 both left) or smears the tab beside it when the fragment
   * is narrow. An end resting exactly on a boundary has nothing cut, and gets
   * the minimum -- enough to say there is more that way, not enough to blur a
   * word that is whole.
   */
  const measure = useCallback(() => {
    const viewport = viewportRef.current
    if (viewport === null) return

    const from = viewport.scrollLeft
    const to = from + viewport.clientWidth
    // Read off the refs rather than off `tabs`, and not only to keep this
    // callback stable: what it wants is the tab lying across a given x, so the
    // order the tabs came in is not information it needs.
    const across = (at: number) =>
      [...tabRefs.current.values()]
        .map((element) => ({ start: element.offsetLeft, end: element.offsetLeft + element.offsetWidth }))
        .find((span) => span.start < at && span.end > at)

    setEdges((held) => {
      const cutStart = across(from)
      const cutEnd = across(to)
      const next = {
        start: from <= 1 ? null : hide(cutStart === undefined ? 0 : cutStart.end - from),
        end: to >= viewport.scrollWidth - 1 ? null : hide(cutEnd === undefined ? 0 : to - cutEnd.start),
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

/**
 * How much of each end is hidden outright, or `null` for an end with nothing
 * behind it.
 *
 * Zero is not the same as `null`: an end resting exactly on a tab boundary
 * hides nothing and still gets the ramp, because an edge drawn hard says the
 * strip ends there.
 */
interface Edges {
  start: number | null
  end: number | null
}

const NO_EDGES: Edges = { start: null, end: null }

/**
 * The cut tab is hidden, and the ramp happens after it.
 *
 * A plain gradient across the fragment does not work, and the two constants it
 * was tried with are why this is spelled out: at 24px and again at 32px, the
 * far half of "Proficienc|ies" sat at 80-90% opacity and read as a word. What
 * hides a fragment is transparency for its whole width, and a short ramp
 * beginning where the next -- whole -- tab does.
 */
const RAMP = 16

/**
 * The most of an end that is ever hidden.
 *
 * At rest nothing near it is reached: the strip lands on a tab's left edge, so
 * the only cut is the one the browser's scroll clamp leaves at the far end, and
 * on the sheet that is 36px. It is a mid-drag guard -- a finger can stop with
 * 150px of a 171px tab across the edge, and blanking that much of a 369px
 * viewport would be a strip that is mostly nothing.
 */
const MAX_HIDDEN = 96

function hide(cut: number): number {
  return Math.min(Math.max(Math.round(cut), 0), MAX_HIDDEN)
}

/** The mask for a strip with those ends, or no mask at all for a strip with neither. */
function maskFor({ start, end }: Edges): CSSProperties {
  if (start === null && end === null) return {}
  const from =
    start === null ? '#000 0' : `transparent 0, transparent ${start}px, #000 ${start + RAMP}px`
  const to =
    end === null
      ? '#000 100%'
      : `#000 calc(100% - ${end + RAMP}px), transparent calc(100% - ${end}px), transparent 100%`
  const mask = `linear-gradient(to right, ${from}, ${to})`
  // Both spellings: the unprefixed property is what every current engine reads,
  // and PostCSS is configured for Mantine's mixins rather than autoprefixing.
  return { maskImage: mask, WebkitMaskImage: mask }
}
