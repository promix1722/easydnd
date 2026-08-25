import { describe, expect, it } from 'vitest'

import { POINT_BUY_BUDGET, STANDARD_ARRAY, pointCost, pointsSpent, rollAbilityScores } from './index'

describe('point buy', () => {
  it('prices the last two steps as the rules do', () => {
    // Linear to 13, and then not: that is the whole of the mechanic.
    expect([8, 9, 10, 11, 12, 13].map(pointCost)).toEqual([0, 1, 2, 3, 4, 5])
    expect(pointCost(14)).toBe(7)
    expect(pointCost(15)).toBe(9)
  })

  it('sells nothing below 8 or above 15', () => {
    // Null rather than a clamp: a 17 is not an expensive score, it is one this
    // method cannot produce, and counting it as 9 points would be a budget
    // nobody could reconcile.
    expect(pointCost(7)).toBeNull()
    expect(pointCost(16)).toBeNull()
  })

  it('totals a set of scores against the budget', () => {
    // The canonical spread: 15 14 13 12 10 8 costs exactly 27.
    expect(pointsSpent([15, 14, 13, 12, 10, 8])).toBe(POINT_BUY_BUDGET)
    // Six 13s is 30 and cannot be bought; six 12s is 24 and leaves three over.
    expect(pointsSpent([12, 12, 12, 12, 12, 12])).toBe(24)
  })

  it('cannot buy the standard array twice over', () => {
    expect(pointsSpent(STANDARD_ARRAY)).toBe(POINT_BUY_BUDGET)
  })
})

describe('rolling', () => {
  it('drops the lowest of four dice', () => {
    const dice = [1, 6, 6, 6]
    let next = 0
    const die = () => dice[next++ % dice.length] ?? 1

    // 1+6+6+6 with the 1 dropped is 18, six times over.
    expect(rollAbilityScores(die)).toEqual([18, 18, 18, 18, 18, 18])
  })

  it('rolls six scores in the range four dice can produce', () => {
    for (const score of rollAbilityScores()) {
      expect(score).toBeGreaterThanOrEqual(3)
      expect(score).toBeLessThanOrEqual(18)
    }
    expect(rollAbilityScores()).toHaveLength(6)
  })
})
