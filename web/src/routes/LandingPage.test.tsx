import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

import { LandingPage } from './LandingPage'

const SLIDE_NAMES = ['Build a character', 'Join a group', 'Run an adventure']

/**
 * The page carries no copy yet, so its accessible names are the whole of what
 * there is to pin -- which is the point of putting them there rather than
 * waiting for a headline. Nothing here asserts which slide is *showing*: jsdom
 * has no layout, embla measures the DOM, and a scroll position it cannot
 * compute is not a claim this suite can make.
 */
describe('LandingPage', () => {
  it.each(['mobile', 'desktop'] as const)('names what the app is for at %s', (viewport) => {
    renderAt(viewport, <LandingPage />)

    const region = screen.getByRole('region', { name: 'What easydnd is for' })
    expect(region).toBeInTheDocument()

    for (const name of SLIDE_NAMES) {
      expect(screen.getByRole('group', { name })).toBeInTheDocument()
    }
  })

  it('keeps the slides in the order the app is met in', () => {
    renderAt('desktop', <LandingPage />)

    const headings = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)

    expect(headings).toEqual(SLIDE_NAMES)
  })

  // Each panel is named by its own heading rather than by a second copy of the
  // words in an aria-label. Pinning the wiring and not just the name is what
  // catches an id that stops resolving -- at which point the name silently
  // becomes "Carousel slide" again and getByRole above would still pass.
  it('names each panel by its own heading', () => {
    renderAt('desktop', <LandingPage />)

    for (const name of SLIDE_NAMES) {
      const slide = screen.getByRole('group', { name })
      const heading = screen.getByRole('heading', { level: 2, name })

      expect(slide.getAttribute('aria-labelledby')).toBe(heading.id)
      expect(slide).toContainElement(heading)
    }
  })

  // Sample copy, but a panel with a heading and no sentence under it is a
  // different layout from the one this page was built around.
  it('carries a caption under each heading', () => {
    renderAt('desktop', <LandingPage />)

    for (const name of SLIDE_NAMES) {
      const slide = screen.getByRole('group', { name })
      const caption = slide.querySelector('p')

      expect(caption?.textContent ?? '').not.toBe('')
    }
  })

  // The controls are the only way through for a visitor with neither a
  // touchscreen nor the arrow keys, and they sit over a panel rather than
  // beside it. 26px -- Mantine's default -- is under every published minimum
  // for a pointer target, so the override is worth pinning.
  it('gives the controls a pointer-sized target', () => {
    renderAt('desktop', <LandingPage />)

    // Mantine converts the number to rem and multiplies by its scale factor, so
    // the property holds `calc(2.75rem * var(--mantine-scale))` -- 44px at the
    // default root size -- rather than anything parseFloat can read.
    const size = screen
      .getByRole('region', { name: 'What easydnd is for' })
      .style.getPropertyValue('--carousel-control-size')

    expect(size).toContain('2.75rem')
    expect(screen.getByRole('button', { name: 'Next slide' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Previous slide' })).toBeInTheDocument()
  })

  // And not drawn at all on a phone, which is the viewport they are least use
  // on: they cover the panel they sit over to duplicate a swipe the screen
  // already offers. The indicators still say how many panels there are.
  it('draws no arrows on a touchscreen', () => {
    renderAt('mobile', <LandingPage />)

    expect(screen.queryByRole('button', { name: 'Next slide' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Previous slide' })).not.toBeInTheDocument()
    // The panels are still reachable -- this removes a control, not a way
    // through.
    expect(screen.getByRole('region', { name: 'What easydnd is for' })).toBeInTheDocument()
  })

  // Only the first of the three works end to end, so none of them is a door:
  // a carousel where one panel navigates and two do not teaches the wrong
  // thing about the app. The header's "Log in" is the only control, and the
  // captions describe rather than invite.
  it('offers nothing to press on the slides themselves', () => {
    renderAt('desktop', <LandingPage />)

    for (const name of SLIDE_NAMES) {
      const slide = screen.getByRole('group', { name })
      expect(slide.querySelector('a')).toBeNull()
      expect(slide.querySelector('button')).toBeNull()
    }
  })

  // The hero mark moved off this page: the header wordmark already names the
  // app, and a mark above a carousel is two heroes competing for one glance.
  it('does not show the hero mark', () => {
    renderAt('desktop', <LandingPage />)

    expect(screen.queryByRole('img', { name: 'easydnd' })).not.toBeInTheDocument()
  })

  // The descendant of DragonMark's "takes a CSS length for its size" test, and
  // it earns its place twice. Mantine's rem() mangles a length that does not
  // begin `calc(` or `clamp(`, so this catches the height arriving as nonsense;
  // and it catches an edit that drops the footer from the calc, which is the
  // difference between filling the page and running underneath the footer.
  // Neither is visible to a test that only counts slides, and neither needs a
  // layout engine to detect.
  it('sizes itself to what the header and the footer leave', () => {
    renderAt('desktop', <LandingPage />)

    const height = screen
      .getByRole('region', { name: 'What easydnd is for' })
      .style.getPropertyValue('--carousel-height')

    expect(height).toContain('--app-shell-header-offset')
    expect(height).toContain('--app-shell-footer-offset')
    expect(height).toContain('--app-shell-padding')
    expect(height.startsWith('calc(')).toBe(true)
  })
})
