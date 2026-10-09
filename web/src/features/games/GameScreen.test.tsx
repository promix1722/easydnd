import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { GameDetail, GroupRole } from '@/lib/api'
import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { pressRowAction } from '@/test/rows'
import { setupUser } from '@/test/user'
import type { Viewport } from '@/test/viewport'

import { GameScreen } from './GameScreen'

function gameAs(role: GroupRole): GameDetail {
  return {
    id: 'gam_1',
    group_id: 'grp_1',
    name: 'Thursday night',
    created_at: '2026-01-01T00:00:00Z',
    role,
    entries: [{
      id: 'pc_chr_1', kind: 'player', character_id: 'chr_1', name: 'Ada', can_edit: role !== 'player', locked: false,
      hp: 20, temp_hp: 0, initiative: null, tags: [],
      stats: { name: 'Ada', max_hp: 24, armor_class: 15, spellcasting: [],
        speeds: [{ kind: 'walking', distance: 30 }], senses: [],
        abilities: { scores: { str: 10, dex: 16, con: 12, int: 10, wis: 12, cha: 10 },
          modifiers: { str: 0, dex: 3, con: 1, int: 0, wis: 1, cha: 0 } } },
    }],
    characters: [
      {
        id: 'chr_1',
        owner_id: 'player-1',
        name: 'Ada',
        level: 3,
        classes: [{ class: 'rogue', level: 3 }],
      },
    ],
  }
}

function renderGame(viewport: Viewport, game: GameDetail) {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify(game), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )
  return renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter initialEntries={['/games/gam_1']}>
        <Routes>
          <Route path="/games/:id" element={<GameScreen />} />
          <Route path="/games" element={<div>your games</div>} />
          <Route path="/groups/:id" element={<div>the group</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  )
}

const originalHitTest = document.elementFromPoint
afterEach(() => {
  document.elementFromPoint = originalHitTest
  vi.unstubAllGlobals()
})

/** A phone prints the hit points a character has; a wide screen, out of how many. */
const shown = (viewport: 'mobile' | 'desktop', current: number) => viewport === 'mobile' ? new RegExp(`^${current}$`) : `${current} / 24`

