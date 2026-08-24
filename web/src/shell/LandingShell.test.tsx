import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router'

import type { AuthState } from '@/lib/auth'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { LandingShell } from './LandingShell'

function landingAt(state: Partial<AuthState> = {}, path = '/') {
  const router = createMemoryRouter(
    [
      {
        path: '/',
        element: <LandingShell />,
        children: [
          { index: true, element: <p>pitch</p> },
          { path: 'login', element: <p>the login page</p> },
        ],
      },
    ],
    { initialEntries: [path] },
  )
  return renderAt(
    'desktop',
    withAuth({ status: 'anonymous', user: null, ...state }, <RouterProvider router={router} />),
  )
}

describe('LandingShell', () => {
  // One control, and it navigates. Choosing between the three ways in needs
  // room to explain what each costs, which lives on /login.
  it('offers a single way in from the header', () => {
    landingAt()

    const login = screen.getByRole('link', { name: 'Log in' })
    expect(login).toBeInTheDocument()
    expect(login).toHaveAttribute('href', '/login')
  })

  // Load-bearing for the footer's markup, not just a leftover: nowhere to
  // navigate to yet, because every section of the app is somebody's -- and two
  // links to static documents are not the app's navigation. So LandingFooter
  // renders its links bare inside AppShell.Footer's <footer>, which announces
  // itself as contentinfo, and never wraps them in a <nav>.
  it('exposes a footer landmark and no navigation', () => {
    landingAt()

    expect(screen.getByRole('contentinfo')).toBeInTheDocument()
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
  })

  // The middle one is why the footer exists at all: the SRD 5.1 data is
  // CC-BY-4.0, and that licence expects its notice in the product rather than
  // only in the repository. See shell/LandingFooter.tsx.
  it('carries the source, the terms and the build', () => {
    landingAt()

    const footer = screen.getByRole('contentinfo')

    expect(screen.getByRole('link', { name: 'GitHub' })).toHaveAttribute(
      'href',
      'https://github.com/promix1722/easydnd',
    )
    expect(screen.getByRole('link', { name: 'Licences' })).toHaveAttribute('href', '/legal')
    // 'dev' is what buildinfo reports when VITE_APP_VERSION is unset, which is
    // every test run and every `vite dev`.
    expect(footer).toHaveTextContent('dev')
  })

  // The footer is the logged-out chrome's, so it follows /status and /legal in
  // rather than disappearing for somebody who is already signed in.
  it('keeps the footer for a signed-in visitor on this chrome', () => {
    landingAt({ status: 'authenticated' })

    expect(screen.getByRole('link', { name: 'Licences' })).toBeInTheDocument()
  })

  // Unlike the passkey buttons it replaced, this one does not depend on
  // WebAuthn -- a browser without it can still reach the guest option.
  it('offers the way in even without WebAuthn', () => {
    landingAt()

    expect(screen.getByRole('link', { name: 'Log in' })).toBeInTheDocument()
  })

  it('hides the button on the page it leads to', () => {
    landingAt({}, '/login')

    expect(screen.queryByRole('link', { name: 'Log in' })).not.toBeInTheDocument()
    expect(screen.getByText('the login page')).toBeInTheDocument()
  })

  // /status wears this chrome for everybody, so a signed-in visitor can land
  // here. Inviting them to sign in again would be a lie, and no control at all
  // would strand them outside the app.
  it('offers the way back to somebody already signed in', () => {
    landingAt({ status: 'authenticated' })

    expect(screen.queryByRole('link', { name: 'Log in' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to easydnd' })).toHaveAttribute('href', '/')
  })
})
