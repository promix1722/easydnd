import type { BuildPolicy } from './packPolicy'
import type { RulesLock } from './packs'
import type { Choice, Entry, Item, Option, Proficiency, Skill as CatalogSkill, Spell } from './catalog'
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
  image?: string
  id: string
  /** The folder the character is filed in. Always set: a character is never
   * in no folder, so a listing can group by this without a fallback bucket. */
  folder: string
  name: string
  level: number
  classes?: ClassLevel[]
}

export interface Identity {
  image?: string
  name: string
  alignment?: string
  race?: string
  subrace?: string
  background?: string
  classes?: ClassLevel[]
  level: number
  /**
   * The level the player declared they are building towards. While `level` is
   * below it, the server keeps one level's choices open. Absent when never
   * declared.
   */
  desiredLevel?: number
  /** The rules the character was built under, e.g. "2014". Absent when never
   * recorded, which reads as the compendium's own. */
  ruleset?: string
  /**
   * Experience points, recorded rather than acted on: a level comes from a
   * level event, so crossing a threshold advances nobody. A table playing
   * milestones leaves it at zero.
   */
  experience: number
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
  /** The equipped item in the sheet's Custom slot, the one placement the server stores. */
  custom?: string
  purse?: Record<string, number>
}

export interface Pool {
  key?: string
  max: number
  used?: number
  recharge?: string
  dice?: string
}

/** A spendable resource a pack declares; `used` on a sheet is always the log's. */
export interface ResourcePool {
  id: string
  name: string
  group?: string
  max: number
  used: number
  dice?: string
  slotLevel?: number
}

/** A scaling value that is read, never spent: a Sneak Attack die, an aura range. */
export interface ResourceParameter {
  name: string
  number: number
  dice?: string
  text?: string
  rational?: { numerator: number; denominator: number }
  boolean?: boolean
}

export interface Resources {
  pools?: Record<string, ResourcePool>
  parameters?: Record<string, ResourceParameter>
  /** Compatibility views of the same pools; nothing new should read them. */
  spellSlots?: Record<string, Pool>
  hitDice?: Pool[]
  class?: Pool[]
}

export interface SpellSource {
  source: string
  class?: string
  ability?: string
  cantrips?: string[]
  known?: string[]
  spellbook?: string[]
  prepared?: string[]
  arcanum?: string[]
  mastery?: string[]
  preparationLimit?: number
}

export interface CustomOption {
 ref?: string
 id?: string
 kind: string
 name: string
 description: string
 source: string
 parent?: string
 ability?: string
 mode?: string
 placement?: string
 level?: number
 hitDie?: number
 speed?: number
 count?: number
 selected: boolean
}
export const upsertCustomOption = (id: string, revision: number, option: CustomOption) =>
 request<WriteResponse>(`${characterPath(id)}/custom-options`, { method: 'POST', body: { revision, option } })
export interface Sheet {
 customOptions?: CustomOption[]
 importSession?: string
 catalogNames?: Record<string,string>
  /**
   * What the sheet's slugs mean, resolved by the server in the same response:
   * the entries a panel reads more than a name from. Present on a sheet that
   * was read, absent on the one a write echoes back.
   */
  catalog?: {
    skills: CatalogSkill[]
    proficiencies?: Proficiency[]
    equipment?: Item[]
    magicItems?: Item[]
    /** The prose behind `actions`, each entry's `slug` being an action's `origin`. */
    actions?: Entry[]
    /** The prose behind the "Traits and features" rows; only the entries that have any. */
    traits?: Entry[]
    features?: Entry[]
    languages?: Entry[]
    spells?: Spell[]
  }
 importedNotes?: string[]
  identity: Identity
  base: Base
  abilities: Abilities
  skills: Record<string, Skill>
  savingThrows: Record<string, SavingThrow>
  status: Status
  equipment: Equipment
  resources: Resources
  spells: { sources?: SpellSource[]; cantrips?: string[]; known?: string[]; prepared?: string[]; ability?: string }
  actions?: SheetAction[]
  feats?: string[]
  traits?: string[]
  features?: string[]
  conditions?: string[]
  proficiencies?: string[]
}

/** Something the character can do on their turn, as the server derived it. */
export interface SheetAction {
  source: string
  /** What produced it, as `kind:slug`; also the key of its prose in `catalog.actions`. */
  origin?: string
  kind: string
  /** Why the character has it: `basic`, `equipment` or `feature`. */
  category?: string
  name: string
  range?: number
  toHit?: number
  damage?: string
  uses?: string
  notes?: string
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
 observed?: boolean
 evidence?: string
  choiceSource?: string
  choiceKind?: string
  purpose?: string
  id?: string
  selections?: Option[]
  seq?: number
  type: string
  at?: string
  ref?: string
  level?: number
  choices?: Answer[]
  changes?: Change[]
  note?: string
  /**
   * The group of the prompt this entry answered, written by the server.
   *
   * Never sent: a client-supplied source would be a second, unverified
   * vocabulary for what an answer means, and the server already knows which
   * prompt it accepted. Absent where nothing can be attributed -- an imported
   * log, a DM's adjustment -- which is what puts those entries in no tab and
   * leaves `/characters/:id/log` as the unabridged record.
   */
  source?: string
}

