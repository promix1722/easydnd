import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

import { D20Roll } from './D20'

/**
 * What is testable here, and what deliberately is not.
 *
 * The die itself is three.js and cannon-es in a WebGL context. jsdom
 * implements no WebGL at all, so `ui/D20Scene.tsx` cannot be rendered, let
 * alone thrown -- and it is behind a dynamic `import()` that never resolves in
 * a suite with no intersection to trigger it. Asserting on a die that cannot
 * exist would mean asserting on a mock, and this suite bans `vi.mock` for
 * reasons `scripts/check-layers.mjs` explains at length.
 *
 * So the shape of the coverage is: the geometry is pinned in
 * `d20Geometry.test.ts`, where it is pure arithmetic and every claim is
 * checkable; the throw and the settle are exercised by hand on a device; and
 * this file pins the part that always ships -- that the heavy half stays
 * unloaded until something asks for it, and that the page says something
 * sensible in the meantime.
 */
describe('D20Roll', () => {
  /*
   * The assertion that protects the bundle.
   *
   * `D20Scene` is roughly 160 kB gzipped and both of the die's homes are the
   * app's front door. It is loaded on an intersection, and jsdom's
   * IntersectionObserver is an inert stub that never reports one -- so nothing
   * here should ever reach the scene. If a refactor turned the lazy import
   * into a static one, or moved the load to mount, this is what would notice,
   * because the fallback would stop being what is on screen.
   */
  it('does not load the 3D scene until something brings it into view', () => {
    renderAt('mobile', <D20Roll />)

    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(document.querySelector('canvas')).toBeNull()
  })

  /*
   * The live region is the die's *only* accessible surface.
   *
   * There is deliberately no visible caption and no printed result: the camera
   * looks straight down, so the number that landed is the one facing you. That
   * leaves nothing at all for a screen reader, because the number is painted
   * into a WebGL canvas -- so this is not a duplicate of something on screen,
   * it is the whole channel, and removing it would make the die unusable
   * rather than merely terser.
   */
  it('keeps a polite live region for the result', () => {
    const { container } = renderAt('mobile', <D20Roll />)

    const live = container.querySelector('[aria-live="polite"]')

    expect(live).toBeInTheDocument()
    expect(live).toHaveAttribute('aria-atomic')
    // Empty until something has actually been rolled -- an announcement of a
    // number nobody threw is worse than silence.
    expect(live).toHaveTextContent('')
  })
})
