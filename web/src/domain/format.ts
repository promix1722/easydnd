/**
 * Display helpers for values the API sends as data.
 *
 * They live in src/domain/ because they are pure and framework-free: no React,
 * no transport, no I/O. That is what this directory was reserved for, and
 * these are the first things to earn a place in it.
 *
 * Note what is *not* here. Nothing computes a rule. Ability modifiers,
 * proficiency bonuses and armor class all arrive derived, because the Go model
 * is the source of truth for the rules and a second implementation in the
 * browser is a second implementation to disagree.
 */

/** Renders a bonus the way a sheet prints it: +3, -1, +0. */
export function signed(n: number): string {
  return n >= 0 ? `+${n}` : String(n)
}

/**
 * Turns a slug into something a person reads: "half-elf" into "Half Elf".
 *
 * A fallback, not a translation. Where the catalogue has a name, use it -- it
 * is already in the negotiated locale, and this is not.
 */
export function titleCase(slug: string): string {
  return slug
    .split('-')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

/** The slug half of a typed reference: "race:half-elf" gives "half-elf". */
export function slugOf(ref: string): string {
  return ref.split(':')[1] ?? ref
}

/** The kind half of a typed reference: "race:half-elf" gives "race". */
export function kindOf(ref: string): string {
  return ref.split(':')[0] ?? ''
}

/**
 * The six abilities, in the order every sheet in the game prints them.
 *
 * This order is the whole reason the constant exists. Scores and saving
 * throws arrive as objects keyed by slug, and a Go map serialises its keys
 * sorted, so a screen that walks the response as it came prints CHA, CON,
 * DEX, INT, STR, WIS -- alphabetical, and unreadable to anyone who has held a
 * character sheet. Anything drawing more than one ability in sequence walks
 * this list instead of the response. (Skills are a different case: there are
 * eighteen of them in no traditional order, so those really are alphabetical.)
 *
 * Hardcoded rather than fetched. These six are the one part of the compendium
 * that cannot change -- a sixth-and-a-half ability would be a different game
 * -- so waiting on a round trip to draw six labelled inputs would be a worse
 * first screen for no benefit. Their *names* are not here: a name is prose,
 * and prose is not what this directory is for. See
 * `features/character/labels.ts`.
 */
export const ABILITY_ORDER = ['str', 'dex', 'con', 'int', 'wis', 'cha'] as const

const CANONICAL = new Set<string>(ABILITY_ORDER)

/**
 * The entries of anything keyed by ability slug, in the canonical order.
 *
 * Two decisions worth stating, because both are about a projection that is
 * not the six keys the sheet expects. A slug the response *omits* draws
 * nothing at all, since a blank card claiming a missing score is worse than a
 * row of five. A slug the six do not cover is kept and drawn last rather than
 * filtered out: an unrecognised ability on a sheet means the server and this
 * client disagree about the game, which is a thing to see rather than a thing
 * to hide.
 */
export function abilitiesInOrder<T>(byAbility: Record<string, T>): [string, T][] {
  const canonical = ABILITY_ORDER.flatMap<[string, T]>((slug) => {
    const value = byAbility[slug]
    return value === undefined ? [] : [[slug, value]]
  })
  return [...canonical, ...Object.entries(byAbility).filter(([slug]) => !CANONICAL.has(slug))]
}
