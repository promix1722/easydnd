import { useCharacterPolicy } from '@/lib/api/catalogScope'
import { useT } from '@/lib/i18n'
import { Group, SimpleGrid, Stack, Text } from '@/ui'

import { ScoreStepper } from './ScoreStepper'

import { ABILITY_ORDER } from '@/domain'

export interface PointBuyProps {
  scores: Record<string, number>
  onChange: (scores: Record<string, number>) => void
}

/**
 * The 27-point buy, priced as the rules price it.
 *
 * Every score starts at 8 and costs nothing there. Up to 13 a point buys a
 * point; 14 costs two and 15 costs two more, which is the whole of the
 * mechanic -- a 15 is paid for by somebody else's 10 -- and it is why this
 * cannot be six spinners with a total underneath. The step buttons are the
 * price list made operable: a raise you cannot afford is not offered, so the
 * budget is something the screen enforces rather than something it complains
 * about afterwards.
 *
 * Points may be left unspent. A player who wants an even spread of 13s has
 * spent 25 and is finished, and a form that refused to let them past would be
 * inventing a rule to protect them from arithmetic they can see.
 */
export function PointBuy({ scores, onChange }: PointBuyProps) {
  const t = useT()
  const policy = useCharacterPolicy()
  const minimum = Math.min(...Object.keys(policy.pointCosts).map(Number))
  const maximum = Math.max(...Object.keys(policy.pointCosts).map(Number))
  const pointCost = (score: number) => policy.pointCosts[String(score)] ?? null
  const spent = ABILITY_ORDER.reduce((total, ability) => total + (pointCost(scores[ability] ?? minimum) ?? 0), 0)
  const left = policy.pointBuyBudget - spent

  const step = (ability: string, by: number) => {
    const next = (scores[ability] ?? minimum) + by
    onChange({ ...scores, [ability]: next })
  }

  /** What moving this score by one costs, or null where it cannot move. */
  const priceOf = (ability: string, by: number): number | null => {
    const from = scores[ability] ?? minimum
    const to = from + by
    if (to < minimum || to > maximum || pointCost(to) === null) return null
    const cost = (pointCost(to) ?? 0) - (pointCost(from) ?? 0)
    return cost > left ? null : cost
  }

  return (
    <Stack gap="sm">
      <Group justify="space-between" align="center">
        <Text size="sm" fw={600}>
          {t('pointBuy.left', { count: left, budget: policy.pointBuyBudget })}
        </Text>
        <Text size="xs" c="dimmed">
          {t('pointBuy.prices', { prices: Object.entries(policy.pointCosts).map(([score, cost]) => `${score}: ${cost}`).join(' · ') })}
        </Text>
      </Group>

      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
        {ABILITY_ORDER.map((ability) => {
          const score = scores[ability] ?? minimum
          return (
            <ScoreStepper
              key={ability}
              ability={ability}
              value={score}
              note={t('pointBuy.spent', { count: pointCost(score) ?? 0 })}
              canLower={priceOf(ability, -1) !== null}
              canRaise={priceOf(ability, 1) !== null}
              onStep={(by) => step(ability, by)}
              min={minimum}
              max={maximum}
            />
          )
        })}
      </SimpleGrid>

      <Text size="xs" c="dimmed">
        {t('pointBuy.hint', { min: minimum, max: maximum })}
      </Text>
    </Stack>
  )
}
