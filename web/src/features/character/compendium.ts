/**
 * The compendium the sheet body draws names out of.
 *
 * Its own module rather than a second export beside the component, because a
 * file that exports both a component and a function loses fast refresh -- and
 * because both sheet screens import this without wanting the component.
 */
import type { CatalogProficiency, CatalogSkill, Entry, Item } from '@/lib/api'
import { bySlug, getCollection } from '@/lib/api'

/**
 * The collections the identity table names things out of.
 *
 * Fetched together and flattened into one map keyed by collection *and* slug:
 * two collections may use the same slug, and a bare slug map would let a
 * background quietly rename a class. Each is session-cached, and the build
 * screen has usually warmed the identity collections before a sheet is opened.
 */
const NAMED = [
  'races',
  'subraces',
  'classes',
  'subclasses',
  'backgrounds',
  'traits',
  'features',
  'languages',
  'equipment',
  'spells',
] as const

async function namesOf(scope = ''): Promise<Map<string, string> | null> {
  try {
    const collections = await Promise.all(
      NAMED.map((collection) => getCollection<Entry>(collection, scope)),
    )
    const names = new Map<string, string>()
    collections.forEach((entries, at) => {
      for (const entry of entries) names.set(`${NAMED[at]}:${entry.slug}`, entry.name)
    })
    return names
  } catch {
    // The sheet is worth drawing with title-cased slugs; it is not worth
    // losing to a compendium request, as with the prompts above.
    return null
  }
}

/** What SheetBody needs out of the compendium to draw names rather than slugs. */
export interface Compendium {
  names: Map<string, string> | null
  skills: Map<string, CatalogSkill> | null
  proficiencies: Map<string, CatalogProficiency> | null
  /**
   * Equipment and magic items by slug: what the Equipment tab sorts and slots
   * by. Optional so a caller that draws no equipment need not load it.
   */
  items?: Map<string, Item> | null
}

/**
 * Fetch the collections the body draws.
 *
 * One loader rather than each screen assembling its own, so an owner's sheet
 * and the one their table opens cannot end up naming things out of different
 * sets. Every request is session-cached, so this is one round of them for the
 * whole visit however many sheets are opened, and each falls back to null
 * rather than failing the sheet -- title-cased slugs are worth drawing.
 */
export async function loadCompendium(scope = ''): Promise<Compendium> {
  const [skills, proficiencies, names, items] = await Promise.all([
    getCollection<CatalogSkill>('skills', scope).then(bySlug, () => null),
    getCollection<CatalogProficiency>('proficiencies', scope).then(bySlug, () => null),
    namesOf(scope),
    loadItems(scope).catch(() => null),
  ])
  return { names, skills, proficiencies, items }
}

/** Every item a character could hold, mundane and magic, by slug. */
export async function loadItems(scope = ''): Promise<Map<string, Item>> {
  const [equipment, magic] = await Promise.all([
    getCollection<Item>('equipment', scope),
    getCollection<Item>('magic-items', scope),
  ])
  return bySlug([...magic, ...equipment])
}
