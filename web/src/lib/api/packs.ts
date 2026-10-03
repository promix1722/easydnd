import type { Sheet } from './characters'
import { request } from './client'

export interface PackRelease {
  id: string
  version: string
  digest: string
}
export interface RulesLock {
  edition: string
  semantics: string
  packs: PackRelease[]
}
export type PackValue =
  string | number | boolean | null | PackValue[] | { [key: string]: PackValue }
export interface PackDocument {
  manifest: { id: string; version: string; [key: string]: PackValue }
  entities: Record<string, PackValue>
  mechanics?: Record<string, PackValue>
  locales: Record<string, PackValue>
}
export interface PackRecord {
  id: string
  title: string
  owned: boolean
  builtin: boolean
  archived: boolean
  revision: number
  draft?: PackDocument
  releases: PackRelease[]
}
export interface PackField {
  integer?: boolean
  type: string
  ref?: string
  properties?: Record<string, PackField>
  items?: PackField
  optional?: boolean
}
export interface PackSchema {
  root: PackField
  definitions: Record<string, PackField>
}
export interface PackValidation {
  valid: boolean
  rules?: RulesLock
  diagnostics: { reason: string; detail: string; path?: string; args?: Record<string, unknown> }[]
}
export interface PackShare {
  pack: string
  contributor: string
  rules: RulesLock
}
export const listPacks = () => request<{ packs: PackRecord[]; defaultRules: RulesLock }>('/packs')
export const getPack = (id: string) => request<PackRecord>(`/packs/${encodeURIComponent(id)}`)
export const getPackSchema = () => request<PackSchema>('/packs/schema')
export const createPack = (title: string, document?: PackDocument) =>
  request<PackRecord>('/packs', { method: 'POST', body: { title, document } })
export const importPack = async (file: File) =>
  request<PackRecord>(
    `/packs/import?title=${encodeURIComponent(file.name.replace(/\.json$/i, ''))}`,
    { method: 'POST', rawBody: await file.text() },
  )
export const savePack = (
  pack: PackRecord,
  document: PackDocument,
  mappings: Record<string, string> = {},
) =>
  request<PackRecord>(`/packs/${pack.id}/draft`, {
    method: 'PUT',
    body: { title: pack.title, document, expectedRevision: pack.revision, mappings },
  })
export const validatePack = (id: string) =>
  request<PackValidation>(`/packs/${id}/validate`, { method: 'POST', body: {} })
export const publishPack = (pack: PackRecord) =>
  request<PackRecord>(`/packs/${pack.id}/publish`, {
    method: 'POST',
    body: { expectedRevision: pack.revision },
  })
export const archivePack = (pack: PackRecord) =>
  request<PackRecord>(`/packs/${pack.id}/archive`, {
    method: 'POST',
    body: { expectedRevision: pack.revision },
  })
export const exportPack = (id: string, version = '') =>
  request<PackDocument>(`/packs/${id}/export?version=${encodeURIComponent(version)}`)
export const resolvePacks = (packs: PackRelease[]) =>
  request<RulesLock>('/packs/resolve', { method: 'POST', body: { packs } })
export const getGroupPacks = (id: string) => request<PackShare[]>(`/groups/${id}/packs`)
export const sharePack = (group: string, pack: string, version: string) =>
  request(`/groups/${group}/packs`, { method: 'POST', body: { pack, version } })
export const unsharePack = (group: string, pack: string) =>
  request(`/groups/${group}/packs?pack=${encodeURIComponent(pack)}`, { method: 'DELETE' })
export interface PackMigration {
  revision: number
  before: Sheet
  after: Sheet
  changed: string[]
  issues?: { eventId: string; reason: string }[]
}
export const migratePackSelection = (
  id: string,
  expectedRevision: number,
  rules: RulesLock,
  dryRun: boolean,
) =>
  request<PackMigration>(`/characters/${id}/rules?dryRun=${String(dryRun)}`, {
    method: 'POST',
    body: { expectedRevision, rules },
  })
