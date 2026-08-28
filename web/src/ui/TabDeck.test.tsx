import { describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'

import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'
import { setupUser } from '@/test/user'

import { NO_SWIPE, swipeAllowedFrom } from './swipe'
import { TabDeck } from './TabDeck'
import type { DeckPanel } from './TabDeck'

/** One word inside each panel, so a slide can be told from its neighbours. */
const INSIDE: Record<string, string> = { identity: 'a name', class: 'a class', race: 'a race' }

const PANELS: DeckPanel[] = ['identity', 'class', 'race'].map((each) => ({
  value: each,
  label: each,
  content: <p>{INSIDE[each]}</p>,
}))

function deck(viewport: Viewport, value = 'class', onChange = vi.fn()) {
  const result = renderAt(
    viewport,
    <TabDeck label="Character build" panels={PANELS} value={value} onChange={onChange} />,
  )
  return { ...result, onChange }
}

/**
 * Both viewports, because this is one of the components that genuinely branch:
 * a phone gets a carousel of every panel, a wide screen gets the strip and the
 * active panel alone. The rule in docs/web.md is about trees that do *not*
 * branch; this is the case the rule exempts.
 *
 * Nothing below asserts which slide is *showing*. jsdom computes no layout and
 * embla measures the DOM, so a scroll position is not a claim this suite can
 * make -- the same bargain `SectionDeck.test.tsx` records. What is assertable
 * is that the strip reads the `value` it was handed and reports the presses it
 * gets, and that is true at both widths.
 */
describe('TabDeck', () => {
  describe.each(['mobile', 'desktop'] as const)('at %s', (viewport) => {
    it('draws every tab in order and marks the one the caller asked for', () => {
      deck(viewport)

      expect
        .soft(within(screen.getByRole('tablist')).getAllByRole('tab').map((tab) => tab.textContent))
        .toEqual(['identity', 'class', 'race'])
      expect
        .soft(screen.getByRole('tab', { name: 'class' }))
        .toHaveAttribute('aria-selected', 'true')
    })

    it('reports the tab that was pressed and changes nothing itself', async () => {
      const { onChange } = deck(viewport)

      await setupUser().click(screen.getByRole('tab', { name: 'race' }))

      // Controlled: which panel is on screen follows `value`, which is the
      // caller's to move.
      expect.soft(onChange).toHaveBeenCalledWith('race')
      expect.soft(screen.getByRole('tab', { name: 'class' })).toHaveAttribute(
        'aria-selected',
        'true',
      )
    })

    it('shows the panel the value names', () => {
      deck(viewport)

      expect(screen.getByText('a class')).toBeInTheDocument()
    })
  })

  describe('on a phone', () => {
    it('names the carousel, so the landmark says what it is', () => {
      deck('mobile')

      expect(screen.getByRole('region', { name: 'Character build' })).toBeInTheDocument()
    })

    it('mounts every panel, as a named slide', () => {
      deck('mobile')

      // All three, because the slide being swiped towards has to exist before
      // it arrives. Nothing here is behind a disclosure.
      for (const panel of PANELS) {
        const slide = screen.getByRole('group', { name: panel.label })
        expect.soft(within(slide).getByText(INSIDE[panel.value] ?? '')).toBeInTheDocument()
      }
    })
  })

  // Embla owns the gesture in a browser and jsdom draws no carousel to press,
  // so the rule it is given is tested rather than the swipe. What it protects:
  // a surface whose whole interaction is a tap that may drift a few pixels --
  // see features/character/ScoreAssignment -- where a drag both scrolls the
  // deck and swallows the click that was about to land.
  describe('what a swipe may start from', () => {
    it('leaves a marked surface, and everything inside it, alone', () => {
      const marked = document.createElement('div')
      marked.setAttribute(NO_SWIPE, 'true')
      const button = document.createElement('button')
      marked.append(button)

      expect.soft(swipeAllowedFrom(marked)).toBe(false)
      expect.soft(swipeAllowedFrom(button)).toBe(false)
      expect.soft(swipeAllowedFrom(document.createElement('div'))).toBe(true)
      // A gesture with no element under it is the deck's, as it was before.
      expect.soft(swipeAllowedFrom(null)).toBe(true)
    })
  })

  describe('on a wide screen', () => {
    /**
     * The point of the branch. A carousel answers a phone -- the panel is the
     * biggest thing on screen and a swipe is the cheapest gesture there is --
     * and answers nothing with a mouse, where a drag is how you select text and
     * five mounted panels are five times the work for a gesture nobody makes.
     */
    it('draws no carousel, and mounts only the panel that is showing', () => {
      deck('desktop')

      expect.soft(screen.queryByRole('region', { name: 'Character build' })).not.toBeInTheDocument()
      expect.soft(screen.queryAllByRole('group')).toHaveLength(0)
      expect.soft(screen.queryByText('a name')).not.toBeInTheDocument()
      expect.soft(screen.queryByText('a race')).not.toBeInTheDocument()
    })
  })
})
