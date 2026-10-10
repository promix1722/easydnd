import { track } from '@/lib/analytics'
import type { ClassLevel, CustomItem, Sheet } from './characters'
import { request } from './client'
import { ApiError } from './errors'
import { getGroup } from './groups'
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
  /** Its owner opened the sheet; a player reads it only then. */
  public?: boolean
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

/**
 * Who a shared character belongs to: their id and the name the group knows
 * them by. Null when either lookup fails or the character is not on the table
 * -- a trail is worth drawing without its player.
 *
 * Two requests the group's screens already make, because a sheet does not say
 * whose it is: the table lists each character's owner, and the group's roster
 * names them.
 */
export async function getSharedOwner(group: string, character: string, signal?: AbortSignal): Promise<{ id: string; name: string } | null> {
  try {
    const [table, detail] = await Promise.all([listTable(group, signal), getGroup(group, signal)])
    const owner = table.characters.find((each) => each.id === character)?.owner_id
    const member = detail.members.find((each) => each.user_id === owner)
    return owner === undefined ? null : { id: owner, name: member?.display_name ?? '' }
  } catch {
    return null
  }
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

export async function createGame(group: string, name: string): Promise<GameDetail> {
  const result = await request<GameDetail>('/games', {
    method: 'POST',
    body: { group_id: group, name },
  })
  track('game_created')
  return result
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

/** A rest for the whole table: a long one returns every spent use, a short one only what a short rest refills. */
export function restGame(id: string, kind: 'short' | 'long'): Promise<GameDetail> {
  return request<GameDetail>(`/games/${encodeURIComponent(id)}/rest?kind=${kind}`, { method: 'POST' })
}

const entryURL = (id: string, entry: string) => `/games/${encodeURIComponent(id)}/entries/${encodeURIComponent(entry)}`

/** The DM gives a seated character an item: the one write of somebody else's character a table makes. */
export function grantItem(id: string, entry: string, item: string, count = 1): Promise<GameDetail> {
  return request<GameDetail>(`${entryURL(id, entry)}/items`, { method: 'POST', body: { item, count } })
}

/** The DM gives a seated character a new custom item: `grantItem` for something no catalogue holds. */
export function grantCustomItem(id: string, entry: string, item: { name: string; description: string; item: CustomItem }): Promise<void> {
  return request<void>(`${entryURL(id, entry)}/custom-items`, { method: 'POST', body: item })
}

/** The DM adds to a seated character's purse, or with a negative amount takes from it. */
export function adjustCoins(id: string, entry: string, unit: string, amount: number): Promise<GameDetail> {
  return request<GameDetail>(`${entryURL(id, entry)}/coins`, { method: 'POST', body: { unit, amount } })
}

/** A player passes an item of their own character's to another one seated at the game. */
export function giveItem(id: string, entry: string, to: string, item: string, count = 1): Promise<GameDetail> {
  return request<GameDetail>(`${entryURL(id, entry)}/give`, { method: 'POST', body: { item, count, to } })
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
    // A frontend update may run against an older API. Those reject
    // before_id-only moves before changing any entries.
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
