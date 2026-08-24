import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'

import {
  ApiError,
  TransportError,
  beginLogin,
  beginRegistration,
  finishLogin,
  finishRegistration,
  getSession,
  listProviders,
  signOut as signOutRequest,
  ssoLinkUrl,
  ssoStartUrl,
  startGuestSession,
  unlinkProvider as unlinkProviderRequest,
  type AuthProviderInfo,
  type SessionUser,
} from '@/lib/api'
import {
  createCredential,
  describeCeremonyFailure,
  getCredential,
  isCeremonyDismissed,
  type CeremonyOptions,
  type CeremonyResponse,
} from '@/lib/webauthn'

import { AuthContext, type AuthState, type AuthStatus } from './state'

/**
 * The query parameter the SSO callback lands on after a failure.
 *
 * The API has no HTML to render, and a JSON error body at the end of a
 * top-level navigation would replace the application with a page of braces --
 * so failures come back as a redirect carrying a coarse code. It is a code
 * rather than a sentence on purpose: text rendered from a query parameter is a
 * way to put chosen words on somebody else's page.
 */
const AUTH_ERROR_PARAM = 'auth_error'

const AUTH_ERRORS: Record<string, string> = {
  access_denied: 'Sign-in was cancelled.',
  session_expired: 'Your session had expired, so you have been signed out. Sign in and try again.',
  unknown_provider: 'That sign-in method is not available.',
  sign_in_failed: 'That sign-in could not be completed. Please try again.',
}

