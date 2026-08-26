import { screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router'
import type { ReactNode } from 'react'

import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'
import { setupUser } from '@/test/user'

import { Page } from './Page'
import type { PageProps } from './Page'

function renderPage(
  at: string,
  props: Omit<PageProps, 'children'> & { children?: ReactNode },
  viewport: Viewport = 'desktop',
) {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={[at]}>
      <Page {...props}>{props.children ?? <p>the body</p>}</Page>
    </MemoryRouter>,
  )
}

/**
 * As in TabRow.test.tsx: Mantine's generated ids differ between two renders.
 *
 * `__m__-_r_4l_` is the third form, and it arrives with the responsive `fz` on
 * the trail: a responsive style prop is emitted as a `<style>` element plus a
 * class minted from `useId`, so two renders of the same tree disagree on the
 * name while agreeing on everything the name refers to.
 */
function structure(html: string): string {
  return html
    .replace(/(id|aria-controls|aria-labelledby|for)="[^"]*"/g, '$1=""')
    .replace(/__m__-_r_[a-z0-9]+_/gi, '__m__-id_')
    .replace(/mantine-[a-z0-9]+-/gi, 'mantine-')
}

/**
 * One viewport, because the last test in this file proves it is enough.
 *
 * `Page` is deliberately not a fifth component that branches on width: the
 * heading row wraps because it is allowed to wrap, and the content cap is
 * simply inert below 1024px. That claim is worth pinning rather than asserting
 * in prose, because the cheapest way to "fix" a future layout problem here
 * would be to reach for `useIsDesktop` -- and this file would go red.
 */
