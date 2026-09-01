import { bySlug, getEntries } from '@/lib/api'
import type { Entry } from '@/lib/api'

import { collectionOfKind, kindOf, slugOf, titleCase } from '@/domain'

/** Anything that names a catalogue entry: a stored event, a dropped one. */
export interface RefBearing {
  ref?: string
  source?: string
  choices?: { prompt: string; picks: string[] }[]
}

const PICK_COLLECTIONS: Record<string, string> = {
  proficiency: 'proficiencies',
  expertise: 'proficiencies',
  multiclass: 'proficiencies',
  'starting-equipment': 'equipment',
  language: 'languages',
  spell: 'spells',
  subfeature: 'features',
  subtrait: 'traits',
}

/**
 * Looks up the compendium's name for every reference a set of entries makes.
 *
 * One request per collection rather than one per entry, and a failure yields
 * no names rather than no page: the slug is a worse label than the name, but
 * it is a much better label than an error. That rule is why this is a map
 * lookup at the call site rather than a component that can fail.
 *
 * It lives here rather than inside the log screen because three screens now
 * want it -- the log, the build screen's settled rows, and the list of what a
 * change would drop -- and the last of those is not even reading stored
 * events. Anything carrying a `ref` will do.
 */
export async function resolveRefNames(
  entries: readonly RefBearing[],
): Promise<Map<string, string>> {
  const wanted = new Map<string, Map<string, string>>()
  for (const entry of entries) {
    for (const ref of [entry.ref, entry.source]) {
      if (ref === undefined) continue
      const collection = collectionOfKind(kindOf(ref))
      if (collection !== null) {
        const slugs = wanted.get(collection) ?? new Map<string, string>()
        slugs.set(slugOf(ref), ref)
        wanted.set(collection, slugs)
      }
    }
    for (const answer of entry.choices ?? []) {
      const collection = answer.prompt
        .split('/')
        .map((part) => PICK_COLLECTIONS[part])
        .find((part) => part !== undefined)
      if (collection === undefined) continue
      const slugs = wanted.get(collection) ?? new Map<string, string>()
      for (const pick of answer.picks) {
        // A nested option is an address, not a catalogue slug. Its following
        // answer carries the actual referenced choice.
        if (!pick.includes('/')) slugs.set(pick, pick)
      }
      wanted.set(collection, slugs)
    }
  }

  const names = new Map<string, string>()
  await Promise.all(
    [...wanted].map(async ([collection, slugs]) => {
      try {
        const loaded = bySlug(await getEntries<Entry>(collection, [...slugs.keys()]))
        for (const [slug, ref] of slugs) {
          const found = loaded.get(slug)
          if (found !== undefined) names.set(ref, found.name)
        }
      } catch {
        // Slugs alone. See the doc comment.
      }
    }),
  )
  return names
}

/** A reference as a person reads it: the compendium's name, or the slug. */
export function refName(ref: string, names: ReadonlyMap<string, string>): string {
  return names.get(ref) ?? titleCase(slugOf(ref))
}
