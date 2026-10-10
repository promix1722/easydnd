import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiPath } from '@/test/api'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { ReceiveScreen } from './ReceiveScreen'

const PREVIEW = { name: 'Ada', level: 3, classes: [{ class: 'wizard', level: 3 }] }

function stubFetch(preview: unknown, status = 200) {
  const fetched = vi.fn(async (input: RequestInfo | URL) => {
    const accepting = apiPath(String(input)).endsWith('/copy-links/accept')
    return new Response(JSON.stringify(accepting ? { id: 'chr_000009', seq: 1, sheet: {} } : preview), {
      status: accepting ? 201 : status,
      headers: { 'Content-Type': 'application/json' },
    })
  })
  vi.stubGlobal('fetch', fetched)
  return fetched
}

function renderReceive(token = 'a-token') {
  return renderAt(
    'desktop',
    withAuth(
      {},
      <MemoryRouter initialEntries={['/characters/receive']}>
        <Routes>
          <Route path="/characters/receive" element={<ReceiveScreen token={token} />} />
          <Route path="/characters/:id" element={<div>the new sheet</div>} />
          <Route path="/characters" element={<div>the character list</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

// One viewport: nothing in this tree branches on width. See docs/web.md.
describe('ReceiveScreen', () => {
  it('names the character, and taking it lands on the copy', async () => {
    const fetched = stubFetch(PREVIEW)
    renderReceive()

    await waitFor(() => expect(screen.getByText('Ada')).toBeInTheDocument())
    expect(screen.getByText('Wizard 3')).toBeInTheDocument()

    await setupUser().click(screen.getByRole('button', { name: 'Add to my characters' }))

    await waitFor(() => expect(screen.getByText('the new sheet')).toBeInTheDocument())
    // The token travels in the body, never the URL.
    const [url, init] = fetched.mock.calls.at(-1) as unknown as [string, RequestInit]
    expect(url).not.toContain('a-token')
    expect(JSON.parse(String(init.body))).toEqual({ token: 'a-token' })
  })

  it('says so when the link carries nothing', async () => {
    stubFetch(PREVIEW)
    renderReceive('')

    await waitFor(() => expect(screen.getByText('No invitation')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Add to my characters' })).not.toBeInTheDocument()
  })

  it('reports a stale link instead of signing anybody out', async () => {
    stubFetch({ error: { code: 'validation_error', reason: 'invite.invalid' } }, 400)
    renderReceive()

    await waitFor(() => expect(screen.getByText('That link is not usable')).toBeInTheDocument())
    expect(screen.getByText(/not valid, or it has expired/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Your characters' })).toBeInTheDocument()
  })
})
