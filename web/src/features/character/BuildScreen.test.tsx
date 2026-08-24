import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { resetCatalogCache } from '@/lib/api'
import { renderAt } from '@/test/render'

import { BuildScreen } from './BuildScreen'

/**
 * The build loop, wired to responses in the shape the real API sends.
 *
 * The payloads below were captured from a running server rather than
 * invented, because the thing worth testing here is that the screen posts
 * what the server asked for -- and a hand-written fixture would only prove
 * the screen agrees with my memory of the contract.
 */

const RACE_PROMPT = {
  seq: 1,
  complete: false,
  prompts: [
    {
      choice: {
        prompt: 'character/race',
        choose: 1,
        kind: 'race',
        from: { kind: 'collection', collection: 'race' },
      },
      group: 'race',
      optional: false,
      advances: false,
      event: { type: 'race' },
      heldOnly: false,
    },
  ],
}

const RACES = [
  { slug: 'half-elf', name: 'Half-Elf', speed: 30 },
  { slug: 'dwarf', name: 'Dwarf', speed: 25 },
]

let posted: { url: string; body: unknown }[] = []

function mockApi() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      const method = init?.method ?? 'GET'
      if (method !== 'GET') {
        posted.push({ url, body: JSON.parse(String(init?.body ?? '{}')) })
        return jsonResponse({ seq: 2, sheet: {} })
      }
      if (url.includes('/prompts')) return jsonResponse(RACE_PROMPT)
      if (url.includes('/catalog/races')) return jsonResponse(RACES)
      return jsonResponse([])
    }),
  )
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function renderBuild(viewport: 'mobile' | 'desktop') {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={['/characters/chr_000001/build']}>
      <Routes>
        <Route path="/characters/:id/build" element={<BuildScreen />} />
        <Route path="/characters/:id" element={<div>sheet</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  posted = []
  resetCatalogCache()
  mockApi()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('BuildScreen at %s', (viewport) => {
  it('asks the question the server said was next', async () => {
    renderBuild(viewport)
    expect(await screen.findByText('Choose a race')).toBeInTheDocument()
    // Options come from the collection the prompt named.
    expect(await screen.findByRole('button', { name: /Half-Elf/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Dwarf/ })).toBeInTheDocument()
  })

  it('posts the event the prompt specified, not one it decided on', async () => {
    const user = userEvent.setup()
    renderBuild(viewport)

    await user.click(await screen.findByRole('button', { name: /Half-Elf/ }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    const write = posted[0]
    expect(write?.url).toContain('/characters/chr_000001/events')
    // The sequence the client believed the log ended at: without it, two
    // clients editing one character would clobber each other silently.
    expect(write?.body).toMatchObject({
      expectedSeq: 1,
      events: [{ type: 'race', ref: 'race:half-elf' }],
    })
  })

  it('cannot go back past the init event', async () => {
    renderBuild(viewport)
    await screen.findByText('Choose a race')
    // seq is 1, so the log holds only the init event and Back is inert.
    expect(screen.getByRole('button', { name: /^back$/i })).toBeDisabled()
  })

  it('shows progress by stage, because the step count is unknowable', async () => {
    renderBuild(viewport)
    await screen.findByText('Choose a race')
    // Nested prompts do not exist until their parent is answered, so there
    // is no total to count towards.
    expect(screen.getByText('abilities')).toBeInTheDocument()
    expect(screen.getByText('race')).toBeInTheDocument()
    expect(screen.getByText('class')).toBeInTheDocument()
  })
})
