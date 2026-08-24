import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import {
  dismissed,
  fakeAuthenticator,
  fakeOptions,
  removeAuthenticator,
} from '@/test/webauthn'

import { AuthProvider } from './AuthProvider'
import { useAuth } from './state'

function respond(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

const account = {
  id: 'abc',
  display_name: 'Alice',
  created_at: '2026-01-01T00:00:00Z',
  credentials: [],
  identities: [],
  anonymous: false,
}

const guest = {
  id: 'anon:xyz',
  display_name: 'Guest',
  created_at: '2026-01-01T00:00:00Z',
  credentials: [],
  identities: [],
  anonymous: true,
}

function Probe() {
  const { status, user, error, providers, signOut, signInAsGuest, signInOrRegister } = useAuth()
  return (
    <div>
      <span data-testid="status">{status}</span>
      <span data-testid="user">{user?.display_name ?? '-'}</span>
      <span data-testid="anonymous">{String(user?.anonymous ?? false)}</span>
      <span data-testid="error">{error ?? '-'}</span>
      <span data-testid="providers">{providers.map((p) => p.id).join(',') || '-'}</span>
      <button onClick={() => void signOut()}>sign out</button>
      <button onClick={() => void signInAsGuest()}>guest</button>
      <button onClick={() => void signInOrRegister()}>passkey</button>
    </div>
  )
}

function renderProvider() {
  return render(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
  removeAuthenticator()
  // The auth_error tests navigate; leaving that behind would leak a query
  // string into every test that ran after them.
  window.history.replaceState(null, '', '/')
})

describe('AuthProvider', () => {
  it('reports the account when the server recognises the cookie', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(200, { user: account })))
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))
    expect(screen.getByTestId('user')).toHaveTextContent('Alice')
  })

  it('reports anonymous on a 401, which is the only thing that signs someone out', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        respond(401, { error: { code: 'unauthenticated', message: 'no session' } }),
      ),
    )
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('anonymous'))
  })

  // Being unable to ask is not the same as being told no. Treating it as a
  // sign-out would eject people whenever the network dropped, and every time
  // an installed PWA opened offline.
  it('reports offline, not anonymous, when the server cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network down')))
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('offline'))
  })

  // Signing out has to work when the session is already unusable -- which is
  // exactly when someone reaches for it.
  it('signs out locally even when the logout request fails', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(respond(200, { user: account }))
      .mockRejectedValueOnce(new TypeError('network down'))
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))

    await userEvent.click(screen.getByRole('button', { name: 'sign out' }))
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('anonymous'))
  })

  it('asks who is signed in exactly once on mount', async () => {
    const fetchMock = vi.fn().mockResolvedValue(respond(200, { user: account }))
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))

    // Counted by endpoint rather than in total: mounting also asks which
    // sign-in providers exist, and a bare call count would turn adding any
    // future bootstrap request into a failure here.
    const sessionCalls = fetchMock.mock.calls.filter(
      ([url]) => (url as string) === '/v1/auth/me',
    )
    expect(sessionCalls).toHaveLength(1)

    const [, init] = sessionCalls[0] as [string, RequestInit]
    // Without this the cookie is never sent and everybody is permanently
    // signed out.
    expect(init.credentials).toBe('same-origin')
  })

  // A guest session is established by one POST rather than a ceremony, but it
  // lands in exactly the same state: authenticated, with a user attached.
  it('adopts a guest session from a single request', async () => {
    // Routed by URL rather than by call order: the provider list is fetched on
    // mount too, and a mockResolvedValueOnce chain would hand its response to
    // whichever request happened to go second.
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if ((url as string).includes('/v1/auth/anonymous')) {
        return Promise.resolve(respond(200, { user: guest }))
      }
      return Promise.resolve(
        respond(401, { error: { code: 'unauthenticated', message: 'no session' } }),
      )
    })
    vi.stubGlobal('fetch', fetchMock)
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('anonymous'))
    await userEvent.click(screen.getByRole('button', { name: 'guest' }))

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))
    expect(screen.getByTestId('user')).toHaveTextContent('Guest')
    // The flag is what every guest-aware surface branches on downstream.
    expect(screen.getByTestId('anonymous')).toHaveTextContent('true')

    // Found by URL rather than by index: mounting also asks which sign-in
    // providers exist, so the guest POST is no longer reliably call [1].
    const guestCall = fetchMock.mock.calls.find(([url]) =>
      (url as string).includes('/v1/auth/anonymous'),
    ) as [string, RequestInit] | undefined
    expect(guestCall).toBeDefined()
    expect(guestCall?.[1].method).toBe('POST')
  })

  // The provider turns any failure into a sentence rather than leaving the
  // page stuck on a spinner -- the guest path gets that for free by sharing
  // the ceremony flows' plumbing, and this is what pins it.
  it('reports a failed guest sign-in without signing anyone in', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string) => {
        if ((url as string).includes('/v1/auth/anonymous')) {
          return Promise.resolve(respond(500, { error: { code: 'server_error', message: 'nope' } }))
        }
        return Promise.resolve(
          respond(401, { error: { code: 'unauthenticated', message: 'no session' } }),
        )
      }),
    )
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('anonymous'))
    await userEvent.click(screen.getByRole('button', { name: 'guest' }))

    await waitFor(() => expect(screen.getByTestId('user')).toHaveTextContent('-'))
    expect(screen.getByTestId('status')).toHaveTextContent('anonymous')
  })

  // The provider list decides whether a "Continue with Google" button is drawn
  // at all, so a deployment that configured none must produce none.
  it('exposes the providers the server offers', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url === '/v1/auth/providers') {
        return Promise.resolve(respond(200, { providers: [{ id: 'google', name: 'Google' }] }))
      }
      return Promise.resolve(respond(401, { error: { code: 'unauthenticated', message: 'no' } }))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(screen.getByTestId('providers')).toHaveTextContent('google'))
  })

  // A deployment with no external provider must still sign people in with a
  // passkey, so a failing provider list is not allowed to break the bootstrap.
  it('survives a failing provider list', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url === '/v1/auth/providers') return Promise.reject(new TypeError('network down'))
      return Promise.resolve(respond(200, { user: account }))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))
    expect(screen.getByTestId('providers')).toHaveTextContent('-')
  })

  // The callback has no HTML to render, so a failed federated sign-in comes
  // back as a redirect carrying a code. It has to become a sentence, and the
  // code has to leave the URL so a reload does not resurrect it.
  it('reports and scrubs an auth_error left by the SSO callback', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(401, { error: { code: 'x' } })))
    window.history.replaceState(null, '', '/?auth_error=access_denied&keep=1')

    renderProvider()

    await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('cancelled'))
    expect(window.location.search).toBe('?keep=1')
  })

  // The link flow's regression: the person is still signed in, so the bootstrap
  // session lookup succeeds -- and if adopting that account cleared the error,
  // a failed "Connect Google" would report nothing at all. The message has to
  // survive a successful refresh landing after it.
  it('keeps the auth_error when the session is still valid', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(200, { user: account })))
    window.history.replaceState(null, '', '/account?auth_error=sign_in_failed')

    renderProvider()

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))
    expect(screen.getByTestId('error')).toHaveTextContent('could not be completed')
  })

  // An unrecognised code must not reach the screen: text rendered from a query
  // parameter is a way to put chosen words on somebody else's page.
  it('does not render an unknown auth_error verbatim', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(401, { error: { code: 'x' } })))
    window.history.replaceState(null, '', '/?auth_error=Your+account+is+suspended,+call+555')

    renderProvider()

    // The generic sentence, not the attacker's.
    await waitFor(() =>
      expect(screen.getByTestId('error')).toHaveTextContent('could not be completed'),
    )
    expect(screen.getByTestId('error')).not.toHaveTextContent('555')
    expect(screen.getByTestId('error')).not.toHaveTextContent('suspended')
  })
  // --- the one passkey button ---

  // Signed out, and every request 401s until a ceremony finishes. Each test
  // below overrides the endpoints it cares about.
  function signedOutFetch(routes: Record<string, unknown>) {
    return vi.fn().mockImplementation((url: string) => {
      for (const [path, body] of Object.entries(routes)) {
        if (url.includes(path)) return Promise.resolve(respond(200, body))
      }
      return Promise.resolve(
        respond(401, { error: { code: 'unauthenticated', message: 'no session' } }),
      )
    })
  }

  it('signs in with the passkey the browser offers, and registers nothing', async () => {
    const authenticator = fakeAuthenticator()
    const fetchMock = signedOutFetch({
      '/v1/auth/login/begin': fakeOptions,
      '/v1/auth/login/finish': { user: account },
    })
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('anonymous'))
    await userEvent.click(screen.getByRole('button', { name: 'passkey' }))

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))
    expect(screen.getByTestId('user')).toHaveTextContent('Alice')
    // The half that matters: somebody who already has an account must not be
    // walked into making a second one.
    expect(authenticator.create).not.toHaveBeenCalled()
    expect(called(fetchMock, '/v1/auth/register/begin')).toBe(false)
  })

  // The whole reason there is one button rather than two. The browser refuses
  // to say whether a passkey exists, so the only way to find out is to ask for
  // one and treat an empty-handed picker as "there was nothing to pick".
  it('creates an account when the picker offers nothing', async () => {
    const authenticator = fakeAuthenticator()
    authenticator.get.mockRejectedValue(dismissed())
    const fetchMock = signedOutFetch({
      '/v1/auth/login/begin': fakeOptions,
      '/v1/auth/register/begin': fakeOptions,
      '/v1/auth/register/finish': { user: account },
    })
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('anonymous'))
    await userEvent.click(screen.getByRole('button', { name: 'passkey' }))

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('authenticated'))
    expect(called(fetchMock, '/v1/auth/register/begin')).toBe(true)
    expect(called(fetchMock, '/v1/auth/register/finish')).toBe(true)
    // The subtle part: the sign-in that failed on the way here is discarded,
    // not reported. One press produces one outcome, and this one worked.
    expect(screen.getByTestId('error')).toHaveTextContent('-')
  })

  // The fallback is for "there was no passkey", not for "the server is down".
  // Registering after a 500 would prompt for a passkey that cannot be stored,
  // and would report a sign-up failure to somebody who asked to sign in.
  it('does not try to register when the server is the problem', async () => {
    const authenticator = fakeAuthenticator()
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if ((url as string).includes('/v1/auth/login/begin')) {
        return Promise.resolve(respond(500, { error: { code: 'server_error', message: 'nope' } }))
      }
      // Registration would work perfectly if it were attempted, so that it is
      // not attempted is the only thing distinguishing this from a fallback.
      if ((url as string).includes('/v1/auth/register/begin')) {
        return Promise.resolve(respond(200, fakeOptions))
      }
      if ((url as string).includes('/v1/auth/register/finish')) {
        return Promise.resolve(respond(200, { user: account }))
      }
      return Promise.resolve(
        respond(401, { error: { code: 'unauthenticated', message: 'no session' } }),
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('anonymous'))
    await userEvent.click(screen.getByRole('button', { name: 'passkey' }))

    await waitFor(() => expect(screen.getByTestId('error')).not.toHaveTextContent('-'))
    expect(screen.getByTestId('status')).toHaveTextContent('anonymous')
    expect(called(fetchMock, '/v1/auth/register/begin')).toBe(false)
    expect(authenticator.create).not.toHaveBeenCalled()
    expect(authenticator.get).not.toHaveBeenCalled()
  })
})

/** Whether the mock was asked for a path, whatever else it was asked for. */
function called(fetchMock: ReturnType<typeof vi.fn>, path: string): boolean {
  return fetchMock.mock.calls.some(([url]) => (url as string).includes(path))
}
