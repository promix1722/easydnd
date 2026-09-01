import { request } from './client'
import { requestLocale } from './locale'

/**
 * The compendium.
 *
 * Shapes mirror internal/api/http/v1/catalog. They are the *API's* format,
 * not the on-disk one: prose is already resolved into the negotiated locale,
 * and every option carries the key an answer names it by.
 */

export interface Entry {
  slug: string
  name: string
  desc?: string[]
}

/** One selectable answer to a prompt. */
export interface Option {
  /**
   * What an answer names this option by. The client never computes this --
   * a bundle of a shortbow and twenty arrows has no slug of its own, and the
   * rule for naming one lives on the server.
   */
  key: string
  kind:
    | 'ref'
    | 'nested'
    | 'bundle'
    | 'ability-bonus'
    | 'damage'
    | 'money'
    | 'size'
    | 'text'
    | 'action'
    | 'score-minimum'
  ref?: string
  count?: number
  choice?: Choice
  items?: Option[]
  ability?: string
  bonus?: number
  minimum?: number
  damage?: { dice: string; type?: string }
  cost?: { amount: number; unit: string }
  size?: string
  /** Resolved prose, for the options that are prose. */
  text?: string
  alignments?: string[]
  recharge?: string
}

export interface OptionSet {
  kind: 'explicit' | 'equipment-category' | 'collection'
  options?: Option[]
  /** Set when the options are "any item in this category". */
  category?: string
  /** Set when the options are "any member of this collection". */
  collection?: string
}

/** "Choose N of these", nesting arbitrarily deep. */
export interface Choice {
  prompt: string
  choose: number
  kind: string
  from: OptionSet
}

export interface CollectionInfo {
  name: string
  count: number
}

export interface Manifest {
  ruleset: string
  locale: string
  locales: string[]
  collections: CollectionInfo[]
}

export interface AbilityBonus {
  ability: string
  bonus: number
}

export interface ItemStack {
  item: string
  count: number
}

export interface Race extends Entry {
  speed: number
  size?: string
  abilityBonuses?: AbilityBonus[]
  abilityBonusOptions?: Choice
  languages?: string[]
  traits?: string[]
  subraces?: string[]
}

export interface Class extends Entry {
  hitDie: number
  savingThrows?: string[]
  subclasses?: string[]
  /** The class level at which the subclass is chosen, derived server-side. */
  subclassLevel?: number
  spellcasting?: { level: number; ability: string }
}

export interface Item extends Entry {
  category?: string
  cost?: { amount: number; unit: string }
  weight?: number
  armor?: { category?: string; baseAC: number }
  weapon?: { category?: string; range?: string; damage?: { dice: string; type?: string } }
}

export interface Skill extends Entry {
  ability: string
}

export interface Proficiency extends Entry {
  /**
   * What the proficiency applies to: "armor", "weapons", "artisans-tools",
   * "gaming-sets", "musical-instruments", "other-tools", "vehicles",
   * "skills", "saving-throws".
   *
   * Optional because the wire omits it when the compendium has no type for an
   * entry, not because a client may invent one.
   */
  type?: string
  /** What it is a proficiency *in*, as "kind:slug". Absent when it stands alone. */
  reference?: string
}

/**
 * A structured rule string: a casting time, a range, a duration.
 *
 * The SRD writes these as prose -- "1 action", "90 feet", "Up to 1 minute" --
 * and the compendium stores them structured so the client can render them per
 * locale. Which optional fields apply depends on the kind; see
 * features/spells/spellText.ts for the rendering.
 */
export interface RuleValue {
  kind: string
  amount?: number
  unit?: string
  distance?: number
  upTo?: boolean
}

export interface SpellComponents {
  verbal?: boolean
  somatic?: boolean
  material?: boolean
  consumed?: boolean
  /** The material component's description. Detail only, never on a summary. */
  text?: string
}

/**
 * The collection serves the summary fields -- through `components`, enough to
 * search and filter -- and `?slugs=` fills in the rest. Every field past
 * `level` is optional because the wire omits its zero value.
 */
