import type { ReactElement } from 'react'

import type { SessionUser } from '@/lib/api'
import { AuthContext, type AuthState } from '@/lib/auth/state'

const account: SessionUser = {
  id: 'abc',
  display_name: 'Alice',
  created_at: '2026-01-01T00:00:00Z',
  credentials: [],
  identities: [],
  anonymous: false,
}

// A guest: a working session with no account behind it. Separate from the
// account above rather than a flag on it, because the components that care
// branch on it and a test reads better naming which one it means.
const guest = {
  id: 'anon:xyz',
  display_name: 'Guest',
  created_at: '2026-01-01T00:00:00Z',
  credentials: [],
  identities: [],
  anonymous: true,
}

/**
 * Wraps a tree in a stubbed auth context.
 *
 * Every component below RootGate assumes it is inside the provider -- useAuth
 * throws otherwise, deliberately, so that a component rendered outside it
 * cannot quietly look signed out. Tests need the same guarantee without
 * standing up a real provider and a fetch mock.
 */
export function withAuth(state: Partial<AuthState>, ui: ReactElement): ReactElement {
  const value: AuthState = {
    status: 'authenticated',
    user: account,
    error: null,
    busy: false,
    providers: [],
    signInOrRegister: async () => true,
    signInAsGuest: async () => true,
    signInWith: () => {},
    linkProvider: () => {},
    unlinkProvider: async () => true,
    signOut: async () => {},
    refresh: async () => {},
    ...state,
  }
  return <AuthContext value={value}>{ui}</AuthContext>
}

/** A stand-in signed-in account, for tests that need to name one. */
export const testAccount = account

/** A stand-in guest session, for the surfaces that treat one differently. */
export const testGuest = guest
