import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { renderAt } from '@/test/render'

import { Pips } from './Pips'

describe('Pips', () => {
  // Lay on Hands at level 20 is a hundred points: a number, not a hundred marks.
  it('counts a large pool with a number instead of marks', () => {
    renderAt('desktop', <Pips name="Lay on Hands" max={100} used={40} />)

    expect(screen.getByText('60 / 100')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Lay on Hands: 60 of 100 left' }).querySelector('svg')).toBeNull()
  })

  it('says a pool without a limit is unlimited rather than drawing it', () => {
    renderAt('desktop', <Pips name="Rage Uses" max={9999} used={0} />)

    expect(screen.getByText('Unlimited')).toBeInTheDocument()
  })
})
