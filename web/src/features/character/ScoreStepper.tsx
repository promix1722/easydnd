import { ActionIcon, Group, NumberInput, Paper, Stack, Text } from '@/ui'

import { abilityName } from '@/domain'

export interface ScoreStepperProps {
  ability: string
  /**
   * What the stepper shows.
   *
   * A string, and possibly an empty one, where the number may be typed over: a
   * field that refills itself the moment it is cleared cannot be typed in.
   */
  value: number | string
  /** A second line under the ability's name -- point buy prints its price. */
  note?: string
  canLower: boolean
  canRaise: boolean
  onStep: (by: number) => void
  /**
   * Present where the number itself may be typed.
   *
   * Point buy leaves it out, and that is the mechanic rather than an omission:
   * a score there is bought, and typing 15 into it would be taking it. Manual
   * entry passes it, because ten clicks to reach a 20 is not an escape hatch.
   */
  onValueChange?: (value: number | string) => void
  min: number
  max: number
  error?: string
}

/**
 * One ability, and the two buttons that move it.
 *
 * It exists because two editors draw the same row for two different reasons.
 * Point buy's steppers are a price list made operable -- a raise you cannot
 * afford is not offered -- and manual entry's are the same control bounded by
 * what a score can be at all. Only the middle differs, and only in whether it
 * can be typed in.
 *
 * The buttons carry the ability's name in their labels, because "+" is not
 * something anybody can be told to press.
 */
export function ScoreStepper({
  ability,
  value,
  note,
  canLower,
  canRaise,
  onStep,
  onValueChange,
  min,
  max,
  error,
}: ScoreStepperProps) {
  const name = abilityName(ability)

  return (
    <Paper withBorder p="sm" radius="md">
      <Stack gap={4}>
        <Group justify="space-between" wrap="nowrap">
          <div>
            <Text size="xs" c="dimmed" tt="uppercase">
              {name}
            </Text>
            {note !== undefined && (
              <Text size="xs" c="dimmed">
                {note}
              </Text>
            )}
          </div>
          <Group gap={6} wrap="nowrap">
            <ActionIcon
              variant="default"
              aria-label={`Lower ${name}`}
              disabled={!canLower}
              onClick={() => onStep(-1)}
            >
              -
            </ActionIcon>
            {onValueChange === undefined ? (
              <Text size="xl" fw={600} w={28} ta="center">
                {value}
              </Text>
            ) : (
              <NumberInput
                aria-label={name}
                hideControls
                min={min}
                max={max}
                w={64}
                styles={{ input: { textAlign: 'center', fontWeight: 600 } }}
                value={value}
                // The message goes under the whole row rather than under the
                // field: a 64px input is not somewhere a sentence fits.
                error={error !== undefined}
                onChange={onValueChange}
              />
            )}
            <ActionIcon
              variant="default"
              aria-label={`Raise ${name}`}
              disabled={!canRaise}
              onClick={() => onStep(1)}
            >
              +
            </ActionIcon>
          </Group>
        </Group>
        {error !== undefined && (
          <Text size="xs" c="red">
            {error}
          </Text>
        )}
      </Stack>
    </Paper>
  )
}
