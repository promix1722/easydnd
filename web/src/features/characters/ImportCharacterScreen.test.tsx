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
    created: '2026-10-04T10:00:00Z',
    finished: false,
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
let sessions: AgentView['session'][]
beforeEach(() => {
  writes = []
  sessions = []
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
      if (/\/agent-sessions(\?|$)/.test(url)) return Response.json(sessions)
      return Response.json([])
    }),
  )
})
for (const viewport of ['desktop', 'mobile'] as const)
  describe(`Import workspace ${viewport}`, () => {
    // The chat writes to a real character, so View and Edit are that
    // character's own pages rather than pages of the chat.
    it.each([['View the sheet', 'sheet of chr1'], ['Edit', 'builder of chr1'], ['Finish', 'sheet of chr1']])('%s opens the real character', async (button, page) => {
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
      // Only Finish tells the server anything: it is what closes the chat.
      expect(writes.map((write) => (write.body as { action: string }).action)).toEqual(button === 'Finish' ? ['finish'] : [])
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
  // The assistant opens by asking for the rules, inside the conversation, and
  // there is nothing to type into until it is the player's turn to speak.
  const log = within(screen.getByRole('log'))
  expect(log.getByText(/Which rules should I use/)).toBeInTheDocument()
  expect(screen.getByRole('textbox')).toBeDisabled()
  // The rules are prepared answers like any other question's: one press.
  await user.click(await log.findByRole('button', { name: 'D&D 2014 v1.0.0' }))
  // The answer is a message of the player's, and the next question follows it.
  // The question it answers stays where it was asked, no longer pressable.
  await waitFor(() => expect(log.getAllByText('D&D 2014 v1.0.0').length).toBeGreaterThan(1))
  expect(log.getByRole('button', { name: 'D&D 2014 v1.0.0' })).toBeDisabled()
  expect(log.getByText(/attach, or would you rather describe/)).toBeInTheDocument()
  // Describing needs no button: the box is simply open. Attaching is a quiet
  // control inside it, which the file then replaces.
  expect(screen.getByRole('textbox')).toBeEnabled()
  expect(log.getByRole('button', { name: 'Attach a sheet' })).toBeInTheDocument()
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

it('offers the composer only on the player\'s turn, and keeps the opening in the log', async () => {
  const user = setupUser()
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard?session=session1']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  // No line above the transcript: no status, no chat id.
  expect(screen.queryByText(/Chat ·/)).not.toBeInTheDocument()
  act(() =>
    Stream.current.emit('snapshot', {
      ...VIEW,
      session: {
        ...VIEW.session,
        revision: 6,
        status: 'running',
        events: [{ id: 9, kind: 'rules', data: { packs: [{ id: 'srd-2014', version: '1.0.0' }] } }, ...VIEW.session.events],
      },
    }),
  )
  // While the assistant works there is nothing to type into.
  await waitFor(() => expect(screen.getByRole('textbox')).toBeDisabled())
  expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  // What was asked and answered before the session began is still there.
  const transcript = screen.getByRole('log')
  expect(transcript.textContent).toMatch(/Which rules should I use.*D&D 2014 v1\.0\.0.*describe the character.*Please import Zephyr/)
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
  expect(transcript.textContent?.indexOf('Draft ready')).toBeLessThan(
    transcript.textContent?.indexOf('What next?') ?? 0,
  )
  // The assistant's message ends in prepared answers, the last of which opens
  // the text field; until it is pressed there is nothing to type into.
  expect(screen.getByRole('textbox')).toBeEnabled()
  // A message that offered no answers gets none invented for it -- but a
  // turn that ended without a question still ends on something to press.
  expect(screen.queryByRole('button', { name: 'Continue' })).not.toBeInTheDocument()
  expect(within(transcript).getByRole('button', { name: 'Finish' })).toBeInTheDocument()
  // Every message of the assistant's ends with it; only the latest is live.

  // The reply is written in the conversation, under the message it answers.
  expect(within(screen.getByRole('log')).getByLabelText('Message the assistant')).toBeInTheDocument()
  // No attaching after the first message.
  expect(document.querySelector('input[type="file"]')).toBeNull()
  await user.type(await screen.findByLabelText('Message the assistant'), 'My next message{Enter}')
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
  await waitFor(() => expect(screen.getByRole('textbox')).toBeDisabled())
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
  await waitFor(() => expect(screen.getByRole('textbox')).toBeEnabled(), { timeout: 4500 })

  await user.type(await screen.findByLabelText('Message the assistant'), 'My answer')
  await user.click(screen.getByRole('button', { name: 'Send' }))
  expect(writes[0]?.body).toMatchObject({ revision: 7, text: 'My answer' })
})

it('gives every import its own message and keeps answered choices', async () => {
  const { container } = renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard/session1']}>
      <Routes>
        <Route path="/ai-wizard/:sessionId" element={<ImportCharacterScreen />} />
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
          { id: 1, kind: 'progress', data: { operation: 'imported', field: 'item', value: 'Dagger × 2' } },
          { id: 2, kind: 'progress', data: { operation: 'imported', field: 'item', value: 'Backpack × 1' } },
          { id: 3, kind: 'progress', data: { operation: 'imported', field: 'identity.name', value: 'Vas Pup' } },
          { id: 4, kind: 'question', text: 'Which origin?', options: ['Wild Magic', 'Draconic'] },
          { id: 5, kind: 'user', text: 'Wild Magic' },
          { id: 6, kind: 'question', text: 'And the rest?', options: ['Pick them for me', 'One by one'] },
        ],
      },
    }),
  )
  await screen.findByText('And the rest?')
  // Never folded, and every write a message of its own: what was imported
  // as a caption, its value beneath.
  expect(container.querySelector('details')).toBeNull()
  expect(screen.getAllByText('Imported field: Equipment')).toHaveLength(2)
  expect(screen.getByText('Imported field: Name')).toHaveStyle({ fontWeight: '700' })
  expect(screen.getByText('Dagger × 2')).toBeInTheDocument()
  expect(screen.getByText('Vas Pup')).toBeInTheDocument()
  // What can be done with the character is beside the page's name, not in the log.
  expect(within(screen.getByRole('log')).queryByRole('button', { name: 'View the sheet' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
  // The choice already made stays under its question, no longer pressable.
  expect(screen.getByRole('button', { name: 'Draconic' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'One by one' })).toBeEnabled()
})

it('opens on the latest unfinished chat, and on a new one when every chat is finished', async () => {
  // Unfinished means Finish was not pressed -- a character the assistant
  // calls ready is still an open chat.
  const chat = (id: string, finished: boolean, created: string) => ({ ...VIEW.session, id, finished, created })
  sessions = [chat('finished', true, '2026-10-04T12:00:00Z'), chat('older', false, '2026-10-04T09:00:00Z'), chat('session1', false, '2026-10-04T11:00:00Z')]
  const { unmount } = renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
        <Route path="/ai-wizard/:sessionId" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  expect(await screen.findByText('Draft ready')).toBeInTheDocument()
  unmount()
  sessions = [chat('finished', true, '2026-10-04T12:00:00Z')]
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
        <Route path="/ai-wizard/:sessionId" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  expect(await screen.findByText(/Which rules should I use/)).toBeInTheDocument()
  expect(screen.queryByText('Draft ready')).not.toBeInTheDocument()
})

it('offers view, edit, finish and delete under the last message once the character is done', async () => {
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard/session1']}>
      <Routes>
        <Route path="/ai-wizard/:sessionId" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  const log = within(screen.getByRole('log'))
  for (const name of ['View the sheet', 'Edit', 'Finish', 'Delete'])
    expect(log.getByRole('button', { name })).toBeInTheDocument()
  // The same four beside the page's name, under the same names.
  expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(2)
  expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument()
})

it('shows a finished chat as a record: nothing to type, finish or delete', async () => {
  const original = vi.mocked(fetch).getMockImplementation()!
  vi.mocked(fetch).mockImplementation((input, init) =>
    String(input).includes('/agent-sessions/session1') && (!init?.method || init.method === 'GET')
      ? Promise.resolve(Response.json({ session: { ...VIEW.session, finished: true } }))
      : original(input, init),
  )
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard/session1']}>
      <Routes>
        <Route path="/ai-wizard/:sessionId" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByText('Draft ready')
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Finish' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'View the sheet' })).toBeInTheDocument()
})