export interface Spell extends Entry {
  source?: string
  level: number
  school?: string
  classes?: string[]
  subclasses?: string[]
  ritual?: boolean
  concentration?: boolean
  castingTime?: RuleValue
  components?: SpellComponents
  range?: RuleValue
  duration?: RuleValue
  attackType?: string
  savingThrow?: { ability: string; success?: string }
  areaOfEffect?: { shape: string; size: number }
  damage?: {
    type?: string
    atSlotLevel?: Record<string, string>
    atCharacterLevel?: Record<string, string>
  }
  healing?: Record<string, string>
  higherLevel?: string[]
}

/**
 * Every collection is fetched at most once per session, per language.
 *
 * The promise is cached rather than the value, so two components mounting in
 * the same tick make one request rather than two. The compendium is immutable
 * for the life of the server process, so nothing here expires -- but it is
 * immutable *per locale*, and the entries carry prose the server has already
 * resolved. Keying on the collection alone is therefore a bug with a delay
 * on it: switch to Russian and every collection already fetched keeps
 * answering in English, for as long as the tab stays open.
 */
const cache = new Map<string, Promise<unknown>>()

function cached<T>(key: string, load: () => Promise<T>): Promise<T> {
  const hit = cache.get(key)
  if (hit) return hit as Promise<T>
  // Cache the failure too, then evict it: a rejected promise left in the map
  // would make one network blip permanent for the rest of the session.
  const started = load().catch((cause: unknown) => {
    cache.delete(key)
    throw cause
  })
  cache.set(key, started)
  return started
}

/**
 * Clears the session cache.
 *
 * Used by the tests, and by the locale provider on a language change. The key
 * carries the locale so a switch cannot serve the wrong language, but nothing
 * would ever evict the abandoned one -- and holding both copies of a 1.5 MB
 * compendium to serve one of them is not a trade worth making.
 */
export function resetCatalogCache(): void {
  cache.clear()
}

export function getManifest(): Promise<Manifest> {
  return cached(`manifest:${requestLocale()}`, () => request<Manifest>('/catalog'))
}

/** Fetches a whole collection, typed by the caller. */
export function getCollection<T extends Entry>(collection: string): Promise<T[]> {
  return cached(`collection:${requestLocale()}:${collection}`, () =>
    request<T[]>(`/catalog/${collection}`),
  )
}

/**
 * Fetches named entries of a collection.
 *
 * Uncached, because the set of slugs varies per call and caching per-slug-set
 * would fill the map with near-duplicates. Callers wanting the whole thing
 * should ask for the whole thing.
 */
export function getEntries<T extends Entry>(collection: string, slugs: string[]): Promise<T[]> {
  if (slugs.length === 0) return Promise.resolve([])
  const query = encodeURIComponent(slugs.join(','))
  return request<T[]>(`/catalog/${collection}?slugs=${query}`)
}

/** Indexes a collection by slug, for the lookups a sheet does constantly. */
export function bySlug<T extends Entry>(entries: T[]): Map<string, T> {
  return new Map(entries.map((entry) => [entry.slug, entry]))
}

/** One search over the spells collection. Every field optional; see search.go. */
export interface SpellSearch {
  q?: string
  level?: number
  school?: string
  class?: string
  castingTime?: string
  concentration?: boolean
  ritual?: boolean
  material?: boolean
  limit?: number
  offset?: number
}

export interface SpellPage {
  spells: Spell[]
  total: number
}

/**
 * Searches the spells collection server-side: filtered, sorted, paged.
 *
 * Uncached, because the query varies per keystroke. The server only answers
 * in this envelope when at least one parameter is sent -- callers here always
 * send `limit` -- otherwise the same route serves the bare array
 * `getCollection` expects.
 */
export function searchSpells(search: SpellSearch, signal?: AbortSignal): Promise<SpellPage> {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(search)) {
    if (value !== undefined && value !== '') params.set(key, String(value))
  }
  return request<SpellPage>(`/catalog/spells?${params.toString()}`, signal ? { signal } : {})
}
