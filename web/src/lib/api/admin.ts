import { request } from './client'

/** One account row of the superadmin's listing. */
export interface AdminPlayer {
  id: string
  display_name: string
  email?: string
  created_at: string
  /** The latest sign-in; absent for a row nobody ever signed in to. */
  last_used_at?: string
  passkeys: number
  anonymous: boolean
  /** The ids of the private packs this account has been handed; absent for none. */
  packs?: string[]
}

/** One character row of the superadmin's listing. */
export interface AdminCharacter {
  id: string
  name: string
  level: number
  classes: { class: string; level: number }[]
  owner: string
  /** Absent for a guest who never joined a group: they have no account row. */
  owner_name?: string
  public: boolean
  revision: number
}

/** A filter set as the URL carries it: absent and empty both mean "any". */
export type AdminFilters = Record<string, string | null | undefined>

function list<T>(path: string, filters: AdminFilters, offset: number, signal?: AbortSignal): Promise<T> {
  const query = new URLSearchParams({ offset: String(offset) })
  for (const [key, value] of Object.entries(filters)) {
    if (value) query.set(key, value)
  }
  return request<T>(`${path}?${query.toString()}`, { signal })
}

/** Every stored account. Filters: `q`, `kind` (`account` | `guest`), `sort` (`created` | `last_login`). 404 unless a superadmin asks. */
export function listAdminPlayers(filters: AdminFilters, offset: number, signal?: AbortSignal) {
  return list<{ players: AdminPlayer[]; total: number }>('/admin/players', filters, offset, signal)
}

/** Every character, whoever owns it, newest first. Filters: `owner`, `public` (`true` | `false`). */
export function listAdminCharacters(filters: AdminFilters, offset: number, signal?: AbortSignal) {
  return list<{ characters: AdminCharacter[]; total: number }>('/admin/characters', filters, offset, signal)
}

/**
 * Deletes an account with its characters and its seats in groups. 400 with a
 * reason while it owns a group others sit at or authored a homebrew pack.
 */
export function deleteAdminPlayer(id: string): Promise<void> {
  return request<void>(`/admin/players/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

/** Deletes anybody's character, with its log. */
export function deleteAdminCharacter(id: string): Promise<void> {
  return request<void>(`/admin/characters/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

/** A private disk pack installed on this server: what a superadmin can hand out. */
export interface AdminPack {
  id: string
  title: string
}

/** The private packs installed here. Empty on a server that has none. */
export function listAdminPacks(signal?: AbortSignal) {
  return request<{ packs: AdminPack[] }>('/admin/packs', { signal })
}

const playerPacks = (player: string) => `/admin/players/${encodeURIComponent(player)}/packs`

/** The ids of the private packs one account has been handed. */
export function getAdminPlayerPacks(player: string, signal?: AbortSignal) {
  return request<{ packs: string[] }>(playerPacks(player), { signal })
}

/** Replaces that list whole. */
export function setAdminPlayerPacks(player: string, packs: string[]): Promise<void> {
  return request<void>(playerPacks(player), { method: 'PUT', body: { packs } })
}
