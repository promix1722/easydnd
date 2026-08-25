import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

import { ProficiencyMark, type ProficiencyLevel } from './ProficiencyMark'

/**
 * What these pin is that the mark is *readable*, not that it is drawn.
 *
 * A skill panel showing eighteen rows distinguishes six of them by this glyph
 * alone. If the four levels ever collapsed into one accessible name, the panel
 * would still look right and would tell a screen reader nothing -- and a test
 * asserting on the SVG's paths would have gone on passing. So these assert on
 * the name each level is announced by, which is the thing a reader actually
 * gets.
 */
describe('ProficiencyMark', () => {
  it('announces each level by a name of its own', () => {
    const names = (['none', 'half', 'proficient', 'expertise'] as ProficiencyLevel[]).map(
      (level) => {
        const { unmount } = renderAt('desktop', <ProficiencyMark level={level} />)
        const name = screen.getByRole('img').getAttribute('aria-label') ?? ''
        unmount()
        return name
      },
    )

    // Four levels, four distinct names -- nothing collapsed into "proficient".
    expect(new Set(names).size).toBe(4)
    expect(names[0]).toMatch(/not proficient/i)
    expect(names[3]).toMatch(/doubled/i)
  })

  // Expertise is the one a player scans for, and "proficient" is a prefix of
  // nothing else here -- but a name match that is merely *contained* in
  // another would make the two indistinguishable to an accessible-name query.
  it('does not announce expertise as a kind of proficient', () => {
    renderAt('desktop', <ProficiencyMark level="expertise" />)

    expect(screen.getByRole('img', { name: /Expertise/ })).toBeInTheDocument()
    expect(screen.queryByRole('img', { name: /^Proficient/ })).not.toBeInTheDocument()
  })

  // A <title> child would be a text node inside the row, and the skills panel
  // reads whole rows as text -- "Stealth DEX +7" must not come back with a
  // sentence about proficiency bonuses in the middle of it.
  it('carries its name in an attribute rather than as text on the page', () => {
    renderAt('desktop', <ProficiencyMark level="expertise" />)

    const mark = screen.getByRole('img')
    expect(mark.querySelector('title')).toBeNull()
    expect(mark.textContent).toBe('')
  })

  it('takes its colour from the row it sits in rather than a literal', () => {
    // The panel dims untrained rows and inks trained ones; a mark with a baked
    // colour would be the one thing on the row that ignored that, and would
    // also ignore the colour scheme.
    renderAt('desktop', <ProficiencyMark level="proficient" />)

    const mark = screen.getByRole('img')
    for (const shape of mark.querySelectorAll('circle, path')) {
      const painted = [shape.getAttribute('fill'), shape.getAttribute('stroke')]
      for (const value of painted) {
        if (value !== null && value !== 'none') expect(value).toBe('currentColor')
      }
    }
  })
})