/**
 * Owns the answer to "who is using this app".
 *
 * The session lives in an HttpOnly cookie, so the browser holds it and script
 * cannot read it. That means this provider has exactly one way to find out:
 * ask the server. It does so once on mount, and again whenever a ceremony
 * changes the answer.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('loading')
  const [user, setUser] = useState<SessionUser | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [providers, setProviders] = useState<AuthProviderInfo[]>([])

  // A ceremony outlives a re-render, and resolving one after the component
  // went away must not set state on a corpse.
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  const adopt = useCallback((account: SessionUser) => {
    if (!mounted.current) return
    setUser(account)
    setStatus('authenticated')
    // Deliberately does NOT clear the error. Every attempt clears it on the
    // way in, which is the right moment; clearing it on success here would
    // erase the message a failed SSO callback just delivered, because the
    // bootstrap refresh resolves after it. That is exactly the link flow --
    // still signed in, so the session lookup succeeds -- and it would have
    // meant a failed link reporting nothing at all.
  }, [])

  const refresh = useCallback(async () => {
    try {
      adopt(await getSession())
    } catch (cause) {
      if (!mounted.current) return
      if (cause instanceof ApiError && cause.status === 401) {
        // The server said so. This is the only thing that signs someone out.
        setUser(null)
        setStatus('anonymous')
        return
      }
      if (cause instanceof TransportError) {
        // We could not ask. Showing the landing page here would sign people
        // out every time the network hiccuped -- and in an installed PWA
        // opened offline, that would be every launch.
        setStatus('offline')
        return
      }
      setUser(null)
      setStatus('anonymous')
    }
  }, [adopt])

  useEffect(() => {
    // Synchronising with an external system -- the server is the only thing
    // that knows whether the HttpOnly cookie is still good, and script cannot
    // read it to find out. Nothing is set synchronously here: every state
    // change inside refresh happens after an await.
    // oxlint-disable-next-line react/set-state-in-effect
    void refresh()
  }, [refresh])

  useEffect(() => {
    // Read in an effect rather than as lazy initial state, because it scrubs
    // the URL: a side effect belongs after the render, not during one. It only
    // ever sets, never clears, so StrictMode's second pass -- which finds the
    // parameter already gone -- cannot wipe the message the first one found.
    const message = takeAuthError()
    // oxlint-disable-next-line react/set-state-in-effect
    if (message) setError(message)
  }, [])

  useEffect(() => {
    // Which sign-in buttons exist is a property of the deployment, not of who
    // is looking, so this runs once and does not repeat on sign-in. A failure
    // is not worth reporting: the passkey buttons still work, and the worst
    // case is one fewer option on screen.
    const controller = new AbortController()
    listProviders(controller.signal)
      .then((available) => {
        if (mounted.current) setProviders(available)
      })
      .catch(() => {})
    return () => controller.abort()
  }, [])

  /**
   * Runs one attempt at establishing a session, whatever shape it takes.
   *
   * The busy flag, the error message and the "did the component survive"
   * guard are the interesting part, and they are identical for a three-step
   * passkey ceremony and for a single POST -- so they live here once and the
   * flows below differ only in what they await.
   */
  const runAuth = useCallback(
    async (attempt: () => Promise<SessionUser>): Promise<boolean> => {
      setBusy(true)
      setError(null)
      try {
        adopt(await attempt())
        return true
      } catch (cause) {
        if (!mounted.current) return false
        setError(describeFailure(cause))
        return false
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [adopt],
  )

  /**
   * The passkey button, both halves of it.
   *
   * Sign-in first, because the common case is somebody who already has a
   * passkey and the browser can find it without being told whose it is. If the
   * picker comes back empty-handed, we register instead and the account is
   * created on the spot.
   *
   * Both ceremonies share one runAuth attempt on purpose. Chaining two would be
   * wrong twice over: runAuth's catch would swallow the sign-in failure and
   * report it before registration ever ran, and busy would drop to false
   * between the two prompts -- a button that stops spinning halfway through, in
   * the exact gap where the operating system is about to ask something. One
   * attempt means one spinner and one message.
   */
  const signInOrRegister = useCallback(
    () =>
      runAuth(async () => {
        try {
          return await runOneCeremony(beginLogin, getCredential, finishLogin)
        } catch (cause) {
          // The browser will not say whether a passkey exists for this site, so
          // "the picker ended without an assertion" is the only signal there
          // is. Treating it as "no passkey" is what lets one button mean both
          // things; the price is that a deliberate cancel is followed by a
          // create prompt, which the button's own copy warns about. Anything
          // else -- a 500, a lost connection, a misconfigured relying party --
          // would fail the same way on the way back, so it is not retried as
          // something it is not.
          if (!isCeremonyDismissed(cause)) throw cause
          return await runOneCeremony(beginRegistration, createCredential, finishRegistration)
        }
      }),
    [runAuth],
  )

  // Not a ceremony: there is no authenticator to prompt and nothing to verify,
  // which is the entire appeal of it.
  const signInAsGuest = useCallback(() => runAuth(startGuestSession), [runAuth])

  /**
   * Leaves for the provider.
   *
   * assign rather than replace, so Back returns to the page they started from
   * rather than skipping past it, and a full navigation rather than a router
   * push because the destination is not ours to route to.
   */
  const signInWith = useCallback((provider: string) => {
    setError(null)
    window.location.assign(ssoStartUrl(provider, currentPath()))
  }, [])

  const linkProvider = useCallback((provider: string) => {
    setError(null)
    window.location.assign(ssoLinkUrl(provider, currentPath()))
  }, [])

  const unlinkProvider = useCallback(
    async (provider: string, subject: string): Promise<boolean> => {
      setBusy(true)
      setError(null)
      try {
        adopt(await unlinkProviderRequest(provider, subject))
        return true
      } catch (cause) {
        if (!mounted.current) return false
        setError(describeFailure(cause))
        return false
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [adopt],
  )

  const signOut = useCallback(async () => {
    try {
      await signOutRequest()
    } catch {
      // The cookie may already be unusable, which is exactly when signing out
      // matters most. Drop the local session either way; a stale cookie the
      // server rejects is indistinguishable from none.
    }
    if (!mounted.current) return
    setUser(null)
    setStatus('anonymous')
    setError(null)
  }, [])

  const value = useMemo<AuthState>(
    () => ({
      status,
      user,
      error,
      busy,
      providers,
      signInOrRegister,
      signInAsGuest,
      signInWith,
      linkProvider,
      unlinkProvider,
      signOut,
      refresh,
    }),
    [
      status,
      user,
      error,
      busy,
      providers,
      signInOrRegister,
      signInAsGuest,
      signInWith,
      linkProvider,
      unlinkProvider,
      signOut,
      refresh,
    ],
  )

  return <AuthContext value={value}>{children}</AuthContext>
}

/**
 * Runs one ceremony: fetch options, prompt the authenticator, post the result.
 *
 * Lifted out of the provider because signInOrRegister runs it twice inside a
 * single attempt, and because it closes over nothing -- it is three functions
 * called in order, and it belongs where that is obvious.
 */
async function runOneCeremony(
  begin: () => Promise<CeremonyOptions>,
  prompt: (options: CeremonyOptions) => Promise<CeremonyResponse>,
  finish: (response: CeremonyResponse) => Promise<SessionUser>,
): Promise<SessionUser> {
  return finish(await prompt(await begin()))
}

/**
 * Reads the SSO failure the callback redirected with, and scrubs it from the
 * URL so a reload or a shared link does not resurrect it.
 *
 * The code is looked up in a table rather than shown: only codes this build
 * knows about become words, and an unrecognised one becomes the generic
 * sentence rather than reaching the screen.
 */
function takeAuthError(): string | null {
  if (typeof window === 'undefined') return null

  const url = new URL(window.location.href)
  const code = url.searchParams.get(AUTH_ERROR_PARAM)
  if (code === null) return null

  url.searchParams.delete(AUTH_ERROR_PARAM)
  window.history.replaceState(null, '', url.pathname + url.search + url.hash)

  return AUTH_ERRORS[code] ?? AUTH_ERRORS.sign_in_failed ?? null
}

/** Where to come back to after the round trip through the provider. */
function currentPath(): string {
  if (typeof window === 'undefined') return '/'
  return window.location.pathname + window.location.search
}

/** Turns anything a ceremony can throw into a sentence. */
function describeFailure(cause: unknown): string {
  if (cause instanceof ApiError) {
    // Field errors carry a message already written for a person. Nothing on the
    // passkey path can raise one any more -- there is no field left to get
    // wrong -- but unlinking a provider still can.
    return cause.fields[0]?.message ?? cause.message
  }
  if (cause instanceof TransportError) {
    return 'Could not reach the server. Check your connection and try again.'
  }
  return describeCeremonyFailure(cause).message
}
