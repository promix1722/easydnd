import { STANDARD_ARRAY, POINT_BUY_BUDGET, pointCost } from '@/domain'
export interface BuildPolicy {
  minScore: number
  maxScore: number
  maxLevel: number
  standardArray: number[]
  pointBuyBudget: number
  pointCosts: Record<string, number>
}
export const DEFAULT_BUILD_POLICY: BuildPolicy = {
  minScore: 1,
  maxScore: 30,
  maxLevel: 20,
  standardArray: [...STANDARD_ARRAY],
  pointBuyBudget: POINT_BUY_BUDGET,
  pointCosts: Object.fromEntries(Array.from({ length: 8 }, (_, i) => [i + 8, pointCost(i + 8)!])),
}
