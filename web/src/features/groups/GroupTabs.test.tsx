import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { GroupDetail } from '@/lib/api'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { GroupScreen } from './GroupScreen'

const group: GroupDetail = {
  id: 'grp_1',
  name: 'Wednesday Night',
  created_at: '2026-01-01T00:00:00Z',
  role: 'owner',
  members: [
    {
      user_id: 'owner-1',
      display_name: 'Olive',
      role: 'owner',
      joined_at: '2026-01-01T00:00:00Z',
      anonymous: false,
    },
  ],
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('GroupScreen tabs', () => {
  it('offers Members and Characters, and keeps Games out of a group', async () => {
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
    renderAt(
      'desktop',
      withAuth(
        {},
        <MemoryRouter initialEntries={['/groups/grp_1']}>
          <Routes>
            <Route path="/groups/:id" element={<GroupScreen />} />
          </Routes>
        </MemoryRouter>,
      ),
    )

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.getByRole('tab', { name: 'Members' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Characters' })).toBeInTheDocument()
    // Games are a section of their own, reached from the main navigation. A
    // group is people and the characters they have shared, and nothing else.
    expect(screen.queryByRole('tab', { name: 'Games' })).not.toBeInTheDocument()
  })

  it('names the group and your rank, and does not count the members', async () => {
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
    renderAt(
      'desktop',
      withAuth(
        {},
        <MemoryRouter initialEntries={['/groups/grp_1']}>
          <Routes>
            <Route path="/groups/:id" element={<GroupScreen />} />
          </Routes>
        </MemoryRouter>,
      ),
    )

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    // Twice: the heading badge, and the one member's Role cell.
    expect(screen.getAllByText('Owner').length).toBeGreaterThan(0)
    // The roster is right there and counts itself; a number above it is one
    // more thing to keep in step with the rows.
    expect(screen.queryByText(/^\d+ members?$/)).not.toBeInTheDocument()
  })
})
