import { screen, waitFor } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { readInviteToken } from '@/features/groups'
import type { AuthState } from '@/lib/auth'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { JoinRoute } from './JoinRoute'

const TOKEN = 'an-invitation'

function setHash(token: string) {
  window.location.hash = token === '' ? '' : `#${token}`
}

function stubPreview() {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            group_id: 'grp_1',
            group_name: 'Wednesday Night',
            role: 'player',
            invited_by: 'Olive',
            expires_at: '',
            already_member: false,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
    ),
  )
}

function renderJoin(state: Partial<AuthState>) {
  const router = createMemoryRouter(
    [
      { path: '/groups/join', element: <JoinRoute /> },
      { path: '/login', element: <p>the login page</p> },
    ],
    { initialEntries: ['/groups/join'] },
  )
  return renderAt('desktop', withAuth(state, <RouterProvider router={router} />))
}

beforeEach(() => {
  window.sessionStorage.clear()
  setHash('')
  stubPreview()
})

afterEach(() => {
  vi.unstubAllGlobals()
  window.sessionStorage.clear()
  setHash('')
})

/**
 * The regression this route exists for.
 *
 * An invitation link is the one deep link that routinely arrives at somebody
 * with no account. Wrapped in `<Private>`, the screen underneath never mounted
 * for exactly those people, so whatever it did to save the token ran only for
 * the ones who did not need it -- and the fragment was gone the moment they
 * pressed "Log in".
 */
describe('a signed-out visitor following an invitation', () => {
  it('saves the token before anything can navigate away', () => {
    setHash(TOKEN)
    renderJoin({ status: 'anonymous', user: null })

    setHash('')
    expect(readInviteToken()).toBe(TOKEN)
  })

  it('is told an invitation is waiting, rather than shown a bare landing page', () => {
    setHash(TOKEN)
    renderJoin({ status: 'anonymous', user: null })

    expect(screen.getByText('You have been invited to a group')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Log in to join' })).toBeInTheDocument()
  })

  it('says so when the link carries no invitation at all', () => {
    renderJoin({ status: 'anonymous', user: null })

    expect(screen.getByText('No invitation')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Log in to join' })).not.toBeInTheDocument()
  })
})

describe('coming back signed in', () => {
  // The trip through Google drops the fragment: it never reaches a server, and
  // the one we do send refuses a return_to containing '#'. What survives is
  // what was saved on the way out.
  it('uses the saved token when the fragment is gone', async () => {
    setHash(TOKEN)
    renderJoin({ status: 'anonymous', user: null }).unmount()

    setHash('')
    renderJoin({})

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Join group' })).toBeInTheDocument()
  })

  it('shows the invitation straight away when the fragment is still there', async () => {
    setHash(TOKEN)
    renderJoin({})

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.getByText(/Olive invited you/)).toBeInTheDocument()
  })
})
