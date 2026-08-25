import type { ClassLevel, Sheet } from './characters'
import { request } from './client'
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
 * `TableCharacter` and the roster entries on a `GameDetail` are the same shape
 * today and are deliberately one type: what a table holds and what a game
 * seats are the same fact seen twice. Split them the first time a seat carries
 * something a share does not.
 */

/** One character on a group's table, or seated at a game. */
export interface TableCharacter {
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
