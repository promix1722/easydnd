import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import type { AuthState } from '@/lib/auth'
import { testGuest, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { HomeRoute } from './HomeRoute'

function credential(backedUp: boolean) {
  return {
    id: 'cred-1',
    created_at: '2026-01-01T00:00:00Z',
    last_used_at: '2026-01-01T00:00:00Z',
    backed_up: backedUp,
  }
}

function homeAt(state: Partial<AuthState>) {
  return renderAt(
    'desktop',
    withAuth(
      state,
      <MemoryRouter>
        <HomeRoute />
      </MemoryRouter>,
    ),
  )
}

/**
 * `/` is one URL with two faces. These pin that the app content is genuinely
 * absent from the document when signed out -- not merely hidden, which would
 * leak it to anyone who opened the inspector.
 */
describe('HomeRoute', () => {
  // The sign-in controls themselves live in the header, in LandingShell, and
  // the page below it is deliberately wordless -- so the mark is what there is
  // to assert. That it is reachable *by name* is the accessibility contract
  // standing in for the headline this page used to open with.
  it('renders the landing page when anonymous', () => {
    homeAt({ status: 'anonymous' })

    expect(screen.getByRole('img', { name: 'easydnd' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Characters' })).not.toBeInTheDocument()
  })

  it('renders the party when authenticated', () => {
    homeAt({
      status: 'authenticated',
      user: {
        id: 'abc',
        display_name: 'Alice',
        created_at: '2026-01-01T00:00:00Z',
        anonymous: false,
        credentials: [credential(true), credential(true)],
        identities: [],
      },
    })

    expect(screen.getByRole('heading', { name: 'Characters' })).toBeInTheDocument()
    expect(screen.queryByRole('img', { name: 'easydnd' })).not.toBeInTheDocument()
  })

  // How many ways in an account has belongs to the account screen, next to the
  // controls that change it -- the home page says nothing about it.
  it('leaves an account holder to their characters', () => {
    homeAt({
      status: 'authenticated',
      user: {
        id: 'abc',
        display_name: 'Alice',
        created_at: '2026-01-01T00:00:00Z',
        anonymous: false,
        credentials: [credential(false)],
        identities: [],
      },
    })

    expect(screen.queryByText(/does not sync anywhere/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/You are playing as a guest/i)).not.toBeInTheDocument()
  })

  // System status is a deploy diagnostic and lives on /status alone, so
  // neither face of the home page carries it.
  it('leaves system status to its own page', () => {
    const { unmount } = homeAt({ status: 'anonymous' })
    expect(screen.queryByRole('heading', { name: 'System status' })).not.toBeInTheDocument()
    unmount()

    homeAt({ status: 'authenticated' })
    expect(screen.queryByRole('heading', { name: 'System status' })).not.toBeInTheDocument()
  })

  // A guest session is the one case the home page still speaks up about: it
  // ends without warning and takes the characters with it.
  it('tells a guest their work is not saved', () => {
    homeAt({ status: 'authenticated', user: testGuest })

    expect(screen.getByText(/You are playing as a guest/i)).toBeInTheDocument()
  })
})
