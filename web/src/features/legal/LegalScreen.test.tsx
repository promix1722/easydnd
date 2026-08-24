import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

import { CC_BY_URL, LICENSE_URL, SRD_ATTRIBUTION, SRD_URL } from './attribution'
import { LegalScreen } from './LegalScreen'

/**
 * CC-BY-4.0 wants the notice in the product, so "it renders" is the whole
 * contract here -- and the wording it renders is pinned against the generator
 * separately, in `attribution.test.ts`.
 */
describe('LegalScreen', () => {
  it('shows the SRD 5.1 notice in full', () => {
    const { container } = renderAt('desktop', <LegalScreen />)

    // Normalised, because the paragraph is broken across elements by the two
    // links inside it -- which is the point of rendering it this way.
    const rendered = container.textContent?.replace(/\s+/g, ' ') ?? ''
    expect(rendered).toContain(SRD_ATTRIBUTION)
  })

  it('makes both licence URLs reachable rather than retypeable', () => {
    renderAt('desktop', <LegalScreen />)

    expect(screen.getByRole('link', { name: SRD_URL })).toHaveAttribute('href', SRD_URL)
    expect(screen.getByRole('link', { name: CC_BY_URL })).toHaveAttribute('href', CC_BY_URL)
  })

  // The split is the point: the code is this project's to license, the game
  // material is not.
  it('keeps the project MIT notice separate from the game data', () => {
    renderAt('desktop', <LegalScreen />)

    expect(screen.getByRole('heading', { name: 'easydnd' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'SRD 5.1' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'MIT License' })).toHaveAttribute('href', LICENSE_URL)
    expect(screen.getByText(/Copyright \(c\) 2026 The easydnd project/)).toBeInTheDocument()
  })
})
