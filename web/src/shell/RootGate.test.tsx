import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router'

import type { AuthState } from '@/lib/auth'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { RootGate } from './RootGate'

function gateAt(viewport: 'mobile' | 'desktop', state: Partial<AuthState>) {
  const router = createMemoryRouter(
    [{ path: '/', element: <RootGate />, children: [{ index: true, element: <p>party</p> }] }],
    { initialEntries: ['/'] },
  )
  return renderAt(viewport, withAuth(state, <RouterProvider router={router} />))
}

describe('RootGate', () => {
  it('shows a loader while the session is still unknown', () => {
    gateAt('desktop', { status: 'loading' })

    expect(screen.getByLabelText('Checking your session')).toBeInTheDocument()
    // The app content must not flash before we know who is looking.
    expect(screen.queryByText('party')).not.toBeInTheDocument()
  })

  it('shows the logged-out chrome with no navigation when anonymous', () => {
    gateAt('desktop', { status: 'anonymous' })

    expect(screen.queryByRole('button', { name: 'Toggle navigation' })).not.toBeInTheDocument()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })

  it('shows the app chrome and the routed content when authenticated', () => {
    gateAt('desktop', { status: 'authenticated' })

    expect(screen.getByRole('button', { name: 'Toggle navigation' })).toBeInTheDocument()
    expect(screen.getByText('party')).toBeInTheDocument()
  })

  it('picks the mobile chrome at a narrow viewport when authenticated', () => {
    gateAt('mobile', { status: 'authenticated' })

    expect(screen.getByRole('tablist')).toBeInTheDocument()
    expect(screen.getByText('party')).toBeInTheDocument()
  })

  // Offline is our ignorance, not a sign-out: showing the landing page here
  // would eject someone whose train went into a tunnel.
  it('offers a retry instead of the landing page when offline', () => {
    gateAt('desktop', { status: 'offline' })

    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
    expect(screen.queryByText('party')).not.toBeInTheDocument()
  })
})
