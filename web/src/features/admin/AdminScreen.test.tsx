import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { AdminOnly } from '@/routes/AdminOnly'
import { apiPath } from '@/test/api'
import { testAccount, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { AdminScreen } from './AdminScreen'

const player = (n: number) => ({
  id: `usr_${n}`, display_name: `Player ${n}`, created_at: '2026-01-01T00:00:00Z', passkeys: 1, anonymous: false,
})
const character = (n: number) => ({
  id: `chr_${n}`, name: `Hero ${n}`, level: 1, classes: [{ class: 'wizard', level: 1 }],
  owner: 'usr_1', owner_name: 'Player 1', public: false, revision: 3,
})

/** Answers both listings two rows at a time out of five, and records every URL. */
function stubFetch(): URL[] {
  const calls: URL[] = []
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://test')
    calls.push(url)
    const offset = Number(url.searchParams.get('offset'))
    const body = apiPath(url.pathname) === '/v1/admin/players'
      ? { players: [player(offset + 1), player(offset + 2)], total: 5 }
      : { characters: [character(offset + 1), character(offset + 2)], total: 5 }
    return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }))
  return calls
}

function renderAdmin(admin: boolean, entry = '/admin') {
  return renderAt('desktop', withAuth(
    { user: { ...testAccount, admin } },
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="/admin" element={<AdminOnly><AdminScreen /></AdminOnly>} />
      </Routes>
    </MemoryRouter>,
  ))
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('AdminScreen', () => {
  it('is the not-found page to anybody but a superadmin, and asks the server nothing', async () => {
    const calls = stubFetch()
    renderAdmin(false)

    expect(screen.queryByRole('tab', { name: 'Players' })).not.toBeInTheDocument()
    expect(calls).toHaveLength(0)
  })

  it('lists players, appends the next page, and filters through the URL', async () => {
    const user = setupUser()
    const calls = stubFetch()
    renderAdmin(true)

    await waitFor(() => expect(screen.getByText('Player 1')).toBeInTheDocument())
    expect(screen.getByText('5 players')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Load more' }))
    await waitFor(() => expect(screen.getByText('Player 4')).toBeInTheDocument())
    expect(screen.getByText('Player 1')).toBeInTheDocument()
    expect(calls.at(-1)?.searchParams.get('offset')).toBe('2')

    await user.type(screen.getByRole('textbox', { name: 'Name, email or ID' }), 'bob')
    await waitFor(() => expect(calls.at(-1)?.searchParams.get('q')).toBe('bob'))
    expect(calls.at(-1)?.searchParams.get('offset')).toBe('0')
  })

  it('opens a player\'s characters from their row, and a character\'s sheet from its own', async () => {
    const user = setupUser()
    const calls = stubFetch()
    renderAdmin(true)

    await user.click(await screen.findByRole('link', { name: 'Player 1' }))

    const hero = await screen.findByRole('link', { name: 'Hero 1' })
    expect(hero).toHaveAttribute('href', '/shared/chr_1')
    expect(calls.at(-1)?.pathname).toBe('/v1/admin/characters')
    expect(calls.at(-1)?.searchParams.get('owner')).toBe('usr_1')
    expect(screen.getByRole('textbox', { name: 'Owner: name, email or ID' })).toHaveValue('usr_1')
    expect(screen.getAllByText('Wizard 1')).toHaveLength(2)
  })
})
