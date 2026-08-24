import { titleCase } from './format'

/** How many levels a character has in one class. */
export interface ClassLevel {
  class: string
  subclass?: string
  level: number
}

/**
 * Renders a class line: "Rogue 3", or "Cleric 2 / Wizard 1" for a
 * multiclassed character.
 *
 * The order is the order the classes were taken, which the API preserves --
 * the first is the class the character started as, and that is the one whose
 * saving throws and starting equipment they got.
 */
export function classLine(classes: readonly ClassLevel[] | undefined): string {
  if (classes === undefined || classes.length === 0) return '--'
  return classes.map((c) => `${titleCase(c.class)} ${c.level}`).join(' / ')
}
