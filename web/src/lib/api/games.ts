import type { ClassLevel, Sheet } from './characters'
import { request } from './client'
import { ApiError } from './errors'
import type { GroupRole } from './groups'

/**
 * Games, and the characters a group has put on its table.
 *
 * Shapes mirror internal/api/http/v1/game, field names included -- the
 * envelope is snake_case there, so it is snake_case here rather than being
 * rewritten in transit.
 *
 * A **game** is one sitting run by a DM. It is never called a session: that
 * word is spoken for several times over by signing in -- `SessionUser`,
 * `getSession`, `startGuestSession` all live one file away in this same flat
 * barrel -- and a second meaning for it here would be unreadable at the import
 * site.
 *
 * Game entries carry independent tracking values; TableCharacter stays the
 * lightweight summary used by the group's shared pool.
 */

/** One character on a group's table, or seated at a game. */
export interface TableCharacter {
  image?: string
  id: string
  owner_id: string
  name: string
  level: number
  classes?: ClassLevel[]
}

/**
 * One row of the games list. It carries no roster: the list draws names.
 *
 * `group_name` is on the row because the list spans every table you sit at, so
 * "Thursday night" alone would not say which one it belongs to.
 */
export interface GameSummary {
  id: string
  group_id: string
  group_name: string
  name: string
  created_at: string
}

export interface GameDetail {
  id: string
  group_id: string
  name: string
  created_at: string
  /** The caller's rank in the group, which is what decides the controls. */
  role: GroupRole
  characters: TableCharacter[]
  entries: GameEntry[]
}

// The group's table.

export function listTable(
  group: string,
  signal?: AbortSignal,
): Promise<{ characters: TableCharacter[] }> {
  return request<{ characters: TableCharacter[] }>(
    `/groups/${encodeURIComponent(group)}/characters`,
    signal ? { signal } : {},
  )
}

export function shareCharacter(
  group: string,
  character: string,
): Promise<{ characters: TableCharacter[] }> {
  return request<{ characters: TableCharacter[] }>(
    `/groups/${encodeURIComponent(group)}/characters`,
    { method: 'POST', body: { character_id: character } },
  )
}

export function unshareCharacter(
  group: string,
  character: string,
): Promise<{ characters: TableCharacter[] }> {
  return request<{ characters: TableCharacter[] }>(
    `/groups/${encodeURIComponent(group)}/characters?character=${encodeURIComponent(character)}`,
    { method: 'DELETE' },
  )
}

/**
 * One shared character's sheet, read by somebody who does not own it.
 *
 * It resolves the same `Sheet` as `getSheet`, because the server renders both
 * with one converter. If this ever needs a type of its own the server has
 * drifted, and the right place to find out is a compile error here.
 */
export function getSharedSheet(character: string, signal?: AbortSignal): Promise<Sheet> {
  return request<Sheet>(
    `/shared/${encodeURIComponent(character)}/sheet`,
    signal ? { signal } : {},
  )
}

// Games.

/** Every game at every table you sit at, newest first. */
export function listGames(signal?: AbortSignal): Promise<{ games: GameSummary[] }> {
  return request<{ games: GameSummary[] }>('/games', signal ? { signal } : {})
}

export function createGame(group: string, name: string): Promise<GameDetail> {
  return request<GameDetail>('/games', {
    method: 'POST',
    body: { group_id: group, name },
  })
}

export function getGame(id: string, signal?: AbortSignal): Promise<GameDetail> {
  return request<GameDetail>(`/games/${encodeURIComponent(id)}`, signal ? { signal } : {})
}

