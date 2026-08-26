import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { CharacterListScreen } from './CharacterListScreen'

/**
 * The party list, drawn as folders.
 *
 * The fetch stub answers by path rather than by call order, because this screen
 * fires two requests at once and their order is not something the screen
 * promises. Every assertion below is about a request that was made, not about
 * when it was made relative to the other one.
 */

const DEFAULT_FOLDER = { id: 'fld_000001', name: 'Default', default: true }
const CAMPAIGN = { id: 'fld_000002', name: 'Campaign', default: false }
// A third folder, because two movable ones is the least it takes to have an
// order at all: with one there is nothing to move it past.
const RETIRED = { id: 'fld_000003', name: 'Retired', default: false }

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
      const method = init?.method ?? 'GET'
      calls.push({ url, method, body: String(init?.body ?? '') })

      const json = (value: unknown, status = 200) =>
        new Response(status === 204 ? null : JSON.stringify(value), {
          status,
          headers: { 'Content-Type': 'application/json' },
        })

      if (url.startsWith('/v1/folders')) {
        if (method === 'GET') return json({ folders: [DEFAULT_FOLDER, CAMPAIGN, RETIRED] })
        if (method === 'POST') return json({ ...CAMPAIGN }, 201)
        if (method === 'PATCH') return json({ ...CAMPAIGN })
        // PUT /v1/folders/order and DELETE /v1/folders/{id}.
        return json(null, 204)
      }
      if (url.includes('/copy')) return json({ id: 'chr_000003', seq: 2, sheet: {} }, 201)
      // Ahead of the catch-all below: the stub answers with a character, not
      // the 204 every other write on this screen returns.
      if (url.includes('/v1/characters/stub')) return json({ id: STUB_ID, seq: 9, sheet: {} }, 201)
      if (method !== 'GET') return json(null, 204)
      return json({ characters: [ADA, BRAM] })
    }),
  )
}

/**
 * A destination that prints the query it was reached with.
 *
 * The `?folder=` is the whole assertion for the add buttons -- which folder a
 * press files into -- and a route element that only said "new character screen"
 * could not tell one press from another.
 */
function Landed({ label }: { label: string }) {
  return <div>{`${label}${useLocation().search}`}</div>
}

