import { afterEach, describe, expect, it, vi } from 'vitest'

import { jsonResponse } from '@/test/api'

import { addGameMonster, orderGameEntries, type GameDetail } from './games'
import { ApiError } from './errors'

const sample = (): GameDetail => ({
  id: 'game', group_id: 'group', name: 'Fight', created_at: '', role: 'dm', characters: [],
  entries: ['a', 'b', 'c'].map((id) => ({ id, kind: 'monster', name: id, can_edit: true })),
})
afterEach(() => vi.unstubAllGlobals())

// The client and the API ship as one release, so each of these is one request:
// the server orders by `before_id` and creates an NPC stub with its defaults.
describe('game ordering and NPC creation', () => {
  it('moves an entry in one request', async () => {
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => jsonResponse(sample()))
    vi.stubGlobal('fetch', fetch)
    await orderGameEntries('game', { entry_id: 'c', before_id: 'a' })
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(JSON.parse(fetch.mock.calls[0]![1]?.body as string)).toEqual({ entry_id: 'c', before_id: 'a' })
  })

  it('does not retry a refused move', async () => {
    const fetch = vi.fn(async () => jsonResponse({ error: { code: 'validation_error' } }, 400))
    vi.stubGlobal('fetch', fetch)
    await expect(orderGameEntries('game', { entry_id: 'c', before_id: 'a' })).rejects.toBeInstanceOf(ApiError)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('adds a stub or a copied character in one request', async () => {
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => jsonResponse(sample()))
    vi.stubGlobal('fetch', fetch)
    await addGameMonster('game')
    await addGameMonster('game', 'character')
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(fetch.mock.calls.map(([, options]) => JSON.parse(options?.body as string))).toEqual([{ character_id: '' }, { character_id: 'character' }])
  })
})
