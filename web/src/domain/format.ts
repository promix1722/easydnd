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
 * The six abilities' full names, keyed by the slug the API uses.
 *
 * Hardcoded for the same reason the create screen hardcodes the list: these
 * six are the one part of the compendium that cannot change, and "Dex +1" on
 * a button is worse than "Dexterity +1" for the sake of a round trip. The
 * catalogue's abilities collection carries the same names for anywhere that
 * needs them localized.
 */
const ABILITY_NAMES: Record<string, string> = {
  str: 'Strength',
  dex: 'Dexterity',
  con: 'Constitution',
  int: 'Intelligence',
  wis: 'Wisdom',
  cha: 'Charisma',
}

/** An ability's full name, or the slug title-cased if it is not one. */
export function abilityName(slug: string): string {
  return ABILITY_NAMES[slug] ?? titleCase(slug)
}
