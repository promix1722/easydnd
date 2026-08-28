/**
 * Dice that get thrown, as opposed to dice that get printed.
 *
 * `domain/abilities.ts` already holds `d6`, and it stays there: it is part of
 * how the six scores are arrived at, which is a rule of character creation
 * rather than a fact about dice. This file is the other kind -- a die somebody
 * rolls because rolling it is the point.
 *
 * Framework-free like the rest of `domain/`, which is what lets `ui/D20.tsx`
 * take it as a default and a test hand it a loaded one instead.
 */

/** The faces a d20 has, and therefore how many the solid in `ui/D20.tsx` draws. */
export const D20_FACES = 20

/** One fair twenty-sided die. */
export function d20(): number {
  return Math.floor(Math.random() * D20_FACES) + 1
}
