import type { Choice } from './catalog'
import { request } from './client'

/**
 * Characters.
 *
 * Shapes mirror internal/api/http/v1/character. The one thing worth knowing
 * before reading them: every write returns the freshly projected sheet, which
 * is why nothing here caches and nothing needs invalidating.
 */

export interface ClassLevel {
  class: string
  subclass?: string
  level: number
}

export interface Summary {
  id: string
  name: string
  level: number
  classes?: ClassLevel[]
}

export interface Identity {
  name: string
  alignment?: string
  race?: string
  subrace?: string
  background?: string
  classes?: ClassLevel[]
  level: number
  personalityTraits?: string[]
  ideals?: string[]
  bonds?: string[]
  flaws?: string[]
}

export interface Base {
  hitPoints: { current: number; max: number; temporary?: number }
  speeds?: { kind: string; distance: number }[]
  senses?: { kind: string; distance: number }[]
  size?: string
  languages?: string[]
  exhaustion?: number
  deathSaves: { successes: number; failures: number }
  inspiration?: boolean
}

export interface Abilities {
  scores: Record<string, number>
  /** Served though never stored: a pure function of the score, computed once. */
  modifiers: Record<string, number>
  method?: string
}

export interface Skill {
  proficiency: 'none' | 'half' | 'proficient' | 'expertise'
  bonus: number
}

export interface SavingThrow {
  proficient: boolean
  bonus: number
}

export interface Spellcasting {
  class: string
  ability: string
  saveDC: number
  attackBonus: number
}

export interface Status {
  armorClass: number
  initiative: number
  proficiencyBonus: number
  passivePerception: number
  spellcasting?: Spellcasting[]
}

export interface ItemStack {
  item?: string
  count: number
  custom?: { name: string; description?: string; weight?: number }
}

export interface Equipment {
  equipped: ItemStack[]
  backpack: ItemStack[]
  loot: ItemStack[]
  purse?: Record<string, number>
}

export interface Pool {
  key?: string
  max: number
  used?: number
  recharge?: string
  dice?: string
}

export interface Resources {
  spellSlots?: Record<string, Pool>
  hitDice?: Pool[]
  class?: Pool[]
}

export interface Sheet {
  identity: Identity
  base: Base
  abilities: Abilities
  skills: Record<string, Skill>
  savingThrows: Record<string, SavingThrow>
  status: Status
  equipment: Equipment
  resources: Resources
  spells: { cantrips?: string[]; known?: string[]; prepared?: string[]; ability?: string }
  actions: unknown[]
  feats?: string[]
  traits?: string[]
  features?: string[]
  conditions?: string[]
  proficiencies?: string[]
}

export interface Answer {
  prompt: string
  picks: string[]
}

export interface Change {
  path: string
  op: 'set' | 'increment' | 'add' | 'remove'
  value: {
    kind: 'int' | 'string' | 'bool' | 'slug' | 'slugs' | 'dice' | 'none'
    int?: number
    string?: string
    bool?: boolean
    slug?: string
    slugs?: string[]
    dice?: string
  }
}

export interface CharacterEvent {
  seq?: number
  type: string
  at?: string
  ref?: string
  level?: number
  choices?: Answer[]
  changes?: Change[]
  note?: string
}

/** What the answer to a prompt must be posted as. */
export interface PromptEvent {
  type: string
  ref?: string
  level?: number
}

export interface Prompt {
  choice: Choice
  /** The catalogue entry posing this prompt, as "kind:slug". */
  source?: string
  group: string
  level?: number
  /** A character is complete without answering an optional prompt. */
  optional: boolean
  /** Answering this prompt grants a level. */
  advances: boolean
  event: PromptEvent
  /** Options the character already has from another source. */
  held?: string[]
  /**
   * Inverts what `held` means: those are the only legal answers rather than
   * the illegal ones. Expertise is the case -- it doubles a proficiency the
   * character already has.
   */
  heldOnly: boolean
}

