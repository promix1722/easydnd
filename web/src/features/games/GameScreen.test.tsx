import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { GameDetail, GroupRole } from '@/lib/api'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { rowsOffering } from '@/test/rows'
import type { Viewport } from '@/test/viewport'

import { GameScreen } from './GameScreen'

function gameAs(role: GroupRole): GameDetail {
  return {
    id: 'gam_1',
    group_id: 'grp_1',
    name: 'Thursday night',
    created_at: '2026-01-01T00:00:00Z',
    role,
    characters: [
      {
        id: 'chr_1',
        owner_id: 'player-1',
        name: 'Ada',
        level: 3,
        classes: [{ class: 'rogue', level: 3 }],
      },
    ],
  }
}

function renderGame(viewport: Viewport, game: GameDetail) {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify(game), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )
  return renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter initialEntries={['/games/gam_1']}>
        <Routes>
          <Route path="/games/:id" element={<GameScreen />} />
          <Route path="/games" element={<div>your games</div>} />
          <Route path="/groups/:id" element={<div>the group</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('GameScreen (%s)', (viewport) => {
  it('shows a DM the roster and the controls that change it', async () => {
    renderGame(viewport, gameAs('dm'))

    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    expect(screen.getByText('Ada')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add character from group' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add my characters' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
    // A row's action, so it is a named button on a desktop and one item inside
    // the row's menu on a phone -- either way, exactly one row offers it.
    expect(rowsOffering(viewport, 'Remove')).toBe(1)
    // The blanket control is gone: seating everyone is ticking Everyone in
    // the group picker, not a second button beside it.
    expect(screen.queryByRole('button', { name: 'Add everyone' })).not.toBeInTheDocument()
  })

  it('says what your rank is at the table it is played at', async () => {
    renderGame(viewport, gameAs('owner'))

    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    // A game is reached from its own section, so the page has to say what you
    // are at its table rather than leaving you to remember.
    expect(screen.getByText('Owner')).toBeInTheDocument()
  })

  it('shows a player the roster and nothing that would come back 403', async () => {
    renderGame(viewport, gameAs('player'))

    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    expect(screen.getByText('Ada')).toBeInTheDocument()
    for (const label of ['Add character from group', 'Add my characters', 'Rename', 'Delete']) {
      expect(screen.queryByRole('button', { name: label })).not.toBeInTheDocument()
    }
    // And no way to unseat anybody: a player's row carries no control at all,
    // at either width.
    expect(rowsOffering(viewport, 'Remove')).toBe(0)
  })
})