export function renameGame(id: string, name: string): Promise<GameDetail> {
  return request<GameDetail>(`/games/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: { name },
  })
}

export function deleteGame(id: string): Promise<void> {
  return request<void>(`/games/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

/**
 * Seats characters at a game.
 *
 * An empty list means everyone on the group's table. That is the "add all"
 * affordance, and it is the same request shape as adding one -- the server
 * resolves the list, because a client enumerating the table and posting it
 * back would race whoever is sharing at that moment.
 */
export function addToGame(id: string, characters: string[]): Promise<GameDetail> {
  return request<GameDetail>(`/games/${encodeURIComponent(id)}/characters`, {
    method: 'POST',
    body: { character_ids: characters },
  })
}

export function removeFromGame(id: string, character: string): Promise<GameDetail> {
  return request<GameDetail>(
    `/games/${encodeURIComponent(id)}/characters?character=${encodeURIComponent(character)}`,
    { method: 'DELETE' },
  )
}

/** Base stats follow the player sheet, or belong to a private monster copy. */
export interface EntryStats {
  name: string
  max_hp: number
  armor_class: number
  spellcasting: Sheet['status']['spellcasting']
  speeds: Sheet['base']['speeds']
  senses: Sheet['base']['senses']
  abilities: Sheet['abilities']
}

export interface GameEntry {
  image?: string
  class?: string
  id: string
  kind: 'player' | 'monster'
  name: string
  character_id?: string
  can_edit: boolean
  locked?: boolean
  hp?: number
  temp_hp?: number
  initiative?: number | null
  tags?: string[]
  stats?: EntryStats
  /** A player's consumables; `used` is this game's count, never the sheet's. */
  resources?: EntryPool[]
}

export interface EntryPool {
  id: string
  name?: string
  group?: string
  max: number
  used: number
  dice?: string
  slot_level?: number
}

export interface EntryPatch {
  hp?: number
  temp_hp?: number
  initiative?: number | null
  tags?: string[]
  locked?: boolean
  stats?: Partial<EntryStats>
  /** Spent count per pool id; pools left out keep theirs. */
  used?: Record<string, number>
}

export function patchGameEntry(id: string, entry: string, patch: EntryPatch): Promise<GameDetail> {
  return request<GameDetail>(`/games/${encodeURIComponent(id)}/entries/${encodeURIComponent(entry)}`, { method: 'PATCH', body: patch })
}

/** The table's only recovery: everybody gets every spent use back. */
export function restGame(id: string): Promise<GameDetail> {
  return request<GameDetail>(`/games/${encodeURIComponent(id)}/rest`, { method: 'POST' })
}

export function deleteGameEntry(id: string, entry: string): Promise<GameDetail> {
  return request<GameDetail>(`/games/${encodeURIComponent(id)}/entries/${encodeURIComponent(entry)}`, { method: 'DELETE' })
}

export async function addGameMonster(id: string, character?: string): Promise<GameDetail> {
  // Record the roster before creating a stub so an older running API can be
  // upgraded to the new default without touching pre-existing monsters.
  const before = character ? null : await getGame(id)
  const game = await request<GameDetail>(`/games/${encodeURIComponent(id)}/monsters`, { method: 'POST', body: { character_id: character ?? '' } })
  if (before) {
    const known = new Set(before.entries.map((entry) => entry.id))
    const added = game.entries.filter((entry) => !known.has(entry.id))
    const stub = added.length === 1 ? added[0] : undefined
    if (stub?.kind === 'monster' && stub.can_edit) {
      const patch: EntryPatch = {}
      if (stub.hp === 1 && stub.stats?.max_hp === 1) {
        patch.hp = 10
        patch.stats = { max_hp: 10 }
      }
      if (stub.name === 'Monster' && stub.stats?.name === 'Monster') {
        patch.stats = { ...patch.stats, name: 'NPC' }
      }
      if (Object.keys(patch).length > 0) return patchGameEntry(id, stub.id, patch)
    }
  }
  return game
}

export async function orderGameEntries(id: string, order: {entry_id?: string; direction?: number; by_initiative?: boolean; before_id?: string}): Promise<GameDetail> {
  const endpoint = `/games/${encodeURIComponent(id)}/order`
  try {
    return await request<GameDetail>(endpoint, { method: 'POST', body: order })
  } catch (cause) {
    // A frontend update must not force a restart of process-local games.
    // Older APIs reject before_id-only moves before changing any entries.
    if (order.before_id === undefined || !order.entry_id || !(cause instanceof ApiError)
      || cause.status !== 400 || cause.code !== 'validation_error') throw cause
  }

  // Each confirmed response is the next source of truth. Move only the chosen
  // entry, preserving other rows and checking stable IDs after each step.
  let game = await getGame(id)
  const limit = Math.max(4, game.entries.length * 2)
  for (let attempt = 0; attempt < limit; attempt++) {
    const from = game.entries.findIndex((entry) => entry.id === order.entry_id)
    const before = order.before_id === '' ? game.entries.length
      : game.entries.findIndex((entry) => entry.id === order.before_id)
    if (from < 0 || before < 0) throw new ApiError(404, { code: 'not_found' })
    const destination = before > from ? before - 1 : before
    if (from === destination || order.entry_id === order.before_id) return game
    game = await request<GameDetail>(endpoint, {
      method: 'POST', body: { entry_id: order.entry_id, direction: destination > from ? 1 : -1 },
    })
  }
  const from = game.entries.findIndex((entry) => entry.id === order.entry_id)
  const before = order.before_id === '' ? game.entries.length
    : game.entries.findIndex((entry) => entry.id === order.before_id)
  if (from >= 0 && before >= 0 && (order.entry_id === order.before_id || from === (before > from ? before - 1 : before))) return game
  throw new ApiError(409, { code: 'validation_error', reason: 'game.orderChanged' })
}
