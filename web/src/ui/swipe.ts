/**
 * Which gestures a carousel may take for itself.
 *
 * Its own file rather than a pair of exports from `TabDeck.tsx`, because a
 * module that exports anything but components loses fast refresh -- and because
 * the rule is about the deck and the surfaces inside it equally, so neither of
 * them is its natural home.
 */

/**
 * The attribute a panel puts on a surface a swipe must not start from.
 *
 * A carousel and a tap target want the same finger. Embla takes the pointer
 * down on every element that is not an input, and from a few pixels of
 * sideways drift it both scrolls the deck and swallows the `click` that was
 * about to land -- so on a surface where the whole interaction is "tap this,
 * then tap where it goes" (`features/character/ScoreAssignment`), a press that
 * drifts does not miss, it takes the panel off screen. Marking the surface
 * hands the gesture back to it: the deck is still swiped from anywhere else on
 * the slide, and the tabs above never stopped working.
 */
export const NO_SWIPE = 'data-no-swipe'

/**
 * Whether a gesture starting here should drag the deck.
 *
 * Exported for its test: embla owns the event in the browser, and jsdom draws
 * no carousel to press.
 */
export function swipeAllowedFrom(target: EventTarget | null): boolean {
  return !(target instanceof Element) || target.closest(`[${NO_SWIPE}]`) === null
}
