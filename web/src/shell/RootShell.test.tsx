import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router'

import { testAccount, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { RootShell } from './RootShell'
import { NAV_ITEMS } from './nav'

function shellAt(viewport: 'mobile' | 'desktop') {
  const router = createMemoryRouter(
    [{ path: '/', element: <RootShell />, children: [{ index: true, element: <p>content</p> }] }],
    { initialEntries: ['/'] },
  )
  return renderAt(viewport, withAuth({}, <RouterProvider router={router} />))
}

describe('RootShell', () => {
  it('renders a navbar with full labels on desktop', () => {
    shellAt('desktop')

    expect(screen.getByRole('button', { name: 'Toggle navigation' })).toBeInTheDocument()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    for (const item of NAV_ITEMS) {
      expect(screen.getByRole('link', { name: item.label })).toBeInTheDocument()
    }
  })

  it('renders a bottom tab bar with short labels on mobile', () => {
    shellAt('mobile')

    expect(screen.getByRole('tablist')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Toggle navigation' })).not.toBeInTheDocument()
    for (const item of NAV_ITEMS) {
      expect(screen.getByRole('tab', { name: item.shortLabel ?? item.label })).toBeInTheDocument()
    }
  })

  // The account is who is looking, not a section of the app: it belongs beside
  // the control that ends the session, and nowhere in the navigation. The name
  // is the link, so there is no second "Account" control to find.
  it.each(['desktop', 'mobile'] as const)(
    'links the account name to /account from the %s header',
    (viewport) => {
      shellAt(viewport)

      const link = screen.getByRole('link', { name: testAccount.display_name })
      expect(link).toHaveAttribute('href', '/account')
      expect(screen.queryByRole('link', { name: 'Account' })).not.toBeInTheDocument()
      expect(NAV_ITEMS.some((item) => item.to === '/account')).toBe(false)
    },
  )

  it('renders the routed content at both viewports', () => {
    const { unmount } = shellAt('desktop')
    expect(screen.getByText('content')).toBeInTheDocument()
    unmount()

    shellAt('mobile')
    expect(screen.getByText('content')).toBeInTheDocument()
  })

  it('marks the current section active in both chromes', () => {
    shellAt('mobile')
    // The index route is '/', so the first nav item owns it.
    expect(screen.getByRole('tab', { name: NAV_ITEMS[0]!.shortLabel! })).toHaveAttribute(
      'aria-selected',
      'true',
    )
  })
})
