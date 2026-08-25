import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router'

import { testAccount, testGuest, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { RootShell } from './RootShell'
import { NAV_ITEMS } from './nav'

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
    for (const item of NAV_ITEMS) {
      expect(screen.getByRole('link', { name: item.label })).toBeInTheDocument()
    }
  })

  // The navbar has nothing to collapse it: a burger defaulting to open drew a
  // close cross to the left of the mark, before the app's own name.
  it('offers no control that hides the desktop navbar', () => {
    shellAt('desktop')

    expect(screen.queryByRole('button', { name: 'Toggle navigation' })).not.toBeInTheDocument()
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

    for (const item of NAV_ITEMS) {
      expect(screen.getByRole('menuitem', { name: item.label })).toHaveAttribute('href', item.to)
    }
    // Exactly one row is marked as where you already are.
    expect(screen.getAllByRole('menuitem').filter((el) => el.getAttribute('aria-current'))).toEqual([
      screen.getByRole('menuitem', { name: 'Characters' }),
    ])
  })

  // The trigger is the only thing on a phone naming the current section, so it
  // has to say something on a path that belongs to none.
  it.each([
    ['/', 'Characters'],
    ['/groups', 'Groups'],
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
      expect(NAV_ITEMS.some((item) => item.to === '/account')).toBe(false)
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
