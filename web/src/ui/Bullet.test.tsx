import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

import { Bullet } from './Bullet'
import { ProficiencyMark } from './ProficiencyMark'

/** Every attribute that decides what the ring looks like. */
const RING = ['cx', 'cy', 'r', 'fill', 'stroke', 'stroke-width', 'opacity']

function ringOf(root: ParentNode): Record<string, string | null> {
  const circles = root.querySelectorAll('circle')
  expect(circles).toHaveLength(1)
  const circle = circles[0]!
  return Object.fromEntries(RING.map((name) => [name, circle.getAttribute(name)]))
}

/**
 * One viewport. Nothing here branches on width, and the suite runs without CSS.
 */
describe('Bullet', () => {
  /**
   * The claim the component is built on, and the one that would rot silently.
   * Four lists share this sheet and two of them are marked by
   * `ProficiencyMark`; if the two rings drift apart in diameter, weight or
   * indent, the lists stop looking like one thing and no other test notices.
   */
  it('draws the same ring an untrained skill does', () => {
    const { container: bullet } = renderAt('desktop', <Bullet />)
    const drawn = ringOf(bullet)

    const { container: mark } = renderAt('desktop', <ProficiencyMark level="none" />)

    expect(drawn).toEqual(ringOf(mark))
  })

  /**
   * And says nothing, which is the whole reason it is not that component.
   * `ProficiencyMark` names itself "Not proficient" and explains proficiency
   * bonuses in a tooltip; beside "Darkvision" that would be a false statement
   * about a racial trait rather than a decoration.
   */
  it('is a decoration, not a statement', () => {
    renderAt('desktop', <Bullet />)

    expect.soft(screen.queryByRole('img')).not.toBeInTheDocument()
    expect.soft(screen.queryByText(/proficient/i)).not.toBeInTheDocument()
  })

  it('takes a size', () => {
    const { container } = renderAt('desktop', <Bullet size={20} />)

    expect(container.querySelector('svg')).toHaveStyle({ width: '20px', height: '20px' })
  })
})
