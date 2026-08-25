import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'
import { setupUser } from '@/test/user'

import { GroupListScreen } from './GroupListScreen'

interface Call {
  url: string
  method: string
  body: string
}

/** Records every request and answers the list, then the create. */
function stubFetch(groups: unknown[]): Call[] {
  const calls: Call[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const call = {
        url: String(input),
        method: init?.method ?? 'GET',
        body: String(init?.body ?? ''),
      }
      calls.push(call)
      const body =
        call.method === 'POST'
          ? { id: 'grp_new', name: 'Wednesday Night', created_at: '', role: 'owner', members: [] }
          : { groups }
      return new Response(JSON.stringify(body), {
        status: call.method === 'POST' ? 201 : 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
  return calls
}

function renderList(viewport: Viewport) {
  return renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter initialEntries={['/groups']}>
        <Routes>
          <Route path="/groups" element={<GroupListScreen />} />
          <Route path="/groups/:id" element={<div>the group</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('GroupListScreen (%s)', (viewport) => {
  it('lists the tables you play at, with your rank in each', async () => {
    stubFetch([
      { id: 'grp_1', name: 'Wednesday Night', created_at: '', role: 'owner' },
      { id: 'grp_2', name: 'Sunday Sandbox', created_at: '', role: 'player' },
    ])
    renderList(viewport)

    await waitFor(() => expect(screen.getByText('Wednesday Night')).toBeInTheDocument())
    expect(screen.getByText('Sunday Sandbox')).toBeInTheDocument()
    expect(screen.getByText('Owner')).toBeInTheDocument()
    expect(screen.getByText('Player')).toBeInTheDocument()
  })

  it('points a newcomer at both ways to get into a group', async () => {
    stubFetch([])
    renderList(viewport)

    await waitFor(() => expect(screen.getByText(/No groups yet/)).toBeInTheDocument())
    // Making one is not the only way in, and the empty state has to say so:
    // most players arrive through somebody else's link.
    expect(screen.getByText(/invitation link/)).toBeInTheDocument()
  })

  it('creates a group and opens it', async () => {
    const calls = stubFetch([])
    renderList(viewport)
    await waitFor(() => expect(screen.getByText(/No groups yet/)).toBeInTheDocument())

    await setupUser().click(screen.getByRole('button', { name: 'New group' }))
    await setupUser().type(await screen.findByLabelText('Name'), 'Wednesday Night')
    await setupUser().click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(screen.getByText('the group')).toBeInTheDocument())
    const post = calls.find((call) => call.method === 'POST')
    expect(post?.url).toContain('/v1/groups')
    expect(post?.body).toContain('Wednesday Night')
  })
})
