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
    // Nowhere to navigate to yet: every section of the app is somebody's.
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
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