describe('Page', () => {
  it('draws a section root as a heading with no breadcrumb trail', () => {
    // The reason "the trail replaces the title" needs no special case for the
    // three list screens: one crumb is a heading, and there is nothing above
    // it to navigate to.
    renderPage('/groups', { trail: [] })

    expect(screen.getByRole('heading', { level: 2, name: 'Groups' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Breadcrumb' })).not.toBeInTheDocument()
  })

  it('derives the first crumb from the URL rather than the caller', () => {
    // A screen cannot start its trail somewhere the navbar disagrees with,
    // because a screen does not get to say where it starts.
    renderPage('/groups/grp_1', { trail: [{ label: 'Wednesday Night' }] })

    const trail = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(within(trail).getByRole('link', { name: 'Groups' })).toHaveAttribute('href', '/groups')
  })

  it('draws the last crumb as the page heading, and not as a link', () => {
    renderPage('/groups/grp_1', { trail: [{ label: 'Wednesday Night' }] })

    expect(
      screen.getByRole('heading', { level: 2, name: 'Wednesday Night' }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Wednesday Night' })).not.toBeInTheDocument()
  })

  it('keeps exactly one heading on the page, however deep the trail', () => {
    // The property the whole design turns on: the trail ends in the heading
    // rather than sitting above a second copy of it.
    renderPage('/groups/grp_1/characters/chr_1', {
      trail: [{ label: 'Wednesday Night', to: '/groups/grp_1' }, { label: 'Ada' }],
    })

    expect(screen.getAllByRole('heading')).toHaveLength(1)
    expect(screen.getByRole('heading', { level: 2, name: 'Ada' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Wednesday Night' })).toHaveAttribute(
      'href',
      '/groups/grp_1',
    )
  })

  it('names the heading while the name it needs is still being fetched', () => {
    // An <h2> with no accessible name is a hole in the page. The skeleton is
    // what stops the heading arriving a beat late and moving everything under
    // it; the hidden word is what stops it being nameless in the meantime.
    renderPage('/groups/grp_1', { trail: [{ label: null }] })

    expect(screen.getByRole('heading', { level: 2, name: 'Loading' })).toBeInTheDocument()
    expect(screen.queryByText('undefined')).not.toBeInTheDocument()
    expect(screen.queryByText('null')).not.toBeInTheDocument()
  })

  it('draws a badge and actions on the heading line', () => {
    renderPage('/groups/grp_1', {
      trail: [{ label: 'Wednesday Night' }],
      badge: <span>Owner</span>,
      actions: <button>Rename</button>,
    })

    expect(screen.getByText('Owner')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rename' })).toBeInTheDocument()
  })

  it('replaces the body while loading, and keeps the trail', () => {
    // The behaviour that retired three early returns: you still know where you
    // are while the thing you came for is on its way.
    renderPage('/groups/grp_1', {
      trail: [{ label: 'Wednesday Night' }],
      state: { kind: 'loading' },
    })

    expect(screen.getByText('Loading...')).toBeInTheDocument()
    expect(screen.queryByText('the body')).not.toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Wednesday Night' })).toBeInTheDocument()
  })

  it('takes a more specific word than "Loading" when a screen has one', () => {
    renderPage('/characters/chr_1', {
      trail: [{ label: null }],
      state: { kind: 'loading', what: 'Projecting the sheet...' },
    })

    expect(screen.getByText('Projecting the sheet...')).toBeInTheDocument()
  })

  it('replaces the body on failure, keeps the trail, and offers the retry', async () => {
    const onRetry = vi.fn()
    const user = setupUser()
    renderPage('/games/gam_1', {
      trail: [{ label: 'Thursday night' }],
      state: {
        kind: 'failed',
        title: 'Could not load this game',
        detail: 'the server said no',
        onRetry,
      },
    })

    expect(screen.getByText('Could not load this game')).toBeInTheDocument()
    expect(screen.getByText('the server said no')).toBeInTheDocument()
    expect(screen.queryByText('the body')).not.toBeInTheDocument()
    // Alone on a blank page is what this used to be. Under a trail is the point.
    expect(screen.getByRole('heading', { level: 2, name: 'Thursday night' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Try again' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('omits the retry when a screen offers no way to ask again', () => {
    renderPage('/games/gam_1', {
      trail: [{ label: 'Thursday night' }],
      state: { kind: 'failed', title: 'Gone', detail: 'That game is not there.' },
    })

    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument()
  })

  it('draws a heading on a path in no section at all', () => {
    // /account and /login belong to no section, so there is no first crumb to
    // derive and the trail is whatever the screen passed.
    renderPage('/account', { trail: [{ label: 'Account' }] })

    expect(screen.getByRole('heading', { level: 2, name: 'Account' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Breadcrumb' })).not.toBeInTheDocument()
  })

  /*
   * The blank line a phone used to draw above every list.
   *
   * A section root's heading is the section's own name, which the phone's
   * chrome is already showing an inch above it -- so it is hidden below `md`.
   * What that left behind was the row it sat in: `ROW_HEIGHT` of nothing plus
   * the stack's gap, at the top of the three busiest screens in the app.
   *
   * The class is Mantine's own `visibleFrom`, which is why these assert a
   * class rather than a computed height: the suite runs without CSS, so a
   * media query is a name in the markup and nothing more. That is the same
   * reason `Page` may not branch on width, and this does not -- what it
   * branches on is whether there is anything left to draw.
   */
  const HIDDEN_ON_PHONE = '.mantine-visible-from-md'

  it('leaves a section root nothing to draw on a phone', () => {
    const { container } = renderPage('/', { trail: [] })

    // The row that carries ROW_HEIGHT goes with the word that was in it, and
    // the block that holds them with the row.
    expect(container.querySelector('[style*="min-height"]')?.closest(HIDDEN_ON_PHONE)).not.toBeNull()
  })

  it('keeps the row where a section root has something else on it', () => {
    const { container } = renderPage('/', { trail: [], actions: <button>New character</button> })

    expect(container.querySelector('[style*="min-height"]')?.closest(HIDDEN_ON_PHONE)).toBeNull()
    // Only the duplicated word goes. The action stays at both widths.
    expect(
      screen.getByRole('heading', { name: 'Characters' }).closest(HIDDEN_ON_PHONE),
    ).not.toBeNull()
    expect(screen.getByRole('button', { name: 'New character' }).closest(HIDDEN_ON_PHONE)).toBeNull()
  })

  it('keeps the row on a page whose heading is its own', () => {
    // A detail page's heading is the thing the page is about, which the chrome
    // says nowhere. Nothing here is a restatement, so nothing here is dropped.
    const { container } = renderPage('/characters/chr_1', { trail: [{ label: 'Ada' }] })

    expect(container.querySelector('[style*="min-height"]')?.closest(HIDDEN_ON_PHONE)).toBeNull()
  })

  it('renders the same markup at both viewports', () => {
    // What earns every test above the right to run once. See the note on this
    // describe block.
    const props = {
      trail: [{ label: 'Wednesday Night', to: '/groups/grp_1' }, { label: 'Ada' }],
      badge: <span>Read only</span>,
      actions: <button>Rename</button>,
    }

    const mobile = renderPage('/groups/grp_1/characters/chr_1', props, 'mobile')
    const mobileHtml = structure(mobile.container.innerHTML)
    mobile.unmount()

    const desktop = renderPage('/groups/grp_1/characters/chr_1', props, 'desktop')
    expect(structure(desktop.container.innerHTML)).toBe(mobileHtml)
  })
})
