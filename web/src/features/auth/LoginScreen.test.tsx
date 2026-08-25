import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { RouterProvider, createMemoryRouter, useLocation } from 'react-router'

import type { AuthState } from '@/lib/auth'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { LoginScreen } from './LoginScreen'

/**
 * Renders where it landed, so a test can assert the whole URL came back.
 *
 * The router's location, not window.location: a memory router keeps its own
 * and never touches jsdom's, so reading the global would only ever report the
 * '/' the test file started at.
 */
function JoinTarget() {
  const { pathname, search, hash } = useLocation()
  return <p>{`joined at ${pathname}${search}${hash}`}</p>
}

function loginAt(
  state: Partial<AuthState> = {},
  from?: string | { pathname: string; search?: string; hash?: string },
) {
  const origin = typeof from === 'string' ? { pathname: from } : from
  const router = createMemoryRouter(
    [
      { path: '/login', element: <LoginScreen /> },
      { path: '/', element: <p>the app</p> },
      { path: '/characters/new', element: <p>the character builder</p> },
      { path: '/groups/join', element: <JoinTarget /> },
    ],
    { initialEntries: [{ pathname: '/login', state: origin ? { from: origin } : null }] },
  )
  return renderAt(
    'desktop',
    withAuth({ status: 'anonymous', user: null, ...state }, <RouterProvider router={router} />),
  )
}

// jsdom has no WebAuthn. The passkey options are gated on it, so a test about
// them has to stub it; the guest option deliberately is not.
beforeEach(() => {
  vi.stubGlobal('PublicKeyCredential', function PublicKeyCredential() {})
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('LoginScreen', () => {
  it('offers every way in, and asks for nothing', () => {
    loginAt()

    expect(screen.getByRole('button', { name: 'Continue with a passkey' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue as a guest' })).toBeInTheDocument()
    // The assertion that pins the point of this screen: there is no field on
    // it. A display name was the last text this app ever asked anybody for.
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  // Signing in and signing up are the same press. Two passkey buttons would ask
  // the visitor which they are, and the browser will not tell the page.
  it('offers one passkey button, not two', () => {
    loginAt()

    expect(screen.getAllByRole('button', { name: /passkey/i })).toHaveLength(1)
  })

  // The passkey is discoverable, so the browser picks it: there is nothing to
  // type and no form in the way.
  it('signs in without asking for anything first', async () => {
    const signInOrRegister = vi.fn(async () => true)
    loginAt({ signInOrRegister })

    await userEvent.click(screen.getByRole('button', { name: 'Continue with a passkey' }))

    expect(signInOrRegister).toHaveBeenCalled()
  })

  it('starts a guest session on one click', async () => {
    const signInAsGuest = vi.fn(async () => true)
    loginAt({ signInAsGuest })

    await userEvent.click(screen.getByRole('button', { name: 'Continue as a guest' }))

    expect(signInAsGuest).toHaveBeenCalled()
  })

  it('surfaces a failed attempt', () => {
    loginAt({ error: 'The passkey prompt was dismissed.' })

    expect(screen.getByText('The passkey prompt was dismissed.')).toBeInTheDocument()
  })

  // A guest session needs no authenticator, so this browser is not locked out
  // the way it was when passkeys were the only way in.
  it('still leads somewhere without WebAuthn', () => {
    vi.unstubAllGlobals()
    loginAt()

    expect(screen.getByRole('button', { name: 'Continue as a guest' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /passkey/i })).not.toBeInTheDocument()
    expect(screen.getByText(/This browser cannot use passkeys/i)).toBeInTheDocument()
  })

  // The header records where the visitor was, which is what replaces the old
  // "the URL never changes" property now that the way in is a page.
  it('returns to the page the visitor came from', async () => {
    loginAt({ signInAsGuest: async () => true }, '/characters/new')

    await userEvent.click(screen.getByRole('button', { name: 'Continue as a guest' }))

    expect(await screen.findByText('the character builder')).toBeInTheDocument()
  })

  it('sends an already signed-in visitor to the app', () => {
    loginAt({ status: 'authenticated' })

    expect(screen.getByText('the app')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Continue as a guest' })).not.toBeInTheDocument()
  })

  // --- external providers ---

  const google = { id: 'google', name: 'Google' }

  it('offers a configured provider alongside the other ways in', () => {
    loginAt({ providers: [google] })

    expect(screen.getByRole('button', { name: 'Continue with Google' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue with a passkey' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue as a guest' })).toBeInTheDocument()
  })

  // A button for a provider the deployment never configured is a dead end: the
  // server would answer the redirect with "unknown sign-in provider".
  it('offers nothing external when none is configured', () => {
    loginAt({ providers: [] })

    // Counted rather than matched loosely: the passkey button shares its
    // "Continue with..." shape with the provider buttons, and what is being
    // asserted here is that nothing on the page leaves for a provider.
    expect(screen.getAllByRole('button')).toHaveLength(2)
    expect(screen.getByRole('button', { name: 'Continue with a passkey' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue as a guest' })).toBeInTheDocument()
  })

  it('leaves for the provider when the button is pressed', async () => {
    const signInWith = vi.fn()
    loginAt({ providers: [google], signInWith })

    await userEvent.click(screen.getByRole('button', { name: 'Continue with Google' }))

    expect(signInWith).toHaveBeenCalledWith('google')
  })

  // The page must still lead somewhere on a browser with no WebAuthn, and a
  // provider is one of the two options that work there.
  it('still offers the provider without passkey support', () => {
    vi.unstubAllGlobals()
    loginAt({ providers: [google] })

    expect(screen.getByRole('button', { name: 'Continue with Google' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue as a guest' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /passkey/i })).not.toBeInTheDocument()
  })

  // Whatever the callback redirected back with is rendered here, because this
  // is where somebody lands after a failed federated sign-in.
  it('shows the error a failed provider sign-in left behind', () => {
    loginAt({ providers: [google], error: 'Sign-in was cancelled.' })

    expect(screen.getByText('Sign-in was cancelled.')).toBeInTheDocument()
  })
})

/**
 * Signing in has to return somebody to the *whole* place they came from.
 *
 * This used to keep only the path. An invitation link is entirely fragment --
 * `/groups/join#<token>` -- so dropping it returned the one visitor who most
 * needed the deep link to the one screen that cannot work without it.
 */
describe('coming back to where you were', () => {
  it('keeps the search and the fragment, not just the path', async () => {
    loginAt({}, { pathname: '/groups/join', search: '?a=1', hash: '#a-token' })

    await userEvent.click(screen.getByRole('button', { name: /guest/i }))

    expect(await screen.findByText('joined at /groups/join?a=1#a-token')).toBeInTheDocument()
  })

  // The state is history, and history is attacker-reachable: a hand-written
  // link could put anything in it.
  it('ignores a return that is not a path of ours', async () => {
    loginAt({}, { pathname: 'https://example.test/steal' })

    await userEvent.click(screen.getByRole('button', { name: /guest/i }))

    expect(await screen.findByText('the app')).toBeInTheDocument()
  })
})
