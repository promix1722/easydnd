import { act, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderAt } from '@/test/render'
import { pacing } from './useReveal'
import { setupUser } from '@/test/user'
import { polling } from '@/lib/api/agent'
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
// The server's side of the poll. A real one answers at once; this one holds
// the request until a test answers it, so that a test decides what the screen
// has seen. `emit` resolves once the screen has taken every answer
// and asked again, so what follows it sees them applied.
const Stream = {
  closed: false,
  waiting: null as ((r: Response) => void) | null,
  queue: [] as unknown[],
  taken: [] as (() => void)[],
  reset() {
    Object.assign(this, { closed: false, waiting: null, queue: [], taken: [] })
  },
  poll(signal?: AbortSignal | null) {
    return new Promise<Response>((resolve, reject) => {
      signal?.addEventListener('abort', () => {
        this.closed = true
        reject(new DOMException('aborted', 'AbortError'))
      })
      if (this.queue.length) return resolve(Response.json(this.queue.shift()))
      this.taken.splice(0).forEach((done) => done())
      this.waiting = resolve
    })
  },
  emit(name: 'snapshot' | 'update', value: unknown) {
    const body = name === 'update' ? { events: [value] } : value
    if (this.waiting) {
      this.waiting(Response.json(body))
      this.waiting = null
    } else this.queue.push(body)
    return new Promise<void>((done) => this.taken.push(done))
  },
}
// A chat as the server opens it: one question, no rules, no character.
const { characterId: _none, ...blank } = VIEW.session
const OPENED: AgentView = {
  session: { ...blank, id: 'opened', status: 'opening', revision: 1, events: [{ id: 1, kind: 'opening' }], files: [] },
}
let writes: { url: string; body: unknown }[]
let sessions: AgentView['session'][]
let opened: AgentView
beforeEach(() => {
  writes = []
  sessions = []
  opened = OPENED
  polling.every = 0
  Stream.reset()
  // The pacing of the chat is one test's subject and every other's delay.
  pacing.on = false
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/packs/resolve')) return Response.json(RULES)
      if (url.includes('/packs')) return Response.json({ defaultRules: RULES, packs: [{ id: 'srd-2014', title: 'D&D 2014', releases: RULES.packs, archived: false }] })
      if (url.includes('/agent-capabilities'))
        return Response.json({ enabled: false })
      if (init?.method === 'POST') {
        const body = init.body instanceof FormData ? init.body : init.body ? JSON.parse(String(init.body)) : undefined
        writes.push({ url, body })
        // The opened chat, answered in steps: its rules, then a first message.
        const events = opened.session.events
        if ((body as { action?: string } | undefined)?.action === 'rules')
          opened = { session: { ...opened.session, revision: 2, events: [...events, { id: events.length + 1, kind: 'rules', data: { packs: RULES.packs } }] } }
        else if (url.includes('/opened/files')) {
          // A first message sent before the rules takes the deployment's own,
          // and the chat says so after the message.
          const said = [
            { kind: 'user', text: String((body as FormData).get('instructions')) },
            ...(events.some((event) => event.kind === 'rules') ? [] : [{ kind: 'rules', data: { packs: RULES.packs, assumed: true } }]),
            { kind: 'assistant', text: 'Draft ready' },
          ]
          opened = {
            session: {
              ...opened.session,
              status: 'review',
              revision: 3,
              characterId: 'chr1',
              events: [...events, ...said.map((event, at) => ({ ...event, id: events.length + at + 1 }))],
            },
          }
        }
        else if (!/\/agent-sessions(\?|$)/.test(url)) return Response.json(VIEW)
        return Response.json(opened)
      }
      if (url.includes('revision=')) return Stream.poll(init?.signal)
      if (url.includes('/agent-sessions/opened')) return Response.json(opened)
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
      expect(Stream.closed).toBe(true)
    })
  })
