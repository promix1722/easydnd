import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router'

import { testAccount, testGuest, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { SECTIONS } from '@/ui'

import en from '@locales/en.json'

import { RootShell } from './RootShell'

/**
 * What a section is called on screen.
 *
 * `Section.label` is a message key, because the table is a constant and the
 * language is React state -- the chrome translates it where it draws it. These
 * tests are about the chrome drawing every section, so they have to ask the
 * catalogue the same question the chrome asks, rather than restating three
 * English words that would then be free to drift from it.
 */
const named = (section: (typeof SECTIONS)[number]) =>
  (en as Record<string, string>)[section.label] ?? section.label

function shellAt(viewport: 'mobile' | 'desktop', at = '/') {
  const router = createMemoryRouter(
    [
      {
        path: '/',
        element: <RootShell />,
        children: [
          { index: true, element: <p>content</p> },
          // Enough of the table for the chrome to be exercised on a section
          // other than the first, and on a path that belongs to no section.
          { path: 'groups', element: <p>content</p> },
          { path: 'account', element: <p>content</p> },
          // A detail page under `/`, which is the case that used to leave the
          // navbar unlit and the phone's trigger reading "Menu".
          { path: 'characters/:id', element: <p>content</p> },
        ],
      },
    ],
    { initialEntries: [at] },
  )
  return renderAt(viewport, withAuth({}, <RouterProvider router={router} />))
}

describe('RootShell', () => {
  it('renders a navbar with full labels on desktop', () => {
    shellAt('desktop')

    expect(screen.getByRole('navigation')).toBeInTheDocument()
    for (const section of SECTIONS) {
      expect(screen.getByRole('link', { name: named(section) })).toBeInTheDocument()
    }
  })

  /*
   * The navbar narrows to a rail, and the control says which way.
   *
   * Asserted on the control and on the links surviving, rather than on width:
   * the navbar is sized from a generated `<style>` element, this suite runs
   * with `css: false`, and jsdom lays nothing out, so no test here can see how
   * wide anything is. `aria-expanded` and the accessible names are the real
   * semantics and the only half of this a test can reach.
   *
   * Know what that means before trusting this file. An earlier attempt at this
   * -- hiding the navbar outright -- passed every assertion in it while the
   * navbar did not move at all, because `AppShell`'s `collapsed` prop is read
   * only in a mode `breakpoint: 'never'` opts out of. The visual half is
   * checked by opening the page.
   *
   * The assertion before that was the opposite again: that nothing could touch
   * the navbar at all. What was wrong then is still wrong and is why the
   * control sits where it does -- a `Burger` defaulting to open drew a close
   * cross, to the left of the mark and before the app's own name. This one
   * lives at the foot of the thing it resizes.
   */
  it('narrows the desktop navbar to a rail and back', async () => {
    const user = setupUser()
    shellAt('desktop')

    const collapse = screen.getByRole('button', { name: 'Collapse navigation' })
    expect(collapse).toHaveAttribute('aria-expanded', 'true')

    await user.click(collapse)

    const expand = screen.getByRole('button', { name: 'Expand navigation' })
    expect(expand).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('button', { name: 'Collapse navigation' })).not.toBeInTheDocument()

    await user.click(expand)
    expect(screen.getByRole('button', { name: 'Collapse navigation' })).toBeInTheDocument()
  })

  // The point of a rail over a navbar that disappears: the sections are still
  // there and still named, so nothing has to be found again to navigate.
  it('keeps every section reachable and named on the rail', async () => {
    const user = setupUser()
    shellAt('desktop')

    await user.click(screen.getByRole('button', { name: 'Collapse navigation' }))

    for (const section of SECTIONS) {
      expect(screen.getByRole('link', { name: named(section) })).toHaveAttribute('href', section.to)
    }
  })

  // A genuine width branch, so it belongs in the one file allowed two
  // viewports: the phone has no navbar for a control to act on.
  it('offers no navigation toggle on mobile', () => {
    shellAt('mobile')

    expect(screen.queryByRole('button', { name: /navigation$/ })).not.toBeInTheDocument()
  })

  // The bottom tab bar this replaced is gone, not hidden: a phone gets one row
  // of chrome, and the sections live behind the control in it.
  it('renders no tab bar at either viewport', () => {
    const { unmount } = shellAt('mobile')
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    unmount()

    shellAt('desktop')
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })

  it('opens every section from the mobile header dropdown', async () => {
    const user = setupUser()
    shellAt('mobile')

    await user.click(screen.getByRole('button', { name: 'Characters' }))

    for (const section of SECTIONS) {
      expect(screen.getByRole('menuitem', { name: named(section) })).toHaveAttribute(
        'href',
        section.to,
      )
    }
    // Exactly one row is marked as where you already are.
    expect(screen.getAllByRole('menuitem').filter((el) => el.getAttribute('aria-current'))).toEqual([
      screen.getByRole('menuitem', { name: 'Characters' }),
    ])
  })

  // The trigger is the only thing on a phone naming the current section, so it
  // has to say something on a path that belongs to none.
  // `/characters/chr_1` is the one that changed: the section table now owns
  // that prefix, so a sheet names its section instead of falling back.
  it.each([
    ['/', 'Characters'],
    ['/groups', 'Groups'],
    ['/characters/chr_1', 'Characters'],
    ['/account', 'Menu'],
  ])('labels the mobile dropdown %s as %s', (at, label) => {
    shellAt('mobile', at)

    expect(screen.getByRole('button', { name: label })).toBeInTheDocument()
  })

  it('marks the current section active in the desktop navbar', () => {
    shellAt('desktop', '/groups')

    expect(screen.getByRole('link', { name: 'Groups' })).toHaveAttribute('data-active', 'true')
  })

  // The account is who is looking, not a section of the app: it belongs beside
  // the control that ends the session, and nowhere in the navigation. It is an
  // icon at both viewports now, so the name it used to print is its accessible
  // name -- the header still says whose session this is when asked, it just no
  // longer spends a phone's narrowest row saying so unprompted.
  it.each(['desktop', 'mobile'] as const)(
    'links the account icon to /account from the %s header',
    (viewport) => {
      shellAt(viewport)

      const link = screen.getByRole('link', { name: `Account: ${testAccount.display_name}` })
      expect(link).toHaveAttribute('href', '/account')
      expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
      expect(SECTIONS.some((section) => section.to === '/account')).toBe(false)
    },
  )

  // The point of the change: the name is a label, not chrome. getByText does
  // not match an aria-label, which is exactly the property being asserted.
  it.each(['desktop', 'mobile'] as const)(
    'does not print the account name on the %s header',
    (viewport) => {
      shellAt(viewport)

      expect(screen.queryByText(testAccount.display_name)).not.toBeInTheDocument()
    },
  )

  // A guest's session is the only copy of their work. An icon cannot draw that
  // distinction, so the label has to carry it.
  it.each(['desktop', 'mobile'] as const)(
    'names the %s sign-out control for a guest session',
    (viewport) => {
      const router = createMemoryRouter(
        [{ path: '/', element: <RootShell />, children: [{ index: true, element: <p>x</p> }] }],
        { initialEntries: ['/'] },
      )
      renderAt(viewport, withAuth({ user: testGuest }, <RouterProvider router={router} />))

      expect(screen.getByRole('button', { name: 'End guest session' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument()
    },
  )

  // The word goes on a phone and the mark stays, so the mark stops being
  // decorative and takes the name. See Wordmark.
  it('drops the wordmark caption on mobile and keeps it on desktop', () => {
    const { unmount } = shellAt('mobile')
    expect(screen.queryByRole('heading', { name: 'easydnd' })).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'easydnd' })).toBeInTheDocument()
    unmount()

    shellAt('desktop')
    expect(screen.getByRole('heading', { name: 'easydnd' })).toBeInTheDocument()
  })

  it('renders the routed content at both viewports', () => {
    const { unmount } = shellAt('desktop')
    expect(screen.getByText('content')).toBeInTheDocument()
    unmount()

    shellAt('mobile')
    expect(screen.getByText('content')).toBeInTheDocument()
  })
})
