/**
 * How the six numbers are arrived at, as the 2014 rules state it.
 *
 * Three of the four methods are rules rather than preferences -- the array is
 * printed, the costs are tabulated, the dice are specified -- so they live
 * here with the rest of the game model rather than inside a form. Framework-
 * free, like everything in domain/: no React, and no randomness this file
 * cannot be asked to do without.
 */

/**
 * The standard array, as the SRD prints it.
 *
 * Highest first, which is the order it is printed in and the order it is
 * easiest to assign from. Nothing here says which ability wants the 15.
 */
export const STANDARD_ARRAY: readonly number[] = [15, 14, 13, 12, 10, 8]

/** What a point-buy character has to spend. */
export const POINT_BUY_BUDGET = 27

/** Every score starts here, and 8 costs nothing. */
export const POINT_BUY_MIN = 8

/** Point buy stops short of 16: the last two steps are what make it a cost. */
export const POINT_BUY_MAX = 15

/**
 * What each score costs, indexed by how far it is above 8.
 *
 * Linear to 13 and then not, which is the whole of the mechanic: 14 costs two
 * points and 15 costs two more, so a 15 is paid for by somebody else's 10.
 */
const POINT_COSTS: readonly number[] = [0, 1, 2, 3, 4, 5, 7, 9]

/**
 * What one score costs, or null where point buy does not sell it.
 *
 * Null rather than a clamp: a score outside 8..15 is not an expensive score,
 * it is a score this method cannot produce, and a total that quietly counted
 * it as 9 points would be a budget nobody could reconcile.
 */
export function pointCost(score: number): number | null {
  return POINT_COSTS[score - POINT_BUY_MIN] ?? null
}

/** What a whole set of scores costs. Unsellable scores count as nothing. */
export function pointsSpent(scores: Iterable<number>): number {
  let total = 0
  for (const score of scores) total += pointCost(score) ?? 0
  return total
}

/** One fair six-sided die. */
export function d6(): number {
  return Math.floor(Math.random() * 6) + 1
}

/**
 * Six scores, rolled 4d6 and dropping the lowest die of each.
 *
 * The die is a parameter because a test that cannot say what was rolled can
 * only assert that six numbers came back, and "between 3 and 18" is not a
 * test of dropping the lowest.
 */
export function rollAbilityScores(die: () => number = d6): number[] {
  return Array.from({ length: 6 }, () => rollOne(die))
}

function rollOne(die: () => number): number {
  const dice = [die(), die(), die(), die()].sort((a, b) => a - b)
  return dice.slice(1).reduce((sum, each) => sum + each, 0)
}