function renderList(viewport: 'mobile' | 'desktop') {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="/" element={<CharacterListScreen />} />
        <Route path="/characters/new" element={<Landed label="new character screen" />} />
        <Route path="/characters/import" element={<Landed label="import screen" />} />
        <Route path="/characters/:id" element={<div>character sheet</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

/**
 * What one folder is holding, scoped to the panel it is drawn in.
 *
 * The panel body carries the id its heading points `aria-controls` at, which is
 * the same handle a screen reader follows -- so a test that finds a character
 * through it is asserting the thing the markup actually claims.
 */
function folderBody(id: string): HTMLElement {
  const body = document.getElementById(`folder-${id}`)
  if (body === null) throw new Error(`folder ${id} is not open`)
  return body
}

/** The header of one folder: the element a drag starts on. */
function folderHeader(name: string): HTMLElement {
  const toggle = screen.getByRole('button', { name: new RegExp(`(Collapse|Expand) ${name}`) })
  const header = toggle.closest('[draggable="true"]')
  if (header === null) throw new Error(`${name} is not draggable`)
  return header as HTMLElement
}

/** Opens one folder's action menu and returns it. */
async function openMenu(user: ReturnType<typeof setupUser>, name: string): Promise<HTMLElement> {
  await user.click(screen.getByRole('button', { name: `Actions for ${name}` }))
  return await screen.findByRole('menu')
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

describe.each(['mobile', 'desktop'] as const)('CharacterListScreen (%s)', (viewport) => {
  it('draws each folder over the characters filed in it', async () => {
    renderList(viewport)

    expect(await screen.findByText('Ada')).toBeInTheDocument()

    // Not "Ada is on the page and says Default beside her" -- Ada is *inside*
    // the Default folder's panel, which is the claim the layout now makes.
    expect(within(folderBody(DEFAULT_FOLDER.id)).getByText('Ada')).toBeInTheDocument()
    expect(within(folderBody(CAMPAIGN.id)).getByText('Bram')).toBeInTheDocument()
    expect(within(folderBody(CAMPAIGN.id)).queryByText('Ada')).not.toBeInTheDocument()

    // An empty folder still draws, and says so.
    expect(within(folderBody(RETIRED.id)).getByText(/Nothing in this folder yet/)).toBeInTheDocument()

    // One request for every character rather than one per folder: a listing
    // already carries the folder each row is in.
    expect(requestsTo('/v1/characters').filter((c) => c.method === 'GET')).toHaveLength(1)
  })

  it('moves a character to another folder', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Move Ada' }))

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
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Copy Ada' }))

    await waitFor(() => {
      onlyRequestTo(`/v1/characters/${ADA.id}/copy`, 'POST')
    })
    // The list is re-read, so the copy shows up without a manual refresh.
    await waitFor(() => {
      expect(requestsTo('/v1/characters').length).toBeGreaterThan(1)
    })
  })

  it('confirms before deleting a character', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Delete Ada' }))

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
})

/**
 * The rest of the screen, at one width.
 *
 * The block above stays at both because `DataList` draws a table on a desktop
 * and cards on a phone, and `ModalSheet` swaps `Modal` for `Drawer` -- the row
 * actions live inside `DataList`, so anything that presses one belongs up
 * there. Nothing here touches either: what these press is a folder's heading,
 * its action menu or the New folder button, and what they assert is the request
 * that went out or where a press navigated to -- the same markup at any width.
 * The folder dialogs are `ModalSheet`s, and that swap is asserted on its own
 * terms in `src/ui/ModalSheet.test.tsx` rather than several more times here.
 * See docs/web.md.
 */
describe('CharacterListScreen', () => {
  const viewport = 'desktop'

  it('collapses a folder and opens it again', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Collapse Campaign' }))
    expect(screen.queryByText('Bram')).not.toBeInTheDocument()
    // The count is what a collapsed folder still says about its contents.
    expect(screen.getByRole('button', { name: 'Expand Campaign' })).toBeInTheDocument()
    // And its neighbours did not close with it.
    expect(screen.getByText('Ada')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Expand Campaign' }))
    expect(await screen.findByText('Bram')).toBeInTheDocument()
  })

  // Where you press decides where the character lands. There is no filter to
  // set first, which is the whole point of a pair of these per folder.
  it('carries the folder its add button sits under', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'New character in Campaign' }))

    expect(
      await screen.findByText(`new character screen?folder=${CAMPAIGN.id}`),
    ).toBeInTheDocument()
  })

  it('carries the folder its import button sits under', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Import into Retired' }))

    expect(await screen.findByText(`import screen?folder=${RETIRED.id}`)).toBeInTheDocument()
  })

  // The order goes out whole rather than as a move, so this asserts the list
  // that was sent and not a delta.
  it('moves a folder down and sends the whole new order', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    const menu = await openMenu(user, 'Campaign')
    await user.click(within(menu).getByRole('menuitem', { name: 'Move down' }))

    await waitFor(() => {
      const sent = onlyRequestTo('/v1/folders/order', 'PUT')
      // The default folder is not in it: it leads the listing whatever
      // anybody moves, and the server refuses an order that names it.
      expect(JSON.parse(sent.body)).toEqual({ folders: [RETIRED.id, CAMPAIGN.id] })
    })
  })

  // The ends of the run have nowhere to go, and the default folder never moves
  // at all -- so neither offers the control.
  it('offers no move past the ends, and none on the default folder', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    const first = await openMenu(user, 'Campaign')
    expect(within(first).queryByRole('menuitem', { name: 'Move up' })).not.toBeInTheDocument()
    expect(within(first).getByRole('menuitem', { name: 'Move down' })).toBeInTheDocument()
    await user.keyboard('{Escape}')

    const last = await openMenu(user, 'Retired')
    expect(within(last).getByRole('menuitem', { name: 'Move up' })).toBeInTheDocument()
    expect(within(last).queryByRole('menuitem', { name: 'Move down' })).not.toBeInTheDocument()
    await user.keyboard('{Escape}')

    const def = await openMenu(user, 'Default')
    expect(within(def).queryByRole('menuitem', { name: /^Move/ })).not.toBeInTheDocument()
  })

  it('takes a folder by dragging it as well as by the menu', async () => {
    renderList(viewport)
    await screen.findByText('Ada')

    // The mouse gesture and the menu are the same operation: this is the one
    // a phone cannot make, which is why the menu carries it too.
    fireEvent.dragStart(folderHeader('Retired'))
    fireEvent.drop(folderHeader('Campaign'))

    await waitFor(() => {
      const sent = onlyRequestTo('/v1/folders/order', 'PUT')
      expect(JSON.parse(sent.body)).toEqual({ folders: [RETIRED.id, CAMPAIGN.id] })
    })
  })

  // The default folder leads the listing, so there is nothing to take hold of.
  it('gives the default folder no grip', async () => {
    renderList(viewport)
    await screen.findByText('Ada')

    expect(
      screen.getByRole('button', { name: 'Collapse Default' }).closest('[draggable="true"]'),
    ).toBeNull()
  })

  // The stub is a development convenience, and these two say it behaves like
  // the buttons beside it rather than like a special case. It renders here
  // because Vitest runs with import.meta.env.DEV set; that a production bundle
  // omits it is not something a test in this environment can observe.
  it('creates a stub character and opens its sheet', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Stub in Default' }))

    // No body: what the stub makes is the server's to decide.
    const request = onlyRequestTo('/v1/characters/stub', 'POST')
    expect(request.body).toBe('')
    // The sheet, not the build screen -- unlike an import, this character is
    // finished, so there is nothing to send the player back to answer.
    expect(await screen.findByText('character sheet')).toBeInTheDocument()
  })

  it('files the stub into the folder whose button was pressed', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'Stub in Campaign' }))

    await waitFor(() => {
      onlyRequestTo(`/v1/characters/stub?folder=${CAMPAIGN.id}`, 'POST')
    })
  })

  it('creates a folder', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'New folder' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Name'), 'Retired')
    await user.click(within(dialog).getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      const created = onlyRequestTo('/v1/folders', 'POST')
      expect(JSON.parse(created.body)).toEqual({ name: 'Retired' })
    })
  })

  // The keys a phone's keyboard offers have to do something. These press Enter
  // rather than the button, which is what a soft keyboard's Go sends and what
  // nothing here answered before the dialogs became real forms.
  it('creates a folder from the keyboard', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'New folder' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Name'), 'Retired{Enter}')

    await waitFor(() => {
      const created = onlyRequestTo('/v1/folders', 'POST')
      expect(JSON.parse(created.body)).toEqual({ name: 'Retired' })
    })
  })

  it('renames a folder from the keyboard', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    const menu = await openMenu(user, 'Campaign')
    await user.click(within(menu).getByRole('menuitem', { name: 'Rename' }))

    const dialog = await screen.findByRole('dialog')
    const name = within(dialog).getByLabelText('Name')
    await user.clear(name)
    await user.type(name, 'Tuesday game{Enter}')

    await waitFor(() => {
      const renamed = onlyRequestTo(`/v1/folders/${CAMPAIGN.id}`, 'PATCH')
      expect(JSON.parse(renamed.body)).toEqual({ name: 'Tuesday game' })
    })
  })

  // An empty name is refused by the server, so the form must not send one --
  // the disabled button was the only thing saying so, and Enter went round it.
  it('sends nothing when the name is blank', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    await user.click(screen.getByRole('button', { name: 'New folder' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Name'), '{Enter}')

    expect(requestsTo('/v1/folders').filter((c) => c.method === 'POST')).toHaveLength(0)
  })

  it('renames a folder, the default one included', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    const menu = await openMenu(user, 'Default')
    await user.click(within(menu).getByRole('menuitem', { name: 'Rename' }))

    const dialog = await screen.findByRole('dialog')
    const name = within(dialog).getByLabelText('Name')
    // It opens on the name it is about to change, rather than empty.
    expect(name).toHaveValue('Default')
    await user.clear(name)
    await user.type(name, 'Active')
    await user.click(within(dialog).getByRole('button', { name: 'Rename' }))

    await waitFor(() => {
      const renamed = onlyRequestTo(`/v1/folders/${DEFAULT_FOLDER.id}`, 'PATCH')
      expect(JSON.parse(renamed.body)).toEqual({ name: 'Active' })
    })
  })

  // The whole reason this dialog exists. Deleting a folder destroys the
  // characters in it, so the confirmation has to say how many -- and the count
  // is a filter over what is already on screen rather than a third request.
  it('names the character count before deleting a folder', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    const menu = await openMenu(user, 'Campaign')
    await user.click(within(menu).getByRole('menuitem', { name: 'Delete' }))

    expect(
      await screen.findByText(/deletes the folder and the 1 character in it/i),
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Delete folder and 1/ }))
    await waitFor(() => {
      onlyRequestTo(`/v1/folders/${CAMPAIGN.id}`, 'DELETE')
    })
  })

  // The default folder is the one an account is guaranteed to have, so it must
  // not even offer the control.
  it('offers no delete for the default folder', async () => {
    const user = setupUser()
    renderList(viewport)
    await screen.findByText('Ada')

    const def = await openMenu(user, 'Default')
    expect(within(def).queryByRole('menuitem', { name: 'Delete' })).not.toBeInTheDocument()
    await user.keyboard('{Escape}')

    const other = await openMenu(user, 'Campaign')
    expect(within(other).getByRole('menuitem', { name: 'Delete' })).toBeInTheDocument()
  })
})
