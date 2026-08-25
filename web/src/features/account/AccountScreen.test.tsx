import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import type { SessionUser } from '@/lib/api'
import type { AuthState } from '@/lib/auth'
import { testAccount, testGuest, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { AccountScreen } from './AccountScreen'

const google = { id: 'google', name: 'Google' }

const passkey = {
  id: 'cred-1',
  created_at: '2026-01-01T00:00:00Z',
  last_used_at: '2026-01-02T00:00:00Z',
  backed_up: true,
}

const identity = {
  provider: 'google',
  subject: 'sub-1',
  email: 'alice@example.com',
  created_at: '2026-01-01T00:00:00Z',
  last_used_at: '2026-01-02T00:00:00Z',
}

/** An account holding exactly the ways in a test names, and no others. */
function accountWith(ways: Partial<SessionUser>): SessionUser {
  return { ...testAccount, ...ways }
}

function accountAt(state: Partial<AuthState> = {}) {
  return renderAt('desktop', withAuth(state, <AccountScreen />))
}

describe('AccountScreen', () => {
  // A guest has no account, so the page has no inventory to draw: one alert
  // saying so is the whole screen.
  it('tells a guest there is nothing to manage', () => {
    accountAt({ user: testGuest })

    expect(screen.getByText('There is no account to manage')).toBeInTheDocument()
    expect(screen.getByText('You are playing as a guest.')).toBeInTheDocument()
    expect(screen.queryByText('Passkeys')).not.toBeInTheDocument()
    expect(screen.queryByText('Connected accounts')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('lists both ways in when the account has both', () => {
    accountAt({
      user: accountWith({ credentials: [passkey], identities: [identity] }),
      providers: [google],
    })

    expect(screen.getByText('Passkeys')).toBeInTheDocument()
    expect(screen.getByText('Connected accounts')).toBeInTheDocument()
    expect(screen.getByText('Synced')).toBeInTheDocument()
    expect(screen.getByText('alice@example.com')).toBeInTheDocument()
  })

  // Nothing on this screen adds a passkey, so a heading over an empty list
  // would be a section that can never fill.
  it('draws no Passkeys section for an account reached through a provider', () => {
    accountAt({ user: accountWith({ identities: [identity] }), providers: [google] })

    expect(screen.queryByText('Passkeys')).not.toBeInTheDocument()
    expect(screen.getByText('Connected accounts')).toBeInTheDocument()
  })

  // The card that has nothing in it and nothing to offer is the one that goes:
  // no provider configured means no identity to list and none to connect.
  it('draws no Connected accounts section when the deployment offers none', () => {
    accountAt({ user: accountWith({ credentials: [passkey] }), providers: [] })

    expect(screen.getByText('Passkeys')).toBeInTheDocument()
    expect(screen.queryByText('Connected accounts')).not.toBeInTheDocument()
  })

  // The one case worth being careful about: a passkey-only account has no
  // identities, and it is precisely the account with no recovery path, so the
  // offer has to survive the card's emptiness test.
  it('still offers the connect action to an account with no identities', async () => {
    const linkProvider = vi.fn()
    accountAt({
      user: accountWith({ credentials: [passkey] }),
      providers: [google],
      linkProvider,
    })

    expect(screen.getByText('Connected accounts')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Connect Google' }))

    expect(linkProvider).toHaveBeenCalledWith('google')
  })

  // The server refuses to remove the last way in; the button says so first.
  it('refuses to disconnect the only way in', () => {
    accountAt({ user: accountWith({ identities: [identity] }), providers: [google] })

    expect(screen.getByRole('button', { name: 'Disconnect' })).toBeDisabled()
  })

  it('offers to disconnect a provider when a passkey remains', async () => {
    const unlinkProvider = vi.fn(async () => true)
    accountAt({
      user: accountWith({ credentials: [passkey], identities: [identity] }),
      providers: [google],
      unlinkProvider,
    })

    await userEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    await userEvent.click(screen.getByRole('button', { name: 'Disconnect' }))

    expect(unlinkProvider).toHaveBeenCalledWith('google', 'sub-1')
  })
})
