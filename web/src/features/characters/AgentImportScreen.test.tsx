import { act, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'
import type { AgentView } from '@/lib/api/agent'
import { AgentImportScreen } from './AgentImportScreen'

const VIEW: AgentView = {
  session: {
    id: 'session1',
    folder: 'folder1',
    status: 'review',
    revision: 5,
    events: [
      { id: 1, kind: 'user', text: 'Please import Zephyr' },
      { id: 2, kind: 'assistant', text: 'Draft ready' },
    ],
    files: [{ name: 'sheet.pdf', mime: 'application/pdf' }],
    manual: [],
    assumptions: ['Matched a misspelled spell name'],
  },
  sheet: {
    identity: { name: 'Zephyr', level: 1, experience: 0 },
    base: {
      hitPoints: { current: 9, max: 9 },
      deathSaves: { successes: 0, failures: 0 },
    },
    abilities: {
      scores: { str: 10, dex: 10, con: 10, int: 10, wis: 10, cha: 10 },
      modifiers: { str: 0, dex: 0, con: 0, int: 0, wis: 0, cha: 0 },
    },
    skills: {},
    savingThrows: {},
    status: {
      armorClass: 10,
      initiative: 0,
      proficiencyBonus: 2,
      passivePerception: 10,
    },
    equipment: { equipped: [], backpack: [], loot: [] },
    resources: {},
    spells: {},
    actions: [],
  },
}
class Stream {
  static current: Stream
  listeners = new Map<string, (e: MessageEvent<string>) => void>()
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  constructor() {
    Stream.current = this
  }
  addEventListener(name: string, listener: (e: MessageEvent<string>) => void) {
    this.listeners.set(name, listener)
  }
  close() {
    this.closed = true
  }
  emit(name: string, value: unknown) {
    this.listeners.get(name)?.(
      new MessageEvent(name, { data: JSON.stringify(value) }),
    )
  }
}
let writes: { url: string; body: unknown }[]
beforeEach(() => {
  writes = []
  vi.stubGlobal('EventSource', Stream)
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (init?.method === 'POST') {
        writes.push({
          url,
          body:
            init.body instanceof FormData
              ? init.body
              : JSON.parse(String(init.body)),
        })
        if (url.includes('/finalize'))
          return Response.json({
            ...VIEW,
            session: {
              ...VIEW.session,
              revision: 6,
              status: 'saved',
              characterId: 'chr1',
            },
          })
        return Response.json(VIEW)
      }
      if (url.includes('/draft/sheet')) return Response.json(VIEW.sheet)
      if (url.includes('/draft/prompts'))
        return Response.json({
          seq: 1,
          revision: 5,
          complete: false,
          prompts: [],
        })
      if (url.includes('/draft/events'))
        return Response.json({
          seq: 1,
          revision: 5,
          events: [
            {
              type: 'init',
              seq: 1,
              changes: [
                {
                  path: 'identity.name',
                  op: 'set',
                  value: { kind: 'string', string: 'Zephyr' },
                },
              ],
            },
          ],
        })
      if (url.includes('/agent-sessions/session1')) return Response.json(VIEW)
      return Response.json([])
    }),
  )
})
for (const viewport of ['desktop', 'mobile'] as const)
  describe(`Import workspace ${viewport}`, () => {
    it('resumes its chat URL and keeps the composer while previewing the sheet', async () => {
      const user = setupUser()
      const result = renderAt(
        viewport,
        <MemoryRouter initialEntries={['/characters/import?session=session1']}>
          <Routes>
            <Route path="/characters/import" element={<AgentImportScreen />} />
            <Route
              path="/characters/import/:sessionId/:importView?"
              element={<AgentImportScreen />}
            />
          </Routes>
        </MemoryRouter>,
      )
      expect(await screen.findByText('Draft ready')).toBeInTheDocument()
      await user.type(
        screen.getByLabelText('Message the assistant'),
        'Keep my custom spell',
      )
      expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'View the sheet' }))
      expect(await screen.findByText('Back to chat')).toBeInTheDocument()
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(
        screen.queryByText('Assumptions to review'),
      ).not.toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Back to chat' }))
      expect(screen.getByLabelText('Message the assistant')).toHaveValue(
        'Keep my custom spell',
      )
      expect(writes).toHaveLength(0)
      result.unmount()
      expect(Stream.current.closed).toBe(true)
    })
    it('saves once explicitly and makes the conversation read-only', async () => {
      const user = setupUser()
      renderAt(
        viewport,
        <MemoryRouter initialEntries={['/characters/import?session=session1']}>
          <Routes>
            <Route path="/characters/import" element={<AgentImportScreen />} />
            <Route
              path="/characters/import/:sessionId/:importView?"
              element={<AgentImportScreen />}
            />
          </Routes>
        </MemoryRouter>,
      )
      await screen.findByText('Draft ready')
      await user.click(screen.getByRole('button', { name: 'Save character' }))
      expect(
        await screen.findByText(/This conversation is now read-only/),
      ).toBeInTheDocument()
      expect(
        screen.queryByLabelText('Message the assistant'),
      ).not.toBeInTheDocument()
      expect(writes.filter((v) => v.url.includes('/finalize'))).toHaveLength(1)
    })
  })
