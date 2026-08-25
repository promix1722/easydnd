import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'

import { CharacterListScreen } from './CharacterListScreen'

/**
 * The party list, with folders.
 *
 * The fetch stub answers by path rather than by call order, because this screen
 * fires two requests at once and their order is not something the screen
 * promises. Every assertion below is about a request that was made, not about
 * when it was made relative to the other one.
 */

const DEFAULT_FOLDER = { id: 'fld_000001', name: 'Default', default: true }
const CAMPAIGN = { id: 'fld_000002', name: 'Campaign', default: false }

const STUB_ID = 'chr_000009'

const ADA = { id: 'chr_000001', folder: DEFAULT_FOLDER.id, name: 'Ada', level: 3, classes: [] }
const BRAM = { id: 'chr_000002', folder: CAMPAIGN.id, name: 'Bram', level: 1, classes: [] }

interface Call {
  url: string
  method: string
  body: string
}

let calls: Call[] = []

function mockApi() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      calls.push({ url, method: init?.method ?? 'GET', body: String(init?.body ?? '') })

      const json = (value: unknown, status = 200) =>
        new Response(status === 204 ? null : JSON.stringify(value), {
          status,
          headers: { 'Content-Type': 'application/json' },
        })

      if (url.startsWith('/v1/folders')) {
        if ((init?.method ?? 'GET') !== 'GET') return json({ ...CAMPAIGN }, 201)
        return json({ folders: [DEFAULT_FOLDER, CAMPAIGN] })
      }
      if (url.includes('/copy')) return json({ id: 'chr_000003', seq: 2, sheet: {} }, 201)
      // Ahead of the catch-all below: the stub answers with a character, not
      // the 204 every other write on this screen returns.
      if (url.includes('/v1/characters/stub')) return json({ id: STUB_ID, seq: 9, sheet: {} }, 201)
      if ((init?.method ?? 'GET') !== 'GET') return json(null, 204)
      if (url.includes(`folder=${CAMPAIGN.id}`)) return json({ characters: [BRAM] })
      if (url.includes(`folder=${DEFAULT_FOLDER.id}`)) return json({ characters: [ADA] })
      return json({ characters: [ADA, BRAM] })
    }),
  )
}

