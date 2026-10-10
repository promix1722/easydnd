import { screen, waitFor } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { kindOfLink, readInviteToken } from '@/features/groups'
import type { AuthState } from '@/lib/auth'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { InvitationLink, InvitationRoute } from './InvitationRoute'

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

function renderJoin(state: Partial<AuthState>, at = '/groups/join') {
  const router = createMemoryRouter(
    [
      { path: '/groups/join', element: <InvitationLink kind="group" /> },
      { path: '/characters/receive', element: <InvitationLink kind="character" /> },
      { path: '/invitations', element: <><p>the invitations page</p><InvitationRoute /></> },
      { path: '/login', element: <p>the login page</p> },
    ],
    { initialEntries: [at] },
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

// One page for every invitation: a link lands on it, and the menu opens it
// empty with a field, which is the only way in for an installed app.
describe('the invitations page', () => {
  const asked = () => vi.mocked(fetch).mock.calls.map(([url]) => String(url))

  it('is where a link moves to, whichever of the two addresses it was sent as', async () => {
    setHash(TOKEN)
    renderJoin({}, '/characters/receive')

    expect(await screen.findByText('the invitations page')).toBeInTheDocument()
    await waitFor(() => expect(asked().some((url) => url.includes('/copy-links/preview'))).toBe(true))
    expect(asked().some((url) => url.includes('/invites/preview'))).toBe(false)
  })

  it('takes a pasted link of either kind, and can be asked for another', async () => {
    renderJoin({}, '/invitations')
    const user = setupUser()

    expect(screen.getByRole('button', { name: 'Open invitation' })).toBeDisabled()
    await user.type(screen.getByRole('textbox', { name: 'Invitation link' }), ' https://easydnd.org/groups/join#pasted ')
    await user.click(screen.getByRole('button', { name: 'Open invitation' }))
    expect(await screen.findByRole('button', { name: 'Join group' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Paste another link' }))
    await user.clear(screen.getByRole('textbox', { name: 'Invitation link' }))
    await user.type(screen.getByRole('textbox', { name: 'Invitation link' }), 'https://easydnd.org/characters/receive#other')
    await user.click(screen.getByRole('button', { name: 'Open invitation' }))
    await waitFor(() => expect(asked().some((url) => url.includes('/copy-links/preview'))).toBe(true))
  })

  it('reads the kind out of a bare token', () => {
    const token = (knd: string) => `h.${btoa(JSON.stringify({ knd })).replace(/=+$/, '')}.s`
    expect(kindOfLink(token('copylink'))).toBe('character')
    expect(kindOfLink(token('invite'))).toBe('group')
    expect(kindOfLink('not-a-token')).toBe('group')
  })
})
