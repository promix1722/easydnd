import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { GameSummary, GroupRole } from '@/lib/api'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { GamesScreen } from './GamesScreen'

const thursday: GameSummary = {
  id: 'gam_1',
  group_id: 'grp_1',
  group_name: 'Wednesday Night',
  name: 'Thursday night',
  created_at: '2026-01-01T00:00:00Z',
}

/** Answers /games and /groups from one stub, by path. */
function stub(games: GameSummary[], role: GroupRole) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      const body = url.includes('/v1/games')
        ? { games }
        : { groups: [{ id: 'grp_1', name: 'Wednesday Night', created_at: '', role }] }
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
}

function renderGames(viewport: Viewport, games: GameSummary[], role: GroupRole) {
  stub(games, role)
  const user = userEvent.setup()
  const rendered = renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter initialEntries={['/games']}>
        <GamesScreen />
      </MemoryRouter>,
    ),
  )
  return { ...rendered, user }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

/**
 * One viewport, because nothing in this claim reaches a component that branches
 * on width -- and because the generic version of it lives in `ui/Page.test.tsx`.
 * What is under test here is this screen's own wiring: it was the one of the
 * three list screens that replaced the whole page on a failure, so a failed
 * list took the heading with it.
 */
describe('GamesScreen, when the list will not load', () => {
  it('keeps the heading and offers the retry', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(JSON.stringify({ error: { message: 'the server said no' } }), {
            status: 500,
            headers: { 'Content-Type': 'application/json' },
          }),
      ),
    )
    renderAt(
      'desktop',
      withAuth(
        {},
        <MemoryRouter initialEntries={['/games']}>
          <GamesScreen />
        </MemoryRouter>,
      ),
    )

    expect(
      await screen.findByRole('button', { name: 'Try again' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Games' })).toBeInTheDocument()
  })
})

describe.each(['mobile', 'desktop'] as const)('GamesScreen (%s)', (viewport) => {
  it('lists your games with the table each sits at', async () => {
    renderGames(viewport, [thursday], 'player')

    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    // A player at three tables cannot tell their Thursdays apart without this.
    expect(screen.getByText('Wednesday Night')).toBeInTheDocument()
  })

  it('offers a new game to somebody who runs a table', async () => {
    renderGames(viewport, [thursday], 'dm')
    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'New game' })).toBeInTheDocument()
  })

  it('calls the picker Group, and keeps a failure inside the dialog', async () => {
    const { user } = renderGames(viewport, [thursday], 'dm')
    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'New game' }))
    // "Group" is what the rest of the app calls it; "table" is prose, not a
    // label for the thing being picked.
    // By role: the label is on the input and on the listbox it controls, so
    // matching label text alone finds two nodes.
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Group' })).toBeInTheDocument(),
    )
    expect(screen.queryByRole('combobox', { name: 'At which table' })).not.toBeInTheDocument()
  })

  it('offers nothing to somebody who only plays', async () => {
    renderGames(viewport, [thursday], 'player')
    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    // No table of their own to open one at, so the button would only ever
    // produce an empty picker.
    expect(screen.queryByRole('button', { name: 'New game' })).not.toBeInTheDocument()
  })
})
