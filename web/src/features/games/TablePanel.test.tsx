import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { GroupRole, TableCharacter } from '@/lib/api'
import { testAccount, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { TablePanel } from './TablePanel'

const mine: TableCharacter = {
  id: 'chr_1',
  owner_id: testAccount.id,
  name: 'Ada',
  level: 3,
  classes: [{ class: 'rogue', level: 3 }],
}

const theirs: TableCharacter = {
  id: 'chr_2',
  owner_id: 'somebody-else',
  name: 'Bram',
  level: 2,
  classes: [{ class: 'cleric', level: 2 }],
}

function stubTable(characters: TableCharacter[]) {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify({ characters }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )
}

function renderTable(viewport: Viewport, role: GroupRole, characters: TableCharacter[]) {
  stubTable(characters)
  return renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter>
        <TablePanel groupId="grp_1" role={role} />
      </MemoryRouter>,
    ),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('TablePanel (%s)', (viewport) => {
  it('shows every member the whole table, not just their own', async () => {
    renderTable(viewport, 'player', [mine, theirs])

    await waitFor(() => expect(screen.getByText('Ada')).toBeInTheDocument())
    expect(screen.getByText('Bram')).toBeInTheDocument()
  })

  it('offers no way to edit anybody, including yourself', async () => {
    renderTable(viewport, 'owner', [mine, theirs])

    await waitFor(() => expect(screen.getByText('Ada')).toBeInTheDocument())
    // Sharing grants a read. There is no edit control on this panel because
    // there is no route behind one.
    expect(screen.queryByRole('button', { name: /edit/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /build/i })).not.toBeInTheDocument()
  })

  it('lets a player take back their own and nobody else’s', async () => {
    renderTable(viewport, 'player', [mine, theirs])

    await waitFor(() => expect(screen.getByText('Ada')).toBeInTheDocument())
    // One "Take off" button, for the one character that is theirs.
    expect(screen.getAllByRole('button', { name: 'Take off' })).toHaveLength(1)
  })

  it('says the table is empty exactly once, and counts nothing above it', async () => {
    renderTable(viewport, 'owner', [])

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Add a character' })).toBeInTheDocument(),
    )
    // It used to be rendered twice: once as a caption above the list and once
    // as the list's own empty state, one directly under the other.
    expect(screen.getAllByText('Nothing on the table yet.')).toHaveLength(1)
  })

  it('does not caption the table with a count', async () => {
    renderTable(viewport, 'owner', [mine, theirs])

    await waitFor(() => expect(screen.getByText('Ada')).toBeInTheDocument())
    // The list says how many there are by being the list.
    expect(screen.queryByText(/readable by everyone here/)).not.toBeInTheDocument()
    expect(screen.queryByText(/2 characters/)).not.toBeInTheDocument()
  })

  it('lets a DM clear anybody off the table', async () => {
    renderTable(viewport, 'dm', [mine, theirs])

    await waitFor(() => expect(screen.getByText('Ada')).toBeInTheDocument())
    // A guest's session ends and their character would otherwise be stuck
    // there, so a DM may take down either.
    expect(screen.getAllByRole('button', { name: 'Take off' })).toHaveLength(2)
  })
})
