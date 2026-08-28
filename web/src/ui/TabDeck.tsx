import { Carousel } from '@mantine/carousel'
import type { EmblaCarouselType } from 'embla-carousel'
import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'

import { swipeAllowedFrom } from './swipe'
import { TabRow } from './TabRow'
import { useIsDesktop } from './useIsDesktop'

export interface DeckPanel {
  /** The tab's value, and the panel's identity. */
  value: string
  /**
   * The tab's word, and the slide's accessible name -- one string in two
   * places rather than two that can drift apart.
   */
  label: string
  content: ReactNode
}

export interface TabDeckProps {
  /**
   * Names the carousel. Mantine gives the root `role="region"` and an
   * `aria-roledescription` of "carousel" already; a landmark called "region"
   * tells a screen reader nothing, and the name is the part only the call site
   * knows.
   */
  label: string
  panels: readonly DeckPanel[]
  /** Which panel is on screen. Controlled: the caller owns the tab. */
  value: string
  onChange: (value: string) => void
  /**
   * Whether the deck can be dragged from one panel to the next.
   *
   * The one reason to say no is a deck whose caller will refuse the move --
   * the build screen before its character exists, where changing tab means
   * creating one. A swipe that snaps back is worse than a swipe that never
   * starts.
   */
  swipeable?: boolean
}

/**
 * A row of tabs over a carousel of their panels: every one mounted, one on
 * screen, swiped or tapped between.
 *
 * `TabRow` is the strip, unchanged and for the reason it exists -- five tabs
 * do not fit across 390px, so it scrolls sideways and brings the active one
 * into view. What this adds is the other half of the gesture: on a phone the
 * panel is the biggest thing on screen and swiping it is the cheapest way to
 * move, where reaching back up to a 60px tab is the dearest.
 *
 * The two directions cannot fight. A tab press changes `value`, the effect
 * scrolls the carousel to it, and embla's `select` reports the index it
 * already holds -- `onChange` is only called when the panel that arrived is
 * not the one the caller asked for, and `scrollTo` an index embla is already
 * on does nothing. So neither side can drive the other in a loop.
 *
 * A slide that is off screen is still mounted, and things on it are still
 * focusable. Tabbing into one is embla's own `watchFocus`, which is on by
 * default: it scrolls the focused slide into view and emits `select`, so the
 * strip follows the focus rather than coming to disagree with what is on
 * screen. Nothing here has to do that itself, which is worth saying because
 * the obvious hand-rolled version is a focus handler that fights the one
 * already there.
 *
 * The carousel is deliberately given no `height`. Mantine's own default is
 * `auto`, which makes the viewport as tall as its tallest slide; a slide is
 * then aligned to the top of it rather than stretched down it, so a short
 * panel keeps its own size and leaves the difference blank. Sizing the
 * viewport to whichever slide is showing would need to measure it, and jsdom
 * computes no layout -- the suite could neither exercise that nor catch it
 * breaking.
 *
 * **A wide screen gets the tabs and nothing else.** The carousel is the answer
 * to a phone: the panel is the biggest thing on the screen, a swipe across it
 * is the cheapest gesture there is, and reaching back up to a 60px tab is the
 * dearest. None of that holds with a mouse -- there the tabs are a click away,
 * a drag is how you select text, and mounting five panels to show one is five
 * times the work for a gesture nobody makes. So above `md` this is `TabRow`
 * with the active panel in it, and the carousel is not built at all.
 *
 * That makes it the seventh component in this client whose two renderings
 * genuinely differ; see docs/web.md, where the list is kept.
 */
export function TabDeck({
  label,
  panels,
  value,
  onChange,
  swipeable = true,
}: TabDeckProps) {
  const isDesktop = useIsDesktop()
  const [embla, setEmbla] = useState<EmblaCarouselType | null>(null)
  const index = panels.findIndex((panel) => panel.value === value)
  // Whether the change on its way in came from somebody pressing a tab. The
  // effect below is the only reader; see what it does with it.
  const pressed = useRef(false)

  /*
   * In an effect rather than in the tab's own handler, because `value` is the
   * caller's: the build screen opens on the first category with something left
   * to answer, and a carousel that only followed presses would be left behind
   * by that.
   *
   * **A press scrolls; anything else jumps**, and the difference is what the
   * motion would be saying. Sliding from one tab to the next answers a press --
   * it shows which way you went. The same slide arriving unasked, a beat after
   * the page painted, is the page moving while you are reading it: a build
   * screen opens on the first unanswered category, which is rarely the first
   * slide, so every load began on the first one and slid sideways off it.
   *
   * A swipe needs neither, and gets neither: it is already on the slide it
   * selected, so the effect leaves it alone to settle.
   */
  useEffect(() => {
    if (isDesktop || embla === null || index < 0) return
    const press = pressed.current
    pressed.current = false
    // A swipe reaches here having already moved the carousel: embla selects the
    // slide the moment the gesture decides, and is still settling on to it. It
    // needs nothing from this, and touching it would cut its own animation
    // short -- which is what "the deck is too fast" looked like.
    if (embla.selectedScrollSnap() === index) return
    embla.scrollTo(index, !press)
  }, [isDesktop, embla, index])

  const tabs = panels.map((panel) => ({ value: panel.value, label: panel.label }))
  const press = (next: string) => {
    pressed.current = true
    onChange(next)
  }

  if (isDesktop) {
    return (
      <TabRow tabs={tabs} value={value} onChange={press}>
        {panels.find((panel) => panel.value === value)?.content}
      </TabRow>
    )
  }

  return (
    <TabRow tabs={tabs} value={value} onChange={press}>
      <Carousel
        aria-label={label}
        slideGap="md"
        // A phone swipes, and the tabs above are the route for everything else.
        // Two 26px arrows sitting over the panel they cover would be a third.
        withControls={false}
        // The tabs already say how many panels there are and name every one of
        // them. Dots under named tabs is the same fact twice, and the quieter
        // half of it.
        withIndicators={false}
        getEmblaApi={setEmbla}
        onSlideChange={(shownAt) => {
          const shown = panels[shownAt]
          if (shown !== undefined && shown.value !== value) onChange(shown.value)
        }}
        // Slides are stretched to the tallest one by default, which would draw
        // one short panel down the height of the longest.
        styles={{ container: { alignItems: 'flex-start' } }}
        // Not looped. These are ordered -- a sheet and a build both decide what
        // order things come in -- so wrapping from the last back to the first
        // is a jump rather than a continuation.
        // A predicate rather than `true`: see NO_SWIPE. `false` still means
        // a deck that does not drag at all, which is a different question.
        emblaOptions={{
          loop: false,
          watchDrag: swipeable ? (_, event) => swipeAllowedFrom(event.target) : false,
        }}
      >
        {panels.map((panel) => (
          // Named by `aria-label` rather than by `aria-labelledby`, because
          // there is no heading inside the slide to point at: the tab is where
          // the panel is named on screen. Both read the same `label`, so the
          // two cannot come to disagree.
          <Carousel.Slide key={panel.value} aria-label={panel.label}>
            {panel.content}
          </Carousel.Slide>
        ))}
      </Carousel>
    </TabRow>
  )
}
