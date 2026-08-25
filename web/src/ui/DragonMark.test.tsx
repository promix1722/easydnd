import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

import { DragonMark } from './DragonMark'

/**
 * The mark was the whole of the signed-out landing page until the carousel
 * replaced it, and it is still the app's hero art -- currently unplaced. These
 * tests stay because what they pin is the convention it set for inline SVGs
 * rather than its old address: that a mark carrying a page is *announced*
 * rather than decorative, and that its size survives being a CSS length. Both
 * outlive the page it used to be on.
 */
describe('DragonMark', () => {
  it('is announced by name rather than hidden', () => {
    renderAt('desktop', <DragonMark />)

    expect(screen.getByRole('img', { name: 'easydnd' })).toBeInTheDocument()
  })

  it('lets a caller name it something else', () => {
    renderAt('desktop', <DragonMark title="easydnd, a red dragon" />)

    expect(screen.getByRole('img', { name: 'easydnd, a red dragon' })).toBeInTheDocument()
  })

  // Callers pass a CSS clamp rather than a number, so the prop has to survive
  // being a string all the way to the rendered style -- which is why it is set
  // there and not on the width attribute. `routes/LandingPage.tsx` has the same
  // requirement of the carousel's height, and pins it the same way.
  it('takes a CSS length for its size', () => {
    renderAt('desktop', <DragonMark size="min(64vw, 300px)" />)

    const mark = screen.getByRole('img', { name: 'easydnd' })
    expect(mark.style.width).toBe('min(64vw, 300px)')
    expect(mark.style.height).toBe('min(64vw, 300px)')
  })
})