describe.each(['mobile', 'desktop'] as const)('GameScreen (%s)', (viewport) => {
  it('shows a DM the roster and the controls that change it', async () => {
    renderGame(viewport, gameAs('dm'))

    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    expect(screen.getByText('Ada')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add from group' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Add my characters' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
    // A row's action, so it is a named button on a desktop and one item inside
    // the row's menu on a phone -- either way, exactly one row offers it.
    expect(screen.getAllByRole('button', { name: /^Actions for / })).toHaveLength(1)
    // The blanket control is gone: seating everyone is ticking Everyone in
    // the group picker, not a second button beside it.
    expect(screen.queryByRole('button', { name: 'Add everyone' })).not.toBeInTheDocument()
  })

  it('says what your rank is at the table it is played at', async () => {
    renderGame(viewport, gameAs('owner'))

    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    // A game is reached from its own section, so the page has to say what you
    // are at its table rather than leaving you to remember.
    expect(screen.getByText('Owner')).toBeInTheDocument()
  })

  it('shows a player the roster and nothing that would come back 403', async () => {
    renderGame(viewport, gameAs('player'))

    await waitFor(() => expect(screen.getByText('Thursday night')).toBeInTheDocument())
    expect(screen.getByText('Ada')).toBeInTheDocument()
    for (const label of ['Add from group', 'Add my characters', 'Rename', 'Delete']) {
      expect(screen.queryByRole('button', { name: label })).not.toBeInTheDocument()
    }
    // And no way to unseat anybody: a player's row carries no control at all,
    // at either width.
    expect(screen.queryAllByRole('button', { name: /^Actions for / })).toHaveLength(0)
  })
})

describe.each(['mobile', 'desktop'] as const)('game tracking (%s)', (viewport) => {
  it('shows compact stats and keeps monster stats out of the player view', async () => {
    const game = gameAs('player')
    game.entries[0]!.initiative = 0
    game.entries.push({ id: 'monster', kind: 'monster', name: 'Goblin', initiative: 19, can_edit: false })
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const hero = within(screen.getByRole('article', { name: 'Ada' }))
    expect(hero.getByText('HP').nextElementSibling).toHaveTextContent(shown(viewport, 20))
    expect(screen.queryByText('No tags')).not.toBeInTheDocument()
    expect(hero.queryByRole('group', { name: 'Tags' })).not.toBeInTheDocument()
    const vitals = hero.getByText('HP').parentElement!.parentElement!
    expect(within(vitals).getAllByText(/^(HP|Temp HP|AC|I|Spell DC)$/).map((label) => label.textContent))
      .toEqual(viewport === 'mobile' ? ['HP', 'Temp HP', 'AC', 'I', 'Spell DC'] : ['HP', 'Temp HP', 'AC', 'Spell DC'])
    if (viewport === 'mobile') {
      // Initiative is a letter on the vitals line, not a number beside the name.
      expect(hero.getByText('I').nextElementSibling).toHaveTextContent('0')
      expect(hero.queryByLabelText('Initiative')).not.toBeInTheDocument()
      expect(hero.queryByText('STR')).not.toBeInTheDocument()
      expect(hero.queryByText('Speed')).not.toBeInTheDocument()
      expect(hero.getByText('AC')).toBeInTheDocument()
      expect(hero.getByText('Temp HP')).toBeInTheDocument()
      expect(hero.getByText('Spell DC')).toBeInTheDocument()
      await setupUser().click(hero.getByRole('button', { name: 'Show details' }))
      expect(hero.getByRole('button', { name: 'Hide details' })).toHaveAttribute('aria-expanded', 'true')
      expect(hero.queryByText('Initiative')).not.toBeInTheDocument()
    } else {
      expect(hero.getByText('Initiative').nextElementSibling).toHaveTextContent('0')
    }
    expect(hero.getByText('STR')).toBeInTheDocument()
    expect(hero.getByText('Normal')).toBeInTheDocument()
    if (viewport === 'mobile') {
      await setupUser().click(hero.getByRole('button', { name: 'Hide details' }))
      expect(hero.queryByText('STR')).not.toBeInTheDocument()
      expect(hero.getByText('HP').nextElementSibling).toHaveTextContent(shown(viewport, 20))
    }
    const goblin = within(screen.getByRole('article', { name: 'Goblin' }))
    expect(goblin.queryByText('HP:')).not.toBeInTheDocument()
    expect(goblin.queryByText('No tags')).not.toBeInTheDocument()
    expect(goblin.queryByLabelText('Initiative')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Order' })).not.toBeInTheDocument()
  })

  it('opens an entry\'s consumables in a dialog, spends one and rests the table', async () => {
    const game = gameAs('dm')
    game.entries[0]!.can_edit = true
    game.entries[0]!.resources = [
      { id: 'spell-slots/1', group: 'spell-slots', max: 2, used: 0, slot_level: 1 },
      { id: 'hit-dice/rogue', group: 'hit-dice', max: 1, used: 1, dice: '1d8' },
    ]
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
      const body = options?.body ? JSON.parse(options.body as string) : {}
      for (const pool of game.entries[0]!.resources!) pool.used = options?.method === 'POST' ? 0 : body.used?.[pool.id] ?? pool.used
      return new Response(JSON.stringify(game), { status: 200, headers: { 'Content-Type': 'application/json' } })
    })
    vi.stubGlobal('fetch', fetch)
    const user = setupUser()
    await pressRowAction(viewport, 'Ada', 'Consumable slots')
    const dialog = within(await screen.findByRole('dialog', { name: 'Consumable slots' }))
    expect(dialog.getByRole('img', { name: 'Hit Dice (d8): 0 of 1 left' })).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: 'Give one back: Spell slots, level 1' })).toBeDisabled()
    await user.click(dialog.getByRole('button', { name: 'Spend one: Spell slots, level 1' }))
    await waitFor(() => expect(dialog.getByRole('img', { name: 'Spell slots, level 1: 1 of 2 left' })).toBeInTheDocument())
    const spend = fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')!
    expect(String(spend[0])).toMatch(/\/games\/gam_1\/entries\/pc_chr_1\?locale=en$/)
    expect(JSON.parse(spend[1]!.body as string)).toEqual({ used: { 'spell-slots/1': 1 } })
    await user.click(dialog.getByRole('button', { name: 'Close' }))

    // Two rests, one dialog each, and the kind travels with the request.
    for (const [label, kind] of [['Long rest', 'long'], ['Short rest', 'short']] as const) {
      await user.click(screen.getByRole('button', { name: label }))
      await user.click(screen.getByRole('button', { name: 'Apply' }))
      await waitFor(() => expect(fetch.mock.calls.some(([url, options]) => options?.method === 'POST' && String(url).includes(`/games/gam_1/rest?kind=${kind}&`))).toBe(true))
    }
  })

  it('lets an unlocked owner edit only changed game fields in the dialog', async () => {
    const game = gameAs('player')
    game.entries[0]!.can_edit = true
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
      if (options?.method === 'PATCH') {
        const patch = JSON.parse(options.body as string)
        Object.assign(game.entries[0]!, patch)
      }
      return new Response(JSON.stringify(game), { status: 200, headers: { 'Content-Type': 'application/json' } })
    })
    vi.stubGlobal('fetch', fetch)
    await pressRowAction(viewport, 'Ada', 'Edit')
    const user = setupUser()
    await user.clear(screen.getByLabelText('Hit points'))
    await user.type(screen.getByLabelText('Hit points'), '18')
    await user.type(screen.getByLabelText('Initiative'), '17')
    expect(within(screen.getByRole('dialog')).queryByRole('textbox', { name: 'Tags' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() => expect(fetch).toHaveBeenCalled())
    const request = fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')!
    expect(String(request[0])).toMatch(/\/games\/gam_1\/entries\/pc_chr_1\?locale=en$/)
    expect(JSON.parse(request[1]!.body as string)).toEqual({ hp: 18, initiative: 17 })
    await waitFor(() => expect(within(screen.getByRole('article', { name: 'Ada' })).getByText('HP').nextElementSibling).toHaveTextContent(shown(viewport, 18)))
  })

  it('keeps typed drafts during refresh and disables saving when the master locks the entry', async () => {
    const game = gameAs('player')
    game.entries[0]!.can_edit = true
    renderGame(viewport, game)
    await screen.findByText('Ada')
    await pressRowAction(viewport, 'Ada', 'Edit')
    const user = setupUser()
    await user.clear(screen.getByLabelText('Hit points'))
    await user.type(screen.getByLabelText('Hit points'), '18')
    const locked = structuredClone(game)
    Object.assign(locked.entries[0]!, { hp: 9, locked: true, can_edit: false })
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(locked), { status: 200 })))
    await act(async () => { window.dispatchEvent(new Event('focus')) })
    await screen.findByText('This entry is no longer editable.')
    expect(screen.getByLabelText('Hit points')).toHaveValue('18')
    expect(screen.getByRole('button', { name: 'Apply' })).toBeDisabled()
  })

  it('retains the last view after a background failure and recovers on retry', async () => {
    const game = gameAs('player')
    renderGame(viewport, game)
    await screen.findByText('Ada')
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('offline') }))
    await act(async () => { window.dispatchEvent(new Event('focus')) })
    await screen.findByText('Updates paused — retrying')
    expect(screen.getByText('Ada')).toBeInTheDocument()
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(game), { status: 200 })))
    await setupUser().click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.queryByText('Updates paused — retrying')).not.toBeInTheDocument())
  })

  it('gives the master controls for sorting, moving, locking and private monsters', async () => {
    renderGame(viewport, gameAs('dm'))
    await screen.findByText('Ada')
    expect(screen.getByRole('button', { name: 'Add NPC stub' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add prepared NPC' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Order' })).toBeInTheDocument()
    const fetch = vi.fn(async () => new Response(JSON.stringify(gameAs('dm')), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    await pressRowAction(viewport, 'Ada', 'Lock')
    await waitFor(() => expect(fetch).toHaveBeenCalled())
    const [, options] = fetch.mock.calls[0] as unknown as [string, RequestInit]
    expect(JSON.parse(options.body as string)).toEqual({ locked: true })
  })
})