it('deduplicates replayed events and restores a coherent snapshot', async () => {
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
  await act(async () => {
    void Stream.emit('update', {
      id: 3,
      kind: 'delta',
      text: 'Checking the spell',
    })
    await Stream.emit('update', {
      id: 3,
      kind: 'delta',
      text: 'Checking the spell',
    })
  })
  expect(screen.getAllByText('Checking the spell')).toHaveLength(1)
  await act(() =>
    Stream.emit('snapshot', {
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

it('opens a chat, answers its rules and sends the sheet with the first message', async () => {
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
  // The assistant opens by asking for the rules -- the first event of a chat
  // the page opened on arriving. It is a question like any later one, so the
  // box is open under it from the start.
  const log = within(screen.getByRole('log'))
  expect(await log.findByText(/Which rules should I use/)).toBeInTheDocument()
  expect(screen.getByRole('textbox')).toBeEnabled()
  // The page is named by when the chat was opened and the head of its id,
  // not by thirty-two hex digits.
  expect(screen.getByRole('heading', { name: /^\d{4}-\d\d-\d\d-\d\d:\d\d:\d\d-opened$/ })).toBeInTheDocument()
  // The rules are prepared answers like any other question's: one press.
  await user.click(await log.findByRole('button', { name: 'D&D 2014 v1.0.0' }))
  // The answer is a message of the player's, and the next question follows it.
  // The question it answers stays where it was asked, and can be answered
  // again until the first message makes the rules final.
  await waitFor(() => expect(log.getAllByText('D&D 2014 v1.0.0').length).toBeGreaterThan(1))
  expect(log.getByRole('button', { name: 'D&D 2014 v1.0.0' })).toBeEnabled()
  expect(log.getByText(/attach, or would you rather describe/)).toBeInTheDocument()
  // Describing needs no button: the box is simply open. Attaching is a quiet
  // control inside it, which the file then replaces.
  expect(screen.getByRole('textbox')).toBeEnabled()
  expect(screen.getByRole('button', { name: 'Attach a sheet' })).toBeInTheDocument()
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
  expect(log.getByRole('button', { name: 'D&D 2014 v1.0.0' })).toBeDisabled()
  // Three writes, one per step: the chat, its rules, its first message.
  expect(writes).toHaveLength(3)
  expect(writes[0]?.url).toContain('folder=folder1')
  expect(writes[1]?.body).toMatchObject({ action: 'rules', revision: 1, rules: RULES })
  const form = writes[2]?.body as FormData
  expect(form.get('instructions')).toBe('Keep the custom items')
  expect(form.get('revision')).toBe('2')
  expect((form.get('files') as File).name).toBe('hero.txt')
})

// A question with buttons under it is answered by pressing one or by writing.
// The first question is no exception, though no model is there to read the
// reply: the page reads which pack it names.
it('takes the rules written into the message box as the answer to the opening question', async () => {
  const user = setupUser()
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  const log = within(screen.getByRole('log'))
  expect(await log.findByText(/Which rules should I use/)).toBeInTheDocument()
  await user.type(screen.getByRole('textbox'), 'd&d 2014{Enter}')
  // The same answer the button gives, and nothing sent besides: the words
  // were the answer, not a description of a character.
  expect(await log.findByText(/attach, or would you rather describe/)).toBeInTheDocument()
  expect(screen.getByRole('textbox')).toHaveValue('')
  expect(writes).toHaveLength(2)
  expect(writes[1]?.body).toMatchObject({ action: 'rules', revision: 1, rules: RULES })
})

it('starts from a first message that came before the rules, and says which it took', async () => {
  const user = setupUser()
  const { container } = renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard']}>
      <Routes>
        <Route path="/ai-wizard" element={<ImportCharacterScreen />} />
        <Route path="/ai-wizard/:sessionId" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  const log = within(screen.getByRole('log'))
  expect(await log.findByText(/Which rules should I use/)).toBeInTheDocument()
  // The sheet can be attached before anything is answered.
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')
  if (!input) throw new Error('File input missing')
  await user.upload(input, new File(['Hero'], 'hero.txt', { type: 'text/plain' }))
  await user.type(screen.getByRole('textbox'), 'Zephyr the bard')
  await user.click(screen.getByRole('button', { name: 'Send' }))
  await screen.findByText('Draft ready')
  // Two writes: the chat and its first message. No rules were sent; the
  // server took its own, and the assistant says so under the message rather
  // than the page pretending the player picked them.
  expect(writes).toHaveLength(2)
  const first = writes[1]?.body as FormData
  expect(first.get('revision')).toBe('1')
  const said = screen.getByRole('log').textContent ?? ''
  expect(said).toMatch(/Zephyr the bard.*I’ll use D&D 2014 v1\.0\.0\..*Draft ready/)
  expect(said).not.toMatch(/would you rather describe/)
})

it('lets a message be sent only on the player\'s turn, and keeps the opening in the log', async () => {
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
  await act(() =>
    Stream.emit('snapshot', {
      ...VIEW,
      session: {
        ...VIEW.session,
        revision: 6,
        status: 'running',
        events: [{ id: 8, kind: 'opening' }, { id: 9, kind: 'rules', data: { packs: [{ id: 'srd-2014', version: '1.0.0' }] } }, ...VIEW.session.events],
      },
    }),
  )
  // While the assistant works the box stays open; only sending waits.
  await waitFor(() => expect(screen.getByRole('status', { name: 'The assistant is working…' })).toBeInTheDocument())
  expect(screen.getByRole('textbox')).toBeEnabled()
  // Send's place is Stop's while a turn is in flight, and pressing it sends
  // the stop control.
  expect(screen.queryByRole('button', { name: 'Send' })).toBeNull()
  await user.click(screen.getByRole('button', { name: 'Stop' }))
  await waitFor(() => expect(writes.map((write) => (write.body as { action: string }).action)).toEqual(['stop']))
  writes = []
  // The opening is part of the log like everything after it.
  const transcript = screen.getByRole('log')
  expect(transcript.textContent).toMatch(/Which rules should I use.*D&D 2014 v1\.0\.0.*describe the character.*Please import Zephyr/)
  await act(() =>
    Stream.emit('snapshot', {
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

  // The message box is a block of its own under the transcript, not part of it.
  expect(within(screen.getByRole('log')).queryByLabelText('Message the assistant')).not.toBeInTheDocument()
  expect(screen.getByLabelText('Message the assistant')).toBeInTheDocument()
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
  await act(() =>
    Stream.emit('snapshot', {
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

it('takes the end of a turn from the poll so the next reply can be sent', async () => {
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
  await act(() =>
    Stream.emit('snapshot', {
      ...VIEW,
      session: { ...VIEW.session, status: 'running', revision: 6 },
    }),
  )
  // The answer can be written while the assistant is still working, and sent
  // once it has finished.
  await user.type(await screen.findByLabelText('Message the assistant'), 'My answer')
  expect(screen.queryByRole('button', { name: 'Send' })).toBeNull()
  await act(() =>
    Stream.emit('snapshot', {
      ...VIEW,
      session: { ...VIEW.session, status: 'waiting', revision: 7 },
    }),
  )
  await waitFor(() => expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled())
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
  await act(() =>
    Stream.emit('snapshot', {
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

it('shows what arrives in one answer a message at a time, and its answers after the last', async () => {
  pacing.on = true
  renderAt(
    'desktop',
    <MemoryRouter initialEntries={['/ai-wizard/session1']}>
      <Routes>
        <Route path="/ai-wizard/:sessionId" element={<ImportCharacterScreen />} />
      </Routes>
    </MemoryRouter>,
  )
  // What was there when the chat was opened is history: shown at once.
  await screen.findByText('Draft ready')
  await act(() =>
    Stream.emit('snapshot', {
      ...VIEW,
      session: {
        ...VIEW.session,
        status: 'waiting',
        revision: 6,
        events: [
          ...VIEW.session.events,
          { id: 3, kind: 'assistant', text: 'I have read the whole sheet' },
          { id: 4, kind: 'question', text: 'Which subclass is it?', options: ['Champion'] },
        ],
      },
    }),
  )
  // Both arrived together; neither is on screen whole, and the dots are.
  expect(screen.queryByText('I have read the whole sheet')).not.toBeInTheDocument()
  expect(screen.queryByText('Which subclass is it?')).not.toBeInTheDocument()
  expect(screen.getByRole('status', { name: 'The assistant is working…' })).toBeInTheDocument()
  // The first is typed out before the second begins.
  await screen.findByText('I have read the whole sheet')
  expect(screen.queryByText('Which subclass is it?')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Champion' })).not.toBeInTheDocument()
  await screen.findByText('Which subclass is it?')
  await waitFor(() => expect(screen.getByRole('button', { name: 'Champion' })).toBeEnabled())
})
