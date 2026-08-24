import { request } from './client'

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

export interface Spell extends Entry {
  level: number
  school?: string
  classes?: string[]
}

/**
 * Every collection is fetched at most once per session.
 *
 * The promise is cached rather than the value, so two components mounting in
 * the same tick make one request rather than two. It is never invalidated: the
 * compendium is immutable for the life of the server process, and a new
 * release reloads the page.
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

/** Clears the session cache. For tests. */
export function resetCatalogCache(): void {
  cache.clear()
}

export function getManifest(): Promise<Manifest> {
  return cached('manifest', () => request<Manifest>('/catalog'))
}

/** Fetches a whole collection, typed by the caller. */
export function getCollection<T extends Entry>(collection: string): Promise<T[]> {
  return cached(`collection:${collection}`, () => request<T[]>(`/catalog/${collection}`))
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