function renderList(viewport: 'mobile' | 'desktop') {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="/" element={<CharacterListScreen />} />
        <Route path="/characters/new" element={<div>new character screen</div>} />
        <Route path="/characters/import" element={<div>import screen</div>} />
        <Route path="/characters/:id" element={<div>character sheet</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

/**
 * The row for one character: a table row on desktop, a card on mobile. Scoping
 * to it is what keeps "Ada is in Default" from matching the word Default
 * wherever else it appears -- the filter's option list, for one.
 */
function rowOf(name: string): HTMLElement {
  const cell = screen.getByText(name)
  const row = cell.closest('tr, [class*="mantine-Card-root"]')
  if (row === null) throw new Error(`no row around ${name}`)
  return row as HTMLElement
}

/** The requests made to one path so far, newest last. */
function requestsTo(fragment: string): Call[] {
  return calls.filter((c) => c.url.includes(fragment))
}

/**
 * The single request expected to one path, failing loudly when there is not
 * exactly one. Returning it typed saves every caller an index and a null check.
 */
function onlyRequestTo(fragment: string, method: string): Call {
  const matching = requestsTo(fragment).filter((c) => c.method === method)
  expect(matching).toHaveLength(1)
  return matching[0] as Call
}

beforeEach(() => {
  calls = []
  mockApi()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('CharacterListScreen (%s)', (viewport) => {
  it('lists every character and names the folder each is in', async () => {
    renderList(viewport)

    expect(await screen.findByText('Ada')).toBeInTheDocument()
    expect(screen.getByText('Bram')).toBeInTheDocument()
    // The folder column only earns its place when the listing spans folders,
    // which is what "All characters" is.
    expect(within(rowOf('Ada')).getByText('Default')).toBeInTheDocument()
    expect(within(rowOf('Bram')).getByText('Campaign')).toBeInTheDocument()
  })

  it('re-fetches with ?folder= when the filter changes', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('combobox', { name: 'Folder' }))
    await user.click(await screen.findByRole('option', { name: 'Campaign' }))

    await waitFor(() => {
      expect(requestsTo(`/v1/characters?folder=${CAMPAIGN.id}`)).toHaveLength(1)
    })
    expect(await screen.findByText('Bram')).toBeInTheDocument()
    expect(screen.queryByText('Ada')).not.toBeInTheDocument()
  })

  it('moves a character to another folder', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByLabelText('Actions for Ada'))
    await user.click(await screen.findByRole('menuitem', { name: 'Move...' }))

    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('combobox', { name: 'Folder' }))
    await user.click(await screen.findByRole('option', { name: 'Campaign' }))
    await user.click(within(dialog).getByRole('button', { name: 'Move' }))

    await waitFor(() => {
      const move = onlyRequestTo(`/v1/characters/${ADA.id}/folder`, 'PUT')
      expect(JSON.parse(move.body)).toEqual({ folder: CAMPAIGN.id })
    })
  })

  it('copies a character', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByLabelText('Actions for Ada'))
    await user.click(await screen.findByRole('menuitem', { name: 'Copy' }))

    await waitFor(() => {
      onlyRequestTo(`/v1/characters/${ADA.id}/copy`, 'POST')
    })
    // The list is re-read, so the copy shows up without a manual refresh.
    await waitFor(() => {
      expect(requestsTo('/v1/characters').length).toBeGreaterThan(1)
    })
  })

  it('confirms before deleting a character', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByLabelText('Actions for Ada'))
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))

    // Nothing has been sent yet: the dialog is the point.
    expect(requestsTo(`/v1/characters/${ADA.id}`).filter((c) => c.method === 'DELETE')).toHaveLength(
      0,
    )

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/cannot be undone/i)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))

    await waitFor(() => {
      onlyRequestTo(`/v1/characters/${ADA.id}`, 'DELETE')
    })
  })

  it('creates a folder', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Manage folders' }))
    await user.type(await screen.findByLabelText('New folder'), 'Retired')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      const created = onlyRequestTo('/v1/folders', 'POST')
      expect(JSON.parse(created.body)).toEqual({ name: 'Retired' })
    })
  })

  // The whole reason this dialog exists. Deleting a folder destroys the
  // characters in it, so the confirmation has to say how many.
  it('names the character count before deleting a folder', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Manage folders' }))
    const folders = await screen.findByRole('dialog')
    await user.click(within(folders).getByRole('button', { name: 'Delete' }))

    const confirm = await screen.findByText(/deletes the folder and the 1 character in it/i)
    expect(confirm).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Delete folder and 1/ }))
    await waitFor(() => {
      onlyRequestTo(`/v1/folders/${CAMPAIGN.id}`, 'DELETE')
    })
  })

  // The stub is a development convenience, and these two say it behaves like
  // the buttons beside it rather than like a special case. It renders here
  // because Vitest runs with import.meta.env.DEV set; that a production bundle
  // omits it is not something a test in this environment can observe.
  it('creates a stub character and opens its sheet', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Stub' }))

    // No body: what the stub makes is the server's to decide.
    const request = onlyRequestTo('/v1/characters/stub', 'POST')
    expect(request.body).toBe('')
    // The sheet, not the build screen -- unlike an import, this character is
    // finished, so there is nothing to send the player back to answer.
    expect(await screen.findByText('character sheet')).toBeInTheDocument()
  })

  it('files the stub into the folder the list is filtered to', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('combobox', { name: 'Folder' }))
    await user.click(await screen.findByRole('option', { name: 'Campaign' }))
    await screen.findByText('Bram')

    await user.click(screen.getByRole('button', { name: 'Stub' }))

    await waitFor(() => {
      onlyRequestTo(`/v1/characters/stub?folder=${CAMPAIGN.id}`, 'POST')
    })
  })

  // The default folder is the one an account is guaranteed to have, so it
  // must not even offer the control.
  it('offers no delete for the default folder', async () => {
    const user = userEvent.setup()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Manage folders' }))
    const dialog = await screen.findByRole('dialog')

    expect(within(dialog).getAllByRole('button', { name: 'Rename' })).toHaveLength(2)
    expect(within(dialog).getAllByRole('button', { name: 'Delete' })).toHaveLength(1)
  })
})
