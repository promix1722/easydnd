import { titleCase } from './format'

/**
 * Display helpers for the character's event log.
 *
 * The log is the record and the sheet is a projection of it, so these render
 * what was *stored* rather than what it evaluated to: an event's own type, its
 * timestamp, the answers it carried and the changes it addressed. Nothing here
 * folds, resolves or derives -- Project() in the Go model does that, and a
 * second fold in the browser is a second fold to disagree.
 */

/** A change's value, as the wire carries it: a tagged union, not `any`. */
export interface ValueLike {
  kind: string
  int?: number
  string?: string
  bool?: boolean
  slug?: string
  slugs?: string[]
  dice?: string
}

/** One addressed mutation: "abilities.dex", set, 16. */
export interface ChangeLike {
  path: string
  op: string
  value: ValueLike
}

/**
 * Ref kinds to the catalogue collection that holds them.
 *
 * One table, two jobs: naming a reference the log stored, and finding the
 * options when a prompt says "any member of this collection". It used to be
 * two tables in two files -- this one and BuildScreen's own COLLECTION_OF --
 * which is one table with two spellings, and the spelling that was missing an
 * entry was whichever one you were not looking at.
 */
const REF_COLLECTIONS: Record<string, string> = {
  race: 'races',
  subrace: 'subraces',
  background: 'backgrounds',
  class: 'classes',
  subclass: 'subclasses',
  feat: 'feats',
  trait: 'traits',
  feature: 'features',
  item: 'equipment',
  'magic-item': 'magic-items',
  spell: 'spells',
  language: 'languages',
  proficiency: 'proficiencies',
  alignment: 'alignments',
  skill: 'skills',
  condition: 'conditions',
  'damage-type': 'damage-types',
}

/**
 * Which collection would name a reference of this kind, if any.
 *
 * Null for the kinds the compendium does not serve as a collection, which is
 * the signal to fall back to the slug.
 */
export function collectionOfKind(kind: string): string | null {
  return REF_COLLECTIONS[kind] ?? null
}

/**
 * Segments of a prompt path that carry no meaning to a reader: the namespace
 * the character itself poses prompts under, and the indices that distinguish
 * one repeat of a choice from the next.
 */
function meaningfulSegments(path: string): string[] {
  const segments = path.split('/').filter((segment) => !/^\d+$/.test(segment))
  return segments[0] === 'character' ? segments.slice(1) : segments
}

/**
 * A prompt's slug as a heading: "character/race" reads back as "Race", and
 * "skill-versatility/proficiency/0" as "Skill Versatility - Proficiency".
 *
 * Prompt slugs are paths, not names. They are namespaced by whatever posed the
 * choice and numbered where a source poses the same choice twice, and both of
 * those are addressing rather than anything a player recognises -- but the
 * namespace is worth keeping, because "Proficiency" alone does not say which
 * of the three things that granted one is being answered.
 */
export function promptLabel(prompt: string): string {
  const segments = meaningfulSegments(prompt)
  if (segments.length === 0) return titleCase(prompt)
  return segments.map(titleCase).join(' \u00b7 ')
}

/**
 * One pick, as a person reads it.
 *
 * A pick is the *key* an option is named by, which is usually a slug but is a
 * path when the option was itself another prompt. In that case the nested
 * answer follows in the same event, so naming the branch is enough.
 */
export function pickLabel(pick: string): string {
  if (!pick.includes('/')) return titleCase(pick)
  const segments = meaningfulSegments(pick)
  return titleCase(segments[segments.length - 1] ?? pick)
}