describe.each(['mobile', 'desktop'] as const)('tracker ordering and layout (%s)', (viewport) => {
  function roster(role: GroupRole) {
    const game = gameAs(role)
    game.entries.push(
      { id: 'mon_a', kind: 'monster', name: 'Goblin', can_edit: role !== 'player' },
      { id: 'mon_b', kind: 'monster', name: 'Orc', can_edit: role !== 'player' },
    )
    return game
  }

  it('puts actions in menus at every width, with move choices only where they apply', async () => {
    renderGame(viewport, roster('dm'))
    await screen.findByText('Ada')
    expect(screen.queryByRole('button', { name: 'Remove Ada' })).not.toBeInTheDocument()
    const user = setupUser()
    await user.click(screen.getByRole('button', { name: 'Actions for Ada' }))
    const menu = within(await screen.findByRole('menu'))
    expect(menu.getByRole('menuitem', { name: 'Edit' })).toBeInTheDocument()
    expect(menu.getByRole('menuitem', { name: 'Lock' })).toBeInTheDocument()
    expect(menu.getByRole('menuitem', { name: 'Move down' })).toBeInTheDocument()
    expect(menu.queryByRole('menuitem', { name: 'Move up' })).not.toBeInTheDocument()
  })

  it('uses label/value pairs for vitals and abilities', async () => {
    renderGame(viewport, roster('dm'))
    await screen.findByText('Ada')
    const row = within(screen.getByRole('article', { name: 'Ada' }))
    expect(row.getByText('HP').nextElementSibling).toHaveTextContent(shown(viewport, 20))
    if (viewport === 'mobile') await setupUser().click(row.getByRole('button', { name: 'Show details' }))
    expect(row.getByText('DEX').nextElementSibling).toHaveTextContent('16 (+3)')
    expect(row.getByText('Spell DC').nextElementSibling).toHaveTextContent('—')
  })

  // A phone orders from the menu only: no grip is drawn there, for a DM either.
  it.runIf(viewport === 'mobile')('draws no drag grip on a phone', async () => {
    renderGame(viewport, roster('dm'))
    await screen.findByText('Ada')
    expect(screen.queryByRole('button', { name: /^Drag to reorder / })).not.toBeInTheDocument()
  })

  function grip(name: string) {
    return screen.getByRole('button', { name: `Drag to reorder ${name}` })
  }

  function movePointer(from: HTMLElement, target: Element, pointerId = 1) {
    document.elementFromPoint = () => target
    fireEvent.pointerDown(from, { isPrimary: true, clientX: 10, clientY: 10, pointerId, pointerType: viewport === 'mobile' ? 'touch' : 'mouse', button: 0 })
    fireEvent.pointerMove(from, { clientX: 100, clientY: 100, pointerId })
  }

  it.runIf(viewport === 'desktop')('moves an entry down with a mouse or touch grip and renders the persisted order', async () => {
    const game = roster('dm')
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
      if (options?.method === 'POST') {
        const body = JSON.parse(options.body as string)
        const from = game.entries.findIndex((entry) => entry.id === body.entry_id)
        const [entry] = game.entries.splice(from, 1)
        game.entries.splice(game.entries.findIndex((item) => item.id === body.before_id), 0, entry!)
      }
      return new Response(JSON.stringify(game), { status: 200 })
    })
    vi.stubGlobal('fetch', fetch)
    const handle = grip('Ada')
    movePointer(handle, screen.getByText('Goblin'))
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    expect(fetch.mock.calls.filter(([, options]) => options?.method === 'POST')).toHaveLength(1)
    await waitFor(() => expect(screen.getAllByRole('article').map((row) => row.getAttribute('aria-label'))).toEqual(['Goblin', 'Ada', 'Orc']))
    const posts = fetch.mock.calls.filter(([, options]) => options?.method === 'POST')
    expect(posts).toHaveLength(1)
    expect(String(posts[0]![0])).toContain('/games/gam_1/order')
    expect(JSON.parse(posts[0]![1]!.body as string)).toEqual({ entry_id: 'pc_chr_1', before_id: 'mon_b' })
  })

  it.runIf(viewport === 'desktop')('moves to the beginning and the reserved end target using stable IDs', async () => {
    const game = roster('dm')
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response(JSON.stringify(game), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    const last = grip('Orc')
    movePointer(last, screen.getByText('Ada'))
    fireEvent.pointerUp(last, { clientX: 100, clientY: 100, pointerId: 1 })
    await waitFor(() => expect(fetch.mock.calls.some(([, options]) => options?.method === 'POST')).toBe(true))
    expect(JSON.parse(fetch.mock.calls.find(([, options]) => options?.method === 'POST')![1]!.body as string))
      .toEqual({ entry_id: 'mon_b', before_id: 'pc_chr_1' })
    await waitFor(() => expect(screen.getByRole('button', { name: 'Drag to reorder Ada' })).toBeInTheDocument())
    fetch.mockClear()
    const first = grip('Ada')
    movePointer(first, document.querySelector('[data-game-drop-end]')!)
    fireEvent.pointerUp(first, { clientX: 100, clientY: 100, pointerId: 1 })
    await waitFor(() => expect(fetch.mock.calls.some(([, options]) => options?.method === 'POST')).toBe(true))
    expect(JSON.parse(fetch.mock.calls.find(([, options]) => options?.method === 'POST')![1]!.body as string))
      .toEqual({ entry_id: 'pc_chr_1', before_id: '' })
  })

  it.runIf(viewport === 'desktop')('does not reorder on a tap, a canceled gesture, or a drop outside the roster', async () => {
    const game = roster('dm')
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const fetch = vi.fn(async () => new Response(JSON.stringify(game), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    const handle = grip('Ada')
    document.elementFromPoint = () => screen.getByText('Orc')
    fireEvent.pointerDown(handle, { isPrimary: true, clientX: 10, clientY: 10, pointerId: 1 })
    fireEvent.pointerMove(handle, { clientX: 12, clientY: 12, pointerId: 1 })
    fireEvent.pointerUp(handle, { clientX: 12, clientY: 12, pointerId: 1 })
    movePointer(handle, screen.getByText('Orc'))
    fireEvent.pointerCancel(handle, { pointerId: 1 })
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    movePointer(handle, screen.getByText('Orc'))
    document.elementFromPoint = () => document.body
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    expect(fetch).not.toHaveBeenCalled()
  })

  it.runIf(viewport === 'desktop')('ignores right-click and another pointer without interrupting the active drag', async () => {
    const game = roster('dm')
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response(JSON.stringify(game), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    const handle = grip('Ada')
    document.elementFromPoint = () => screen.getByText('Orc')
    fireEvent.pointerDown(handle, { isPrimary: true, clientX: 10, clientY: 10, pointerId: 1, pointerType: 'mouse', button: 2 })
    fireEvent.pointerMove(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    expect(fetch).not.toHaveBeenCalled()
    movePointer(handle, screen.getByText('Orc'), 1)
    fireEvent.pointerCancel(handle, { pointerId: 2 })
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 2 })
    expect(fetch).not.toHaveBeenCalled()
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    await waitFor(() => expect(fetch.mock.calls.some(([, options]) => options?.method === 'POST')).toBe(true))
  })

  it.runIf(viewport === 'desktop')('resolves the destination against the refreshed roster during a drag', async () => {
    const game = roster('dm')
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const handle = grip('Ada')
    movePointer(handle, screen.getByText('Orc'))
    const updated = structuredClone(game)
    updated.entries = [updated.entries[1]!, updated.entries[0]!, updated.entries[2]!]
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response(JSON.stringify(updated), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    await act(async () => { window.dispatchEvent(new Event('focus')) })
    await waitFor(() => expect(screen.getAllByRole('article')[0]).toHaveAttribute('aria-label', 'Goblin'))
    document.elementFromPoint = () => screen.getByText('Goblin')
    fireEvent.pointerUp(handle, { clientX: 100, clientY: 100, pointerId: 1 })
    await waitFor(() => expect(fetch.mock.calls.some(([, options]) => options?.method === 'POST')).toBe(true))
    expect(JSON.parse(fetch.mock.calls.find(([, options]) => options?.method === 'POST')![1]!.body as string))
      .toEqual({ entry_id: 'pc_chr_1', before_id: 'mon_a' })
  })

  it('does not offer drag ordering to players', async () => {
    const game = roster('player')
    game.entries[0]!.can_edit = true
    renderGame(viewport, game)
    await screen.findByText('Ada')
    expect(screen.queryByRole('button', { name: /^Drag to reorder / })).not.toBeInTheDocument()
    await setupUser().click(screen.getByRole('button', { name: 'Actions for Ada' }))
    expect(screen.getByRole('menuitem', { name: 'Edit' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: /^Move / })).not.toBeInTheDocument()
  })
})

describe.each(['mobile', 'desktop'] as const)('tracker refinements (%s)', (viewport) => {
  it('shows class artwork for copied NPCs and a stable random icon for stubs', async () => {
    const game = gameAs('player')
    game.entries = [
      { id: 'copied', kind: 'monster', name: 'Copied rogue', class: 'dnd-2014/rogue', can_edit: false },
      { id: 'stub', kind: 'monster', name: 'NPC', can_edit: false },
      { id: 'uploaded', kind: 'monster', name: 'Portrait NPC', image: '/custom.webp', class: 'wizard', can_edit: false },
    ]
    renderGame(viewport, game)
    const copied = await screen.findByRole('article', { name: 'Copied rogue' })
    expect(within(copied).getByAltText('')).toHaveAttribute('src', '/avatars/rogue.webp')
    expect(within(screen.getByRole('article', { name: 'NPC' })).getByAltText('').getAttribute('src')).toMatch(/^\/avatars\/random\/\w+\.webp$/)
    expect(within(screen.getByRole('article', { name: 'Portrait NPC' })).getByAltText('')).toHaveAttribute('src', '/custom.webp')
  })

  it('puts character and NPC add buttons in the toolbar above the roster', async () => {
    renderGame(viewport, gameAs('dm'))
    const button = await screen.findByRole('button', { name: 'Add prepared NPC' })
    const toolbar = button.parentElement!
    expect(toolbar).toContainElement(screen.getByRole('button', { name: 'Add NPC stub' }))
    const row = screen.getByRole('article', { name: 'Ada' })
    expect(button.compareDocumentPosition(row) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    const addCharacter = screen.getByRole('button', { name: 'Add from group' })
    expect(addCharacter.compareDocumentPosition(button) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(addCharacter.compareDocumentPosition(row) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(toolbar).toContainElement(addCharacter)
    // Six actions, one style: no button is drawn quieter than the others.
    const buttons = within(toolbar).getAllByRole('button')
    expect(buttons.map((each) => each.textContent)).toEqual(['Add from group', 'Add NPC stub', 'Add prepared NPC', 'Long rest', 'Short rest', 'Order'])
    expect(new Set(buttons.map((each) => each.getAttribute('data-variant')))).toEqual(new Set(['light']))
    expect(screen.queryByRole('button', { name: 'Add my characters' })).not.toBeInTheDocument()
  })

  it('edits only walking and spell DC in the monster movement section, preserving other copied modes and senses', async () => {
    const game = gameAs('dm')
    const source = structuredClone(game.entries[0]!)
    game.entries = [{ ...source, id: 'monster', kind: 'monster', name: 'Goblin' }]
    const monster = game.entries[0]!
    delete monster.character_id
    monster.stats!.speeds = [{ kind: 'walking', distance: 30 }, { kind: 'flying', distance: 60 }]
    monster.stats!.senses = [{ kind: 'darkvision', distance: 60 }]
    renderGame(viewport, game)
    await screen.findByText('Goblin')
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response(JSON.stringify(game), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    await pressRowAction(viewport, 'Goblin', 'Edit')
    const dialog = screen.getByRole('dialog')
    expect(dialog.querySelector('input')).toBe(screen.getByLabelText('Name'))
    expect(within(dialog).queryByRole('textbox', { name: 'Tags' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Walking')).toBeInTheDocument()
    expect(screen.getByLabelText('Spell save DC')).toBeInTheDocument()
    for (const label of ['Flying', 'Climbing', 'Swimming', 'Burrowing', 'Darkvision', 'Blindsight', 'Tremorsense', 'Truesight']) {
      expect(screen.queryByLabelText(label)).not.toBeInTheDocument()
    }
    const user = setupUser()
    await user.clear(screen.getByLabelText('Walking'))
    await user.type(screen.getByLabelText('Walking'), '40')
    await user.type(screen.getByLabelText('Spell save DC'), '13')
    await user.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() => expect(fetch.mock.calls.some(([, options]) => options?.method === 'PATCH')).toBe(true))
    const body = JSON.parse(fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')![1]!.body as string)
    expect(body.stats.speeds).toEqual([{ kind: 'flying', distance: 60 }, { kind: 'walking', distance: 40 }])
    expect(body.stats.spellcasting).toEqual([{ class: '', ability: '', attackBonus: 0, saveDC: 13 }])
    expect(body.stats).not.toHaveProperty('senses')
  })
})


describe.each(['mobile', 'desktop'] as const)('inline game tags (%s)', (viewport) => {
  it('adds and removes tags in the row without opening a dialog or changing other values', async () => {
    const game = gameAs('player')
    game.entries[0]!.can_edit = true
    renderGame(viewport, game)
    await screen.findByText('Ada')
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
      if (options?.method === 'PATCH') Object.assign(game.entries[0]!, JSON.parse(options.body as string))
      return new Response(JSON.stringify(game), { status: 200 })
    })
    vi.stubGlobal('fetch', fetch)
    const row = within(screen.getByRole('article', { name: 'Ada' }))
    const user = setupUser()
    await user.click(row.getByRole('button', { name: 'Add tag' }))
    await user.type(row.getByRole('textbox', { name: 'Tags' }), ' Concentrating {Enter}')
    const remove = await row.findByRole('button', { name: 'Remove tag Concentrating' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(row.getByText('HP').nextElementSibling).toHaveTextContent(shown(viewport, 20))
    await user.click(remove)
    await waitFor(() => expect(row.queryByRole('button', { name: 'Remove tag Concentrating' })).not.toBeInTheDocument())
    expect(row.queryByText('No tags')).not.toBeInTheDocument()
    expect(row.getByRole('button', { name: 'Add tag' })).toBeInTheDocument()
    expect(fetch.mock.calls.filter(([, options]) => options?.method === 'PATCH').map(([, options]) => JSON.parse(options!.body as string)))
      .toEqual([{ tags: ['Concentrating'] }, { tags: [] }])
  })

  it('shows read-only tags without mutation controls on locked or other players entries', async () => {
    const game = gameAs('player')
    game.entries[0]!.tags = ['Poisoned']
    game.entries[0]!.locked = true
    renderGame(viewport, game)
    const row = within(await screen.findByRole('article', { name: 'Ada' }))
    expect(row.getByText('Poisoned')).toBeInTheDocument()
    expect(row.queryByRole('button', { name: 'Add tag' })).not.toBeInTheDocument()
    expect(row.queryByRole('button', { name: 'Remove tag Poisoned' })).not.toBeInTheDocument()
  })

  it('retains an inline draft on failure and disables it if a refresh locks the entry', async () => {
    const game = gameAs('player')
    game.entries[0]!.can_edit = true
    renderGame(viewport, game)
    const row = within(await screen.findByRole('article', { name: 'Ada' }))
    const user = setupUser()
    await user.click(row.getByRole('button', { name: 'Add tag' }))
    await user.type(row.getByRole('textbox', { name: 'Tags' }), 'Blessed')
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => options?.method === 'PATCH'
      ? new Response(JSON.stringify({ error: { code: 'access_denied' } }), { status: 403 })
      : new Response(JSON.stringify(game), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    await user.click(row.getByRole('button', { name: 'Add tag' }))
    await row.findByRole('alert')
    expect(row.getByRole('textbox', { name: 'Tags' })).toHaveValue('Blessed')
    expect(row.queryByRole('button', { name: 'Remove tag Blessed' })).not.toBeInTheDocument()
    game.entries[0]!.can_edit = false
    await act(async () => { window.dispatchEvent(new Event('focus')) })
    await waitFor(() => expect(row.getByRole('textbox', { name: 'Tags' })).toBeDisabled())
    expect(row.getByRole('button', { name: 'Add tag' })).toBeDisabled()
    expect(fetch.mock.calls.filter(([, options]) => options?.method === 'PATCH')).toHaveLength(1)
  })
})


describe.each(['mobile', 'desktop'] as const)('game damage editor (%s)', (viewport) => {
  it.each([
    { damage: 2, hp: 20, temp: 1, patch: { temp_hp: 1 } },
    { damage: 3, hp: 20, temp: 0, patch: { temp_hp: 0 } },
    { damage: 7, hp: 16, temp: 0, patch: { hp: 16, temp_hp: 0 } },
    { damage: 50, hp: 0, temp: 0, patch: { hp: 0, temp_hp: 0 } },
  ])('previews $damage damage and commits it only when Apply is pressed', async ({ damage, hp, temp, patch }) => {
    const game = gameAs('dm')
    game.entries[0]!.temp_hp = 3
    renderGame(viewport, game)
    await screen.findByText('Ada')
    await pressRowAction(viewport, 'Ada', 'Edit')
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
      if (options?.method === 'PATCH') Object.assign(game.entries[0]!, JSON.parse(options.body as string))
      return new Response(JSON.stringify(game), { status: 200 })
    })
    vi.stubGlobal('fetch', fetch)
    const user = setupUser()
    await user.type(screen.getByLabelText('Damage'), String(damage))
    expect(screen.getByLabelText('Hit points')).toHaveValue(String(hp))
    expect(screen.getByLabelText('Temp HP')).toHaveValue(String(temp))
    expect(fetch).not.toHaveBeenCalled()
    expect(within(screen.getByRole('dialog')).queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(JSON.parse(fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')![1]!.body as string)).toEqual(patch)
    await waitFor(() => expect(within(screen.getByRole('article', { name: 'Ada' })).getByText('HP').nextElementSibling).toHaveTextContent(shown(viewport, hp)))
  })

  it('recalculates revised damage from the draft instead of subtracting on every keystroke', async () => {
    const game = gameAs('dm')
    game.entries[0]!.temp_hp = 3
    renderGame(viewport, game)
    await screen.findByText('Ada')
    await pressRowAction(viewport, 'Ada', 'Edit')
    const user = setupUser()
    await user.type(screen.getByLabelText('Damage'), '7')
    expect(screen.getByLabelText('Hit points')).toHaveValue('16')
    await user.clear(screen.getByLabelText('Damage'))
    await user.type(screen.getByLabelText('Damage'), '2')
    expect(screen.getByLabelText('Hit points')).toHaveValue('20')
    expect(screen.getByLabelText('Temp HP')).toHaveValue('1')
    await user.clear(screen.getByLabelText('Damage'))
    expect(screen.getByLabelText('Hit points')).toHaveValue('20')
    expect(screen.getByLabelText('Temp HP')).toHaveValue('3')
  })

  it('discards the damage preview when the close cross is pressed', async () => {
    const game = gameAs('dm')
    game.entries[0]!.temp_hp = 3
    renderGame(viewport, game)
    await screen.findByText('Ada')
    await pressRowAction(viewport, 'Ada', 'Edit')
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response(JSON.stringify(game), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    const user = setupUser()
    await user.type(screen.getByLabelText('Damage'), '7')
    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(fetch.mock.calls.some(([, options]) => options?.method === 'PATCH')).toBe(false)
    await pressRowAction(viewport, 'Ada', 'Edit')
    expect(screen.getByLabelText('Hit points')).toHaveValue('20')
    expect(screen.getByLabelText('Temp HP')).toHaveValue('3')
    expect(screen.getByLabelText('Damage')).toHaveValue('')
  })

  it('accepts a direct HP correction without applying the previous damage again', async () => {
    const game = gameAs('dm')
    game.entries[0]!.temp_hp = 3
    renderGame(viewport, game)
    await screen.findByText('Ada')
    await pressRowAction(viewport, 'Ada', 'Edit')
    const user = setupUser()
    await user.type(screen.getByLabelText('Damage'), '7')
    await user.clear(screen.getByLabelText('Hit points'))
    await user.type(screen.getByLabelText('Hit points'), '12')
    expect(screen.getByLabelText('Damage')).toHaveValue('')
    expect(screen.getByLabelText('Temp HP')).toHaveValue('0')
    await user.type(screen.getByLabelText('Damage'), '2')
    expect(screen.getByLabelText('Hit points')).toHaveValue('10')
  })

  it('retains the damage draft after a failed Apply and retries without applying it twice', async () => {
    const game = gameAs('dm')
    game.entries[0]!.temp_hp = 3
    renderGame(viewport, game)
    await screen.findByText('Ada')
    await pressRowAction(viewport, 'Ada', 'Edit')
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response(JSON.stringify({ error: { code: 'access_denied' } }), { status: 403 }))
    vi.stubGlobal('fetch', fetch)
    const user = setupUser()
    await user.type(screen.getByLabelText('Damage'), '7')
    await user.click(screen.getByRole('button', { name: 'Apply' }))
    await within(screen.getByRole('dialog')).findByRole('alert')
    expect(screen.getByLabelText('Hit points')).toHaveValue('16')
    expect(screen.getByLabelText('Temp HP')).toHaveValue('0')
    fetch.mockImplementation(async () => new Response(JSON.stringify(game), { status: 200 }))
    await user.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(fetch.mock.calls.filter(([, options]) => options?.method === 'PATCH').map(([, options]) => JSON.parse(options!.body as string)))
      .toEqual([{ hp: 16, temp_hp: 0 }, { hp: 16, temp_hp: 0 }])
  })
})