/** What the answer to a prompt must be posted as. */
export interface PromptEvent {
  type: string
  ref?: string
  level?: number
}

export interface Prompt {
  blocked?: string[]
  purpose?: string
  upTo?: boolean
  choice: Choice
  /** The catalogue entry posing this prompt, as "kind:slug". */
  source?: string
  group: string
  level?: number
  /** A character is complete without answering an optional prompt. */
  optional: boolean
  /** What the answer must be posted as. */
  event: PromptEvent
  /** Options the character already has from another source. */
  held?: string[]
  /**
   * Inverts what `held` means: those are the only legal answers rather than
   * the illegal ones. Expertise is the case -- it doubles a proficiency the
   * character already has.
   */
  heldOnly: boolean
  /** The class's advice for the six scores: every ability, most important first. On the ability-score prompt only. */
  recommended?: string[]
}

export interface SpellRule {
  maxLevelCount?: number
  id: string
  source: string
  class?: string
  classLevel?: number
  count: number
  minLevel: number
  maxLevel: number
  purpose: string
  optional: boolean
  listClasses?: string[]
  automatic?: string[]
}

export interface PromptsResponse {
  buildPolicy?: BuildPolicy
  spellRules?: SpellRule[]
  revision?: number
  seq: number
  /** Nothing required is outstanding. Separate from the list being empty: a
   * finished character can still carry optional prompts. */
  complete: boolean
  prompts: Prompt[]
}

export interface WriteResponse {
  revision?: number
  seq: number
  sheet: Sheet
}

/** Why an entry did not survive a replacement. */
export type DropReason =
  /** The character is no longer being asked the question it answered. */
  | 'not-offered'
  /** It stayed, but some of its answers did not. */
  | 'answers-dropped'
  /** Everything it said is gone, so there is nothing left to keep. */
  | 'empty'

/** One answer a replacement invalidated, named the way the server names it. */
export interface LostAnswer {
  prompt: string
  picks?: string[]
  /** The `validateAnswer` vocabulary: no new words on the wire. */
  rule: string
  message?: string
}

/**
 * An entry a replacement cost, reported before it is paid for.
 *
 * `seq` is the entry's **original** position: the log is renumbered on the way
 * out, so the number that identifies it to the reader is the one it had when
 * they wrote it.
 */
export interface Dropped {
  seq: number
  type: string
  ref?: string
  level?: number
  source?: string
  reason: DropReason
  lost?: LostAnswer[]
}

/**
 * The write response, plus what the replacement cost.
 *
 * A dry run and its commit return the same shape from the same code path, so
 * a preview that disagrees with what actually happens is not a thing that can
 * be built here.
 */
export interface ReviseResponse extends WriteResponse {
  dropped?: Dropped[]
}

export interface CreateResponse {
  id: string
  seq: number
  sheet: Sheet
}

/**
 * Everything creating a character takes: a name.
 *
 * It used to take the score method and all six scores too, and that was eight
 * selections written as one entry -- which meant the name and the scores had
 * nothing a player could point at and change. They are ordinary open choices
 * now, answered from their own tabs and each written as its own entry.
 */
export interface NewCharacter {
  image?: string
  rules?: RulesLock
  name: string
  alignment?: string
  /** Where to file it. Omitted means the account's default folder. */
  folder?: string
}

/**
 * Lists the account's characters, optionally narrowing to one folder.
 *
 * A folder the account does not own is a 404 rather than an empty list, so an
 * unowned id cannot be mistaken for a folder with nothing in it.
 */
export function listCharacters(
  folder?: string,
  signal?: AbortSignal,
): Promise<{ characters: Summary[] }> {
  const path = folder ? `/characters?folder=${encodeURIComponent(folder)}` : '/characters'
  return request<{ characters: Summary[] }>(path, signal ? { signal } : {})
}

export function createCharacter(body: NewCharacter): Promise<CreateResponse> {
  return request<CreateResponse>('/characters', { method: 'POST', body })
}

export function getSheet(id: string, signal?: AbortSignal): Promise<Sheet> {
  return request<Sheet>(`${characterPath(id)}/sheet`, signal ? { signal } : {})
}

export function getPrompts(id: string, signal?: AbortSignal, before?: number): Promise<PromptsResponse> {
  const query = before === undefined ? '' : `?before=${before}`
  return request<PromptsResponse>(`${characterPath(id)}/prompts${query}`, signal ? { signal } : {})
}

