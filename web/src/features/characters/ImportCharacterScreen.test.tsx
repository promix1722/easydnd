import { act, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'
import type { AgentView } from '@/lib/api/agent'
import { ImportCharacterScreen } from './ImportCharacterScreen'

const RULES = { edition: '2014', semantics: '1', packs: [{ id: 'srd-2014', version: '1.0.0', digest: 'a'.repeat(64) }] }
const VIEW: AgentView = {
  session: {
    id: 'session1',
    folder: 'folder1',
    status: 'review',
    revision: 5,
    characterId: 'chr1',
    events: [
      { id: 1, kind: 'user', text: 'Please import Zephyr' },
      { id: 2, kind: 'assistant', text: 'Draft ready' },
    ],
    files: [{ name: 'sheet.pdf', mime: 'application/pdf' }],
    manual: [],
    assumptions: ['Matched a misspelled spell name'],
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
      if (url.includes('/packs/resolve')) return Response.json(RULES)
      if (url.includes('/packs')) return Response.json({ defaultRules: RULES, packs: [{ id: 'srd-2014', title: 'D&D 2014', releases: RULES.packs, archived: false }] })
      if (url.includes('/agent-capabilities'))
        return Response.json({ enabled: false })
      if (init?.method === 'POST') {
        writes.push({
          url,
          body:
            init.body instanceof FormData
              ? init.body
              : JSON.parse(String(init.body)),
        })
        return Response.json(VIEW)
      }
      if (url.includes('/agent-sessions/session1')) return Response.json(VIEW)
      return Response.json([])
    }),
  )
})
for (const viewport of ['desktop', 'mobile'] as const)
  describe(`Import workspace ${viewport}`, () => {
    // The chat writes to a real character, so View and Edit are that
    // character's own pages rather than pages of the chat.
    it.each([['View the sheet', 'sheet of chr1'], ['Edit', 'builder of chr1']])('%s opens the real character', async (button, page) => {
      const user = setupUser()
      const result = renderAt(
        viewport,
        <MemoryRouter initialEntries={['/ai-wizard?session=session1']}>
          <Routes>
            <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
            <Route path="/characters/chr1" element={<p>sheet of chr1</p>} />
            <Route path="/characters/chr1/build" element={<p>builder of chr1</p>} />
          </Routes>
        </MemoryRouter>,
      )
      expect(await screen.findByText('Draft ready')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Save character' })).not.toBeInTheDocument()
      await user.click(screen.getAllByRole('button', { name: button })[0]!)
      expect(await screen.findByText(page)).toBeInTheDocument()
      expect(writes).toHaveLength(0)
      result.unmount()
      expect(Stream.current.closed).toBe(true)
    })
  })
it('deduplicates replayed stream events and restores a coherent snapshot', async () => {
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard?session=session1']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
        <Route
          path="/ai-wizard/:sessionId"
          element={<ImportCharacterScreen />}
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
    <MemoryRouter initialEntries={['/ai-wizard?folder=folder1']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
        <Route
          path="/ai-wizard/:sessionId"
          element={<ImportCharacterScreen />}
        />
      </Routes>
    </MemoryRouter>,
  )
  // The rules are the assistant's first message, already chosen: nothing
  // stands between opening the wizard and sending.
  expect(within(screen.getByRole('log')).getByText('Rule packs')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
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
  await waitFor(() => expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled())
  await user.click(screen.getByRole('button', { name: 'Send' }))
  await screen.findByText('Draft ready')
  expect(writes).toHaveLength(1)
  expect(writes[0]?.url).toContain('folder=folder1')
  const form = writes[0]?.body as FormData
  expect(form.get('instructions')).toBe('Keep the custom items')
  expect(JSON.parse(String(form.get('rules')))).toEqual(RULES)
  expect((form.get('files') as File).name).toBe('hero.txt')
})

it('keeps messages in order and waits for the assistant before sending', async () => {
  const user = setupUser()
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard?session=session1']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
        <Route
          path="/ai-wizard/:sessionId"
          element={<ImportCharacterScreen />}
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

it('renders activity inline, combines attachments with the user message and answers questions', async () => {
  const user = setupUser()
  const { container } = renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard/session1']}>
      <Routes>
        <Route
          path="/ai-wizard/:sessionId"
          element={<ImportCharacterScreen />}
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
  expect(screen.queryByText('Reading the draft')).not.toBeInTheDocument()
  expect(container.querySelector('details')).toBeNull()
  expect(screen.queryByText(/private source/)).not.toBeInTheDocument()
  expect(
    screen.queryByText('Do not display this assumption'),
  ).not.toBeInTheDocument()
  // One bubble: the attachment sits on the message that sent it.
  expect(screen.getByRole('log').textContent).toContain('rogue.pdfImport my rogue')
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
    <MemoryRouter initialEntries={['/ai-wizard/session1']}>
      <Routes>
        <Route
          path="/ai-wizard/:sessionId"
          element={<ImportCharacterScreen />}
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
