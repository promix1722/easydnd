import { ActionIcon, Group, Paper, SimpleGrid, Stack, Text } from '@/ui'

import {
  ABILITY_ORDER,
  POINT_BUY_BUDGET,
  POINT_BUY_MAX,
  POINT_BUY_MIN,
  abilityName,
  pointCost,
  pointsSpent,
} from '@/domain'

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
  const spent = pointsSpent(ABILITY_ORDER.map((ability) => scores[ability] ?? POINT_BUY_MIN))
  const left = POINT_BUY_BUDGET - spent

  const step = (ability: string, by: number) => {
    const next = (scores[ability] ?? POINT_BUY_MIN) + by
    onChange({ ...scores, [ability]: next })
  }

  /** What moving this score by one costs, or null where it cannot move. */
  const priceOf = (ability: string, by: number): number | null => {
    const from = scores[ability] ?? POINT_BUY_MIN
    const to = from + by
    if (to < POINT_BUY_MIN || to > POINT_BUY_MAX) return null
    const cost = (pointCost(to) ?? 0) - (pointCost(from) ?? 0)
    return cost > left ? null : cost
  }

  return (
    <Stack gap="sm">
      <Group justify="space-between" align="center">
        <Text size="sm" fw={600}>
          {left} {left === 1 ? 'point' : 'points'} left of {POINT_BUY_BUDGET}
        </Text>
        <Text size="xs" c="dimmed">
          9-13 cost one each · 14 costs two · 15 costs two more
        </Text>
      </Group>

      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
        {ABILITY_ORDER.map((ability) => {
          const score = scores[ability] ?? POINT_BUY_MIN
          return (
            <Paper key={ability} withBorder p="sm" radius="md">
              <Group justify="space-between" wrap="nowrap">
                <div>
                  <Text size="xs" c="dimmed" tt="uppercase">
                    {abilityName(ability)}
                  </Text>
                  <Text size="xs" c="dimmed">
                    {pointCost(score) ?? 0} spent
                  </Text>
                </div>
                <Group gap={6} wrap="nowrap">
                  <ActionIcon
                    variant="default"
                    aria-label={`Lower ${abilityName(ability)}`}
                    disabled={priceOf(ability, -1) === null}
                    onClick={() => step(ability, -1)}
                  >
                    -
                  </ActionIcon>
                  <Text size="xl" fw={600} w={28} ta="center">
                    {score}
                  </Text>
                  <ActionIcon
                    variant="default"
                    aria-label={`Raise ${abilityName(ability)}`}
                    disabled={priceOf(ability, 1) === null}
                    onClick={() => step(ability, 1)}
                  >
                    +
                  </ActionIcon>
                </Group>
              </Group>
            </Paper>
          )
        })}
      </SimpleGrid>

      <Text size="xs" c="dimmed">
        Nothing starts below 8 or is bought above 15. Racial bonuses are added by the rules, not
        here, so a 15 and a +2 is a 17 on the sheet.
      </Text>
    </Stack>
  )
}
