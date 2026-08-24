import { createContext, useContext } from 'react'

import type { AuthProviderInfo, SessionUser } from '@/lib/api'

/**
 * Whether we know who is using the app.
 *
 * A union rather than loose booleans, so "loading but also authenticated" and
 * "signed out with a user still set" are unrepresentable. The distinction
 * between `anonymous` and `offline` matters: the first is a fact the server
 * stated, the second is our own ignorance, and showing a landing page for the
 * second would sign people out every time a train went into a tunnel.
 *
 * Note the vocabulary trap: `anonymous` here means SIGNED OUT, and predates the
 * guest-session feature. A guest is `authenticated` with `user.anonymous` set
 * -- they have a working session, it just names no account. The two senses of
 * the word never appear in the same expression, but they do appear in the same
 * file, so check which one you mean.
 */
export type AuthStatus = 'loading' | 'anonymous' | 'authenticated' | 'offline'

export interface AuthState {
  status: AuthStatus
  /** The signed-in account, or null. Only trustworthy when status is 'authenticated'. */
  user: SessionUser | null
  /** The last ceremony failure, in words worth showing someone. Cleared on the next attempt. */
  error: string | null
  /** True while a passkey ceremony is in flight. */
  busy: boolean

  /**
   * The external sign-in methods this deployment offers, or an empty list.
   *
   * Empty while status is 'loading', and empty for good on a deployment that
   * configured none -- a button drawn for a provider that is not there would
   * be a dead end.
   */
  providers: AuthProviderInfo[]

  /**
   * The one way in with a passkey.
   *
   * Signs in with the passkey the browser offers; if the picker ends without
   * one, creates an account instead -- because the browser will not say which
   * case it is, so trying is the only way to find out. Resolves true when
   * either half worked.
   */
  signInOrRegister: () => Promise<boolean>
  /**
   * Starts a guest session: no passkey, no account, nothing kept. Resolves
   * true when it worked.
   */
  signInAsGuest: () => Promise<boolean>
  /**
   * Leaves for the provider to sign in with it.
   *
   * Returns nothing and never resolves in any useful sense: it is a top-level
   * navigation, so the page it was called from is on its way out.
   */
  signInWith: (provider: string) => void
  /** Leaves for the provider to connect it to the signed-in account. */
  linkProvider: (provider: string) => void
  /** Disconnects an external account. Resolves true when it worked. */
  unlinkProvider: (provider: string, subject: string) => Promise<boolean>
  /** Clears the session. */
  signOut: () => Promise<void>
  /** Re-checks the session with the server. */
  refresh: () => Promise<void>
}

export const AuthContext = createContext<AuthState | null>(null)

/**
 * Reads the auth state.
 *
 * Throws rather than returning a default: a component rendering outside the
 * provider would otherwise look signed out, which is the one wrong answer that
 * fails silently.
 */
export function useAuth(): AuthState {
  const state = useContext(AuthContext)
  if (!state) {
    throw new Error('useAuth must be used inside <AuthProvider>')
  }
  return state
}