export function getEvents(
  id: string,
  signal?: AbortSignal,
): Promise<{ seq: number; revision?: number; rules?: RulesLock; events: CharacterEvent[] }> {
  return request<{ seq: number; revision?: number; rules?: RulesLock; events: CharacterEvent[] }>(
    `${characterPath(id)}/events`,
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
  expectedRevision = expectedSeq,
): Promise<WriteResponse> {
  return request<WriteResponse>(`${characterPath(id)}/events`, {
    method: 'POST',
    body: { expectedSeq, expectedRevision, events },
  })
}

/**
 * Replaces one entry in place, revalidating everything after it.
 *
 * This is the whole of changing your mind: there is no append-a-correction
 * path, because a correction that does not sit where the original sat leaves
 * the entries between them meaning what they meant before it. The server
 * replays the suffix against the log as rebuilt *so far*, so an entry is
 * judged by what came before it and nothing is circular.
 *
 * `expectedSeq` guards the whole log, exactly as it does on append. It is also
 * what makes a stale preview safe: if the log moved between the dry run and
 * the commit, the commit is a sequence conflict rather than a quiet surprise.
 *
 * `dryRun` runs every line of that except the store, so the `dropped` list a
 * player is shown is produced by the code that will do the work.
 */
export function replaceEvent(
  id: string,
  seq: number,
  expectedSeq: number,
  event: CharacterEvent,
  dryRun = false,
  expectedRevision = expectedSeq,
): Promise<ReviseResponse> {
  return request<ReviseResponse>(`${characterPath(id)}/events/${seq}${dryRun ? '?dryRun=true' : ''}`, {
    method: 'PUT',
    body: { expectedSeq, expectedRevision, event },
  })
}

/**
 * Removes one entry, revalidating everything after it.
 *
 * The same mechanism as `replaceEvent` with nothing put back: un-taking a
 * level, and dropping an answer so its question comes back outstanding.
 *
 * `expectedSeq` travels in the query rather than in a body, matching the
 * truncate route it sits beside -- a DELETE with a body is legal and widely
 * mishandled, and there is nothing here that a query string cannot carry.
 */
export function deleteEvent(
  id: string,
  seq: number,
  expectedSeq: number,
  dryRun = false,
  expectedRevision = expectedSeq,
): Promise<ReviseResponse> {
  return request<ReviseResponse>(
    `${characterPath(id)}/events/${seq}?expectedSeq=${expectedSeq}&expectedRevision=${expectedRevision}${dryRun ? '&dryRun=true' : ''}`,
    { method: 'DELETE' },
  )
}

/**
 * Drops every event after `after`.
 *
 * Nothing in this client calls it any more -- changing an answer is
 * `replaceEvent`, and un-taking a level is `deleteEvent` -- but it is working,
 * tested API, and withdrawing it would be a breaking change made as a side
 * effect of a decision about a screen.
 */
export function truncateEvents(
  id: string,
  expectedSeq: number,
  after: number,
  expectedRevision = expectedSeq,
): Promise<WriteResponse> {
  return request<WriteResponse>(
    `${characterPath(id)}/events?after=${after}&expectedSeq=${expectedSeq}&expectedRevision=${expectedRevision}`,
    { method: 'DELETE' },
  )
}

export function deleteCharacter(id: string): Promise<void> {
  return request<void>(`${characterPath(id)}`, { method: 'DELETE' })
}

/**
 * Files a character in another folder.
 *
 * Its own route rather than a PATCH on the character, because the folder is the
 * one thing about a stored character that changes without an event -- a name, a
 * level or a score can only change by appending to the log.
 *
 * An empty folder means the account's default.
 */
export function moveCharacter(id: string, folder: string): Promise<void> {
  return request<void>(`${characterPath(id)}/folder`, { method: 'PUT', body: { folder } })
}

/**
 * Duplicates a character, log and all.
 *
 * The copy is named after the original with " (copy)" on the end, and lands
 * beside it unless another folder is named.
 */
export function copyCharacter(id: string, folder?: string): Promise<CreateResponse> {
  return request<CreateResponse>(`${characterPath(id)}/copy`, {
    method: 'POST',
    body: { folder: folder ?? '' },
  })
}

/** Atomically save edits to several past choices together with new answers. */
export function reviseEvents(id: string, expectedSeq: number, expectedRevision: number,
  replacements: { seq: number; event: CharacterEvent }[], events: CharacterEvent[], dryRun = false,
): Promise<ReviseResponse> {
  return request<ReviseResponse>(`${characterPath(id)}/events/revise${dryRun ? '?dryRun=true' : ''}`, {
    method: 'POST', body: { expectedSeq, expectedRevision, replacements, events },
  })
}

/** Where one character's own routes live. */
export function characterPath(id: string): string {
  return `/characters/${encodeURIComponent(id)}`
}

/**
 * Dresses a character who has nothing on: one suitable item from the backpack
 * in each slot. The build screen calls it on Finish; a character already
 * wearing anything is left as it is, so calling it again is harmless.
 */
export function autoEquip(id: string): Promise<void> {
  return request<void>(`${characterPath(id)}/auto-equip`, { method: 'POST' })
}

/** Who may read a character besides its owner and the groups it is shared with. */
export interface Visibility {
  /** Open to anybody signed in who has its link: a read of the sheet, never a write. */
  public: boolean
}

export function getVisibility(id: string, signal?: AbortSignal): Promise<Visibility> {
  return request<Visibility>(`${characterPath(id)}/visibility`, signal ? { signal } : {})
}

export function setVisibility(id: string, visible: boolean): Promise<Visibility> {
  return request<Visibility>(`${characterPath(id)}/visibility`, { method: 'PUT', body: { public: visible } })
}
