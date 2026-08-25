import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { GroupDetail, GroupRole } from '@/lib/api'
import { testAccount, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { GroupScreen } from './GroupScreen'

/** A table with one of everything, seen through `role`'s eyes. */
function groupAs(role: GroupRole): GroupDetail {
  return {
    id: 'grp_1',
    name: 'Wednesday Night',
    created_at: '2026-01-01T00:00:00Z',
    role,
    members: [
      {
        user_id: role === 'owner' ? testAccount.id : 'owner-1',
        display_name: role === 'owner' ? 'Alice' : 'Olive',
        role: 'owner',
        joined_at: '2026-01-01T00:00:00Z',
        anonymous: false,
      },
      {
        user_id: role === 'dm' ? testAccount.id : 'dm-1',
        display_name: role === 'dm' ? 'Alice' : 'Devon',
        role: 'dm',
        joined_at: '2026-01-02T00:00:00Z',
        anonymous: false,
      },
      {
        user_id: role === 'player' ? testAccount.id : 'player-1',
        display_name: role === 'player' ? 'Alice' : 'Wintry Otter',
        role: 'player',
        joined_at: '2026-01-03T00:00:00Z',
        anonymous: role !== 'player',
      },
    ],
  }
}

function stubFetch(group: GroupDetail) {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify(group), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )
}

function renderGroup(viewport: Viewport, group: GroupDetail) {
  stubFetch(group)
  return renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter initialEntries={['/groups/grp_1']}>
        <Routes>
          <Route path="/groups/:id" element={<GroupScreen />} />
          <Route path="/groups" element={<div>the group list</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('GroupScreen (%s)', (viewport) => {
  it('shows the owner what only an owner may do', async () => {
    renderGroup(viewport, groupAs('owner'))

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Invite' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
  })

  it('offers a DM the invite button but not deletion', async () => {
    renderGroup(viewport, groupAs('dm'))

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Invite' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
    // A DM may walk away; an owner may not, which the next test pins.
    expect(screen.getByRole('button', { name: 'Leave' })).toBeInTheDocument()
  })

  it('offers a player nothing but leaving', async () => {
    renderGroup(viewport, groupAs('player'))

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Invite' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Manage' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Leave' })).toBeInTheDocument()
  })

  it('marks a guest in the roster', async () => {
    renderGroup(viewport, groupAs('owner'))

    await waitFor(() => expect(screen.getByText('Wintry Otter')).toBeInTheDocument())
    expect(screen.getAllByText('Guest').length).toBeGreaterThan(0)
  })
})

// The owner's Leave button is rendered and disabled rather than hidden: a
// control that is not there teaches nothing, and this is the rule people are
// most likely to go looking for.
describe('the owner cannot leave', () => {
  it('shows Leave, disabled, with the way out beside it', async () => {
    renderGroup('desktop', groupAs('owner'))

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    const leave = screen.getByRole('button', { name: 'Leave' })
    expect(leave).toBeInTheDocument()
    expect(leave).toHaveAttribute('data-disabled')
    // Deleting the group is the escape hatch, so it must be on screen too.
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
  })
})
