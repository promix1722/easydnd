import { afterEach, describe, expect, it, vi } from 'vitest'

import { addGameMonster, orderGameEntries, type GameDetail } from './games'
import { ApiError } from './errors'

const sample = (): GameDetail => ({
  id: 'game', group_id: 'group', name: 'Fight', created_at: '', role: 'dm', characters: [],
  entries: ['a', 'b', 'c'].map((id) => ({ id, kind: 'monster', name: id, can_edit: true })),
})
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status })
afterEach(() => vi.unstubAllGlobals())

function oldAPI() {
  const game = sample()
  const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
    if (options?.method !== 'POST') return json(game)
    const body = JSON.parse(options.body as string)
    if (body.direction !== -1 && body.direction !== 1) return json({ error: { code: 'validation_error' } }, 400)
    const from = game.entries.findIndex((entry) => entry.id === body.entry_id)
    const [entry] = game.entries.splice(from, 1)
    game.entries.splice(from + body.direction, 0, entry!)
    return json(game)
  })
  vi.stubGlobal('fetch', fetch)
  return { game, fetch }
}

describe('game drag ordering', () => {
  it('uses one atomic request on an updated server', async () => {
    const fetch = vi.fn(async () => json(sample()))
    vi.stubGlobal('fetch', fetch)
    await orderGameEntries('game', { entry_id: 'c', before_id: 'a' })
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('moves down to the end on the running older API without losing other entries', async () => {
    const { fetch } = oldAPI()
    const game = await orderGameEntries('game', { entry_id: 'a', before_id: '' })
    expect(game.entries.map((entry) => entry.id)).toEqual(['b', 'c', 'a'])
    const moves = fetch.mock.calls.filter(([, options]) => options?.method === 'POST')
      .map(([, options]) => JSON.parse(options!.body as string))
    expect(moves).toEqual([
      { entry_id: 'a', before_id: '' },
      { entry_id: 'a', direction: 1 },
      { entry_id: 'a', direction: 1 },
    ])
  })

  it('moves up to a stable target on the older API', async () => {
    oldAPI()
    const game = await orderGameEntries('game', { entry_id: 'c', before_id: 'a' })
    expect(game.entries.map((entry) => entry.id)).toEqual(['c', 'a', 'b'])
  })

  it('does not move when the entry is already before the target', async () => {
    const { fetch } = oldAPI()
    const game = await orderGameEntries('game', { entry_id: 'a', before_id: 'b' })
    expect(game.entries.map((entry) => entry.id)).toEqual(['a', 'b', 'c'])
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('refuses missing targets without issuing adjacent moves', async () => {
    const { fetch } = oldAPI()
    await expect(orderGameEntries('game', { entry_id: 'a', before_id: 'deleted' })).rejects.toMatchObject({ status: 404 })
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('does not retry permission failures or rejected initiative sorting', async () => {
    const fetch = vi.fn(async () => json({ error: { code: 'access_denied' } }, 403))
    vi.stubGlobal('fetch', fetch)
    await expect(orderGameEntries('game', { entry_id: 'c', before_id: 'a' })).rejects.toBeInstanceOf(ApiError)
    expect(fetch).toHaveBeenCalledTimes(1)
    fetch.mockClear()
    fetch.mockImplementation(async () => json({ error: { code: 'validation_error' } }, 400))
    await expect(orderGameEntries('game', { by_initiative: true })).rejects.toMatchObject({ status: 400 })
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('bounds retries when concurrent ordering prevents reaching the destination', async () => {
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
      if (options?.method === 'POST' && JSON.parse(options.body as string).before_id !== undefined) {
        return json({ error: { code: 'validation_error' } }, 400)
      }
      return json(sample())
    })
    vi.stubGlobal('fetch', fetch)
    await expect(orderGameEntries('game', { entry_id: 'a', before_id: '' })).rejects.toMatchObject({ status: 409, reason: 'game.orderChanged' })
    expect(fetch).toHaveBeenCalledTimes(8)
  })
})


describe('monster creation defaults', () => {
  it.each([1, 10])('initializes only the new legacy stub with %i HP to NPC and 10/10', async (hp) => {
    const before = sample()
    const stub = { ...before.entries[0]!, id: 'new', name: 'Monster', hp, stats: { name: 'Monster', max_hp: hp } }
    const created = { ...before, entries: [...before.entries, stub] }
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => {
      if (options?.method === 'POST') return json(created)
      if (options?.method === 'PATCH') return json({ ...created, entries: [...before.entries, { ...stub, name: 'NPC', hp: 10, stats: { name: 'NPC', max_hp: 10 } }] })
      return json(before)
    })
    vi.stubGlobal('fetch', fetch)
    const result = await addGameMonster('game')
    expect(fetch.mock.calls.map(([, options]) => options?.method ?? 'GET')).toEqual(['GET', 'POST', 'PATCH'])
    expect(String(fetch.mock.calls[2]![0])).toContain('/entries/new')
    expect(JSON.parse(fetch.mock.calls[2]![1]!.body as string)).toEqual(hp === 1
      ? { hp: 10, stats: { max_hp: 10, name: 'NPC' } } : { stats: { name: 'NPC' } })
    expect(result.entries.at(-1)).toMatchObject({ name: 'NPC', hp: 10, stats: { name: 'NPC', max_hp: 10 } })
  })

  it('does not patch stubs that already use 10/10 or ambiguous concurrent additions', async () => {
    const before = sample()
    const fetch = vi.fn(async (_url: unknown, options?: RequestInit) => json(options?.method === 'POST'
      ? { ...before, entries: [...before.entries, { ...before.entries[0], id: 'new', hp: 10, stats: { max_hp: 10 } }] }
      : before))
    vi.stubGlobal('fetch', fetch)
    await addGameMonster('game')
    expect(fetch).toHaveBeenCalledTimes(2)
    fetch.mockClear()
    fetch.mockImplementation(async (_url: unknown, options?: RequestInit) => json(options?.method === 'POST'
      ? { ...before, entries: [...before.entries, ...['new', 'other'].map((id) => ({ ...before.entries[0], id, hp: 1, stats: { max_hp: 1 } }))] }
      : before))
    await addGameMonster('game')
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('keeps copied character HP unchanged without a compatibility patch', async () => {
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => json(sample()))
    vi.stubGlobal('fetch', fetch)
    await addGameMonster('game', 'character')
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(JSON.parse(fetch.mock.calls[0]![1]?.body as string)).toEqual({ character_id: 'character' })
  })
})
