import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { InvitePreview } from '@/lib/api'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { JoinScreen } from './JoinScreen'

const PREVIEW: InvitePreview = {
  group_id: 'grp_1',
  group_name: 'Wednesday Night',
  role: 'player',
  invited_by: 'Olive',
  expires_at: '2026-01-02T00:00:00Z',
  already_member: false,
}

function stubFetch(body: unknown, status = 200) {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify(body), {
          status,
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )
}

function renderJoin(viewport: Viewport, token = 'a-token') {
  return renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter initialEntries={['/groups/join']}>
        <Routes>
          <Route path="/groups/join" element={<JoinScreen token={token} />} />
          <Route path="/groups" element={<div>the group list</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  )
}

beforeEach(() => {
  window.sessionStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
  window.sessionStorage.clear()
})

describe.each(['mobile', 'desktop'] as const)('JoinScreen (%s)', (viewport) => {
  it('names the group and whoever is asking', async () => {
    stubFetch(PREVIEW)
    renderJoin(viewport)

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.getByText(/Olive invited you/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join group' })).toBeInTheDocument()
  })

  it('says so when there is no invitation at all', async () => {
    stubFetch(PREVIEW)
    renderJoin(viewport, '')

    await waitFor(() => expect(screen.getByText('No invitation')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Join group' })).not.toBeInTheDocument()
  })
})

// A stale link must read as a bad link and not as a lost session. The server
// answers 400 rather than 401 precisely so that this screen can say so without
// the client tearing the session down -- see the group usecase's openInvite.
describe('a stale link', () => {
  it('reports the problem instead of signing anybody out', async () => {
    stubFetch(
      { error: { code: 'validation_error', message: 'this invitation link is not valid, or it has expired' } },
      400,
    )
    renderJoin('desktop')

    await waitFor(() =>
      expect(screen.getByText('That invitation is not usable')).toBeInTheDocument(),
    )
    expect(screen.getByText(/not valid, or it has expired/)).toBeInTheDocument()
    // Still signed in, so the way back into the app is offered.
    expect(screen.getByRole('button', { name: 'Your groups' })).toBeInTheDocument()
  })
})

// Somebody who is already seated is told so, rather than being offered a
// button whose effect they cannot see.
describe('an invitation to a group you are already in', () => {
  it('offers to open it instead of joining', async () => {
    stubFetch({ ...PREVIEW, already_member: true })
    renderJoin('desktop')

    await waitFor(() =>
      expect(screen.getByText('You are already in this group.')).toBeInTheDocument(),
    )
    expect(screen.queryByRole('button', { name: 'Join group' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Open it' })).toBeInTheDocument()
  })
})