it('deduplicates replayed stream events and restores a coherent snapshot', async () => {
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/characters/import?session=session1']}>
      <Routes>
        <Route path="/characters/import" element={<AgentImportScreen />} />
        <Route
          path="/characters/import/:sessionId/:importView?"
          element={<AgentImportScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  act(() => {
    Stream.current.emit('update', {
      id: 3,
      kind: 'delta',
      text: 'Checking the spell',
    })
    Stream.current.emit('update', {
      id: 3,
      kind: 'delta',
      text: 'Checking the spell',
    })
  })
  expect(screen.getAllByText('Checking the spell')).toHaveLength(1)
  act(() =>
    Stream.current.emit('snapshot', {
      ...VIEW,
      session: {
        ...VIEW.session,
        revision: 6,
        events: [
          ...VIEW.session.events,
          { id: 3, kind: 'assistant', text: 'Spell checked' },
        ],
      },
    }),
  )
  await waitFor(() =>
    expect(screen.getByText('Spell checked')).toBeInTheDocument(),
  )
  expect(screen.queryByText('Checking the spell')).not.toBeInTheDocument()
})

it('uploads source bytes and optional instructions before creating a session', async () => {
  const user = setupUser()
  const { container } = renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/characters/import?folder=folder1']}>
      <Routes>
        <Route path="/characters/import" element={<AgentImportScreen />} />
        <Route
          path="/characters/import/:sessionId/:importView?"
          element={<AgentImportScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  expect(screen.getByRole('button', { name: 'Import' })).toBeDisabled()
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')
  if (!input) throw new Error('File input missing')
  await user.upload(
    input,
    new File(['Hero'], 'hero.txt', { type: 'text/plain' }),
  )
  await user.type(
    screen.getByLabelText('Instructions (optional)'),
    'Keep the custom items',
  )
  await user.click(screen.getByRole('button', { name: 'Import' }))
  await screen.findByText('Draft ready')
  expect(writes).toHaveLength(1)
  expect(writes[0]?.url).toContain('folder=folder1')
  const form = writes[0]?.body as FormData
  expect(form.get('instructions')).toBe('Keep the custom items')
  expect((form.get('files') as File).name).toBe('hero.txt')
})

it('keeps messages in order and waits for the assistant before sending', async () => {
  const user = setupUser()
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/characters/import?session=session1']}>
      <Routes>
        <Route path="/characters/import" element={<AgentImportScreen />} />
        <Route
          path="/characters/import/:sessionId/:importView?"
          element={<AgentImportScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  act(() =>
    Stream.current.emit('snapshot', {
      ...VIEW,
      session: { ...VIEW.session, revision: 6, status: 'running' },
    }),
  )
  await user.type(
    screen.getByLabelText('Message the assistant'),
    'My next message{Enter}',
  )
  expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  expect(screen.queryByRole('button', { name: 'Stop' })).not.toBeInTheDocument()
  expect(writes).toHaveLength(0)
  act(() =>
    Stream.current.emit('snapshot', {
      ...VIEW,
      session: {
        ...VIEW.session,
        revision: 7,
        status: 'waiting',
        events: [
          ...VIEW.session.events,
          { id: 3, kind: 'assistant', text: 'What next?' },
        ],
      },
    }),
  )
  const transcript = screen.getByRole('log')
  expect(transcript.textContent?.indexOf('Draft ready')).toBeLessThan(
    transcript.textContent?.indexOf('What next?') ?? 0,
  )
  await user.click(screen.getByRole('button', { name: 'Send' }))
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0]?.body).toMatchObject({
    action: 'message',
    revision: 7,
    text: 'My next message',
  })
})

it('opens the standard editor inside the same chat without saving', async () => {
  const user = setupUser()
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/characters/import?session=session1']}>
      <Routes>
        <Route path="/characters/import" element={<AgentImportScreen />} />
        <Route
          path="/characters/import/:sessionId/:importView?"
          element={<AgentImportScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  await user.type(
    screen.getByLabelText('Message the assistant'),
    'Keep this draft message',
  )
  await user.click(screen.getByRole('button', { name: 'Edit' }))
  expect(
    await screen.findByRole('tab', { name: 'Personal' }),
  ).toBeInTheDocument()
  expect(screen.getByRole('tab', { name: 'Equipment' })).toBeInTheDocument()
  expect(screen.queryByRole('tab', { name: 'Spells' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('link', { name: 'session1' }))
  expect(screen.getByLabelText('Message the assistant')).toHaveValue(
    'Keep this draft message',
  )
  expect(writes).toHaveLength(0)
})

it('renders activity inline, combines attachments with the user message and answers questions', async () => {
  const user = setupUser()
  const { container } = renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/characters/import/session1']}>
      <Routes>
        <Route
          path="/characters/import/:sessionId/:importView?"
          element={<AgentImportScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  act(() =>
    Stream.current.emit('snapshot', {
      ...VIEW,
      session: {
        ...VIEW.session,
        revision: 6,
        status: 'waiting',
        events: [
          {
            id: 1,
            kind: 'user',
            text: 'Import my rogue',
            files: [{ name: 'rogue.pdf', mime: 'application/pdf' }],
          },
          {
            id: 2,
            kind: 'tool',
            tool: 'get_build_context',
            source: 'private source page 9',
            assumption: 'Do not display this assumption',
          },
          {
            id: 3,
            kind: 'question',
            text: 'How should I preserve Arcane Trickster?',
            options: ['Keep as custom', 'I will provide details'],
          },
        ],
      },
    }),
  )
  expect(screen.getByText('Reading the draft')).toBeVisible()
  expect(container.querySelector('details')).toBeNull()
  expect(screen.queryByText(/private source/)).not.toBeInTheDocument()
  expect(
    screen.queryByText('Do not display this assumption'),
  ).not.toBeInTheDocument()
  expect(
    screen.getByText('rogue.pdf').parentElement?.parentElement?.textContent,
  ).toContain('Import my rogue')
  await user.click(screen.getByRole('button', { name: 'Keep as custom' }))
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0]?.body).toMatchObject({
    action: 'message',
    text: 'Keep as custom',
    revision: 6,
  })
})

it('recovers a missed end-of-turn snapshot so the next reply can be sent', async () => {
  const user = setupUser()
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/characters/import/session1']}>
      <Routes>
        <Route
          path="/characters/import/:sessionId/:importView?"
          element={<AgentImportScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  act(() =>
    Stream.current.emit('snapshot', {
      ...VIEW,
      session: { ...VIEW.session, status: 'running', revision: 6 },
    }),
  )
  await user.type(screen.getByLabelText('Message the assistant'), 'My answer')
  expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  const original = vi.mocked(fetch).getMockImplementation()!
  vi.mocked(fetch).mockImplementation((input, init) =>
    String(input).includes('/agent-sessions/session1') && (!init?.method || init.method === 'GET')
      ? Promise.resolve(
          Response.json({
            ...VIEW,
            session: { ...VIEW.session, status: 'waiting', revision: 7 },
          }),
        )
      : original(input, init),
  )
  await waitFor(
    () => expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled(),
    { timeout: 4500 },
  )
  await user.click(screen.getByRole('button', { name: 'Send' }))
  expect(writes[0]?.body).toMatchObject({ revision: 7, text: 'My answer' })
})

it('reloads a standalone preview with the import breadcrumbs and no assumptions panel', async () => {
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/characters/import/session1/character']}>
      <Routes>
        <Route
          path="/characters/import/:sessionId/:importView?"
          element={<AgentImportScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Back to chat')
  expect(
    screen.getByRole('link', { name: 'Chatbot Creation' }),
  ).toHaveAttribute('href', '/characters/import')
  expect(screen.getByRole('link', { name: 'session1' })).toHaveAttribute(
    'href',
    '/characters/import/session1',
  )
  expect(screen.queryByRole('log')).not.toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(
    screen.queryByText('Matched a misspelled spell name'),
  ).not.toBeInTheDocument()
})