export interface PromptsResponse {
  seq: number
  /** Nothing required is outstanding. Separate from the list being empty: a
   * finished character still has the optional prompt offering them a level. */
  complete: boolean
  prompts: Prompt[]
}

export interface WriteResponse {
  seq: number
  sheet: Sheet
}

export interface CreateResponse {
  id: string
  seq: number
  sheet: Sheet
}

export interface NewCharacter {
  name: string
  alignment?: string
  method?: string
  /** The *base* array, before racial bonuses; the server applies those. */
  abilities: Record<string, number>
}

/** One line of an import report: a field of the export, and what became of it. */
export interface ImportEntry {
  field: string
  detail: string
}

/**
 * Everything an import could not carry across.
 *
 * Not a failure list. SRD 5.1 publishes one background and one feat, so a
 * sheet from a tool with the full rules always leaves something behind; this
 * is what makes that visible instead of silent.
 */
export interface ImportReport {
  /** Named something SRD 5.1 does not publish. */
  unresolved: ImportEntry[]
  /** Real data the model has no home for. */
  skipped: ImportEntry[]
  /** Prompts the import left for the player to answer. */
  open: string[]
}

export interface ImportResponse {
  id: string
  seq: number
  sheet: Sheet
  report: ImportReport
}

export function listCharacters(signal?: AbortSignal): Promise<{ characters: Summary[] }> {
  return request<{ characters: Summary[] }>('/characters', signal ? { signal } : {})
}

export function createCharacter(body: NewCharacter): Promise<CreateResponse> {
  return request<CreateResponse>('/characters', { method: 'POST', body })
}

/**
 * Imports a character from a sheet exported by another tool.
 *
 * The file's bytes are the body: the route takes the export itself, not a
 * wrapper object, so rawBody sends it untouched rather than re-encoding JSON
 * that is already JSON.
 *
 * An imported character arrives with every choice unanswered, so callers
 * should send the player to the build screen rather than the sheet.
 */
export async function importCharacter(file: File): Promise<ImportResponse> {
  return request<ImportResponse>('/characters/import', {
    method: 'POST',
    rawBody: await file.text(),
  })
}

export function getSheet(id: string, signal?: AbortSignal): Promise<Sheet> {
  return request<Sheet>(`/characters/${id}/sheet`, signal ? { signal } : {})
}

export function getPrompts(id: string, signal?: AbortSignal): Promise<PromptsResponse> {
  return request<PromptsResponse>(`/characters/${id}/prompts`, signal ? { signal } : {})
}

export function getEvents(
  id: string,
  signal?: AbortSignal,
): Promise<{ seq: number; events: CharacterEvent[] }> {
  return request<{ seq: number; events: CharacterEvent[] }>(
    `/characters/${id}/events`,
    signal ? { signal } : {},
  )
}

/**
 * Appends events to a character's log.
 *
 * `expectedSeq` is not optional. The whole log is one record, so two clients
 * editing one character would otherwise read, modify and write the same blob
 * and the later write would discard the earlier silently.
 */
export function appendEvents(
  id: string,
  expectedSeq: number,
  events: CharacterEvent[],
): Promise<WriteResponse> {
  return request<WriteResponse>(`/characters/${id}/events`, {
    method: 'POST',
    body: { expectedSeq, events },
  })
}

/**
 * Drops every event after `after`: the Back button, and un-taking a level.
 *
 * Not what changing a pick needs -- answers fold last-write-wins, so
 * re-answering a prompt is a plain append.
 */
export function truncateEvents(
  id: string,
  expectedSeq: number,
  after: number,
): Promise<WriteResponse> {
  return request<WriteResponse>(
    `/characters/${id}/events?after=${after}&expectedSeq=${expectedSeq}`,
    { method: 'DELETE' },
  )
}

export function deleteCharacter(id: string): Promise<void> {
  return request<void>(`/characters/${id}`, { method: 'DELETE' })
}
