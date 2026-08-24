import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

import { DragonMark } from './DragonMark'

/**
 * The mark is the whole of the signed-out landing page, so the thing worth
 * pinning is not how it is drawn but that it is *announced*. A decorative
 * version of this component would leave that page empty to a screen reader.
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

  // The landing page passes a CSS clamp rather than a number, so the prop has
  // to survive being a string all the way to the rendered style -- which is
  // why it is set there and not on the width attribute.
  it('takes a CSS length for its size', () => {
    renderAt('desktop', <DragonMark size="min(64vw, 300px)" />)

    const mark = screen.getByRole('img', { name: 'easydnd' })
    expect(mark.style.width).toBe('min(64vw, 300px)')
    expect(mark.style.height).toBe('min(64vw, 300px)')
  })
})
