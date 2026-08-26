import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { SectionDeck, type DeckSection } from './SectionDeck'

const sections: DeckSection[] = [
  { key: 'identity', title: 'Identity', desktop: 'full', content: <p>Elf</p> },
  { key: 'abilities', title: 'Abilities', desktop: 'full', content: <p>STR 16</p> },
  { key: 'skills', title: 'Skills', desktop: 'panel', content: <p>Stealth +7</p> },
  { key: 'gear', title: 'Resources and gear', desktop: 'panel', content: <p>A rope</p> },
]

const NAMES = ['Identity', 'Abilities', 'Skills', 'Resources and gear']

function deck(viewport: 'mobile' | 'desktop', override: DeckSection[] = sections) {
  return renderAt(viewport, <SectionDeck label="Character sheet" sections={override} />)
}

/**
 * Both viewports, deliberately, because this is one of the five components that
 * genuinely branch on width -- the two renderings share no container at all.
 * The rule in docs/web.md is about trees that do *not* branch; this is the case
 * the rule exempts.
 *
 * Nothing below asserts which slide is *showing*. jsdom computes no layout and
 * embla measures the DOM, so a scroll position is not a claim this suite can
 * make -- the same bargain `routes/LandingPage.test.tsx` records. What is
 * assertable is that the strip reads React state, which is why pressing a tab
 * has an observable effect here and swiping does not.
 */
describe('SectionDeck', () => {
  describe('on a wide screen', () => {
    it('draws every section at once, and nothing to press', () => {
      deck('desktop')

      for (const text of ['Elf', 'STR 16', 'Stealth +7', 'A rope']) {
        expect.soft(screen.getByText(text)).toBeInTheDocument()
      }
      expect.soft(screen.queryByRole('tablist')).not.toBeInTheDocument()
      expect.soft(screen.queryAllByRole('button')).toHaveLength(0)
    })

    // A `panel` is headed by its title; a `full` section is not, because the
    // wide layout draws those bare and the title is only there to name a tab.
    it('heads the panels and leaves the full-width sections bare', () => {
      deck('desktop')

      expect.soft(screen.getByRole('heading', { name: 'Skills' })).toBeInTheDocument()
      expect.soft(screen.getByRole('heading', { name: 'Resources and gear' })).toBeInTheDocument()
      expect.soft(screen.queryByRole('heading', { name: 'Identity' })).not.toBeInTheDocument()
      expect.soft(screen.queryByRole('heading', { name: 'Abilities' })).not.toBeInTheDocument()
    })

    // A `full` section between two panels breaks the grid, so that the order of
    // `sections` is the order of the page rather than everything bordered
    // sinking to the bottom.
    it('keeps the page in the order the sections were given', () => {
      deck('desktop', [
        sections[0]!,
        sections[2]!,
        { key: 'vitals', title: 'Vitals', desktop: 'full', content: <p>14 hp</p> },
        sections[3]!,
      ])

      const order = ['Elf', 'Stealth +7', '14 hp', 'A rope'].map((text) => screen.getByText(text))
      for (const [at, node] of order.entries()) {
        const next = order[at + 1]
        if (next === undefined) continue
        expect
          .soft(node.compareDocumentPosition(next) & Node.DOCUMENT_POSITION_FOLLOWING)
          .toBeTruthy()
      }
    })
  })

  describe('on a phone', () => {
    it('names the carousel, so the landmark says what it is', () => {
      deck('mobile')

      expect(screen.getByRole('region', { name: 'Character sheet' })).toBeInTheDocument()
    })

    it('carries every section in order, as a tab and as a named slide', () => {
      deck('mobile')

      expect
        .soft(within(screen.getByRole('tablist')).getAllByRole('tab').map((tab) => tab.textContent))
        .toEqual(NAMES)

      for (const name of NAMES) {
        expect.soft(screen.getByRole('group', { name })).toBeInTheDocument()
      }
    })

    /**
     * The point of the change, and the direct replacement for the
     * `aria-expanded` assertions `Columns.test.tsx` makes about its accordion.
     * Nothing here is behind a disclosure: every section is mounted, in the
     * accessibility tree, and readable -- which slide the viewport is scrolled
     * to is the carousel's business and not a state the sections are in.
     */
    it('hides nothing behind a disclosure', () => {
      deck('mobile')

      for (const text of ['Elf', 'STR 16', 'Stealth +7', 'A rope']) {
        expect.soft(screen.getByText(text)).toBeInTheDocument()
      }
      for (const tab of screen.getAllByRole('tab')) {
        expect.soft(tab).not.toHaveAttribute('aria-expanded')
      }
    })

    it('opens on the first section and moves when a tab is pressed', async () => {
      deck('mobile')

      const [first, second] = screen.getAllByRole('tab')
      expect.soft(first).toHaveAttribute('aria-selected', 'true')
      expect.soft(second).toHaveAttribute('aria-selected', 'false')

      await setupUser().click(screen.getByRole('tab', { name: 'Skills' }))

      expect.soft(first).toHaveAttribute('aria-selected', 'false')
      expect.soft(screen.getByRole('tab', { name: 'Skills' })).toHaveAttribute(
        'aria-selected',
        'true',
      )
    })

    /**
     * The cheapest possible pin on "a slide is as tall as the tallest one": the
     * inverse of `LandingPage.test.tsx`'s height assertion. Mantine's own
     * default for the custom property is `auto`, and setting it to anything
     * would be claiming a height jsdom cannot measure and a phone would have to
     * scroll inside.
     */
    it('fixes no height on the carousel', () => {
      deck('mobile')

      const region = screen.getByRole('region', { name: 'Character sheet' })
      expect(region.style.getPropertyValue('--carousel-height')).toBe('')
    })
  })
})
