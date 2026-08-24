import { useState } from 'react'

import type { ApiFieldError, Change } from '@/lib/api'
import { Button, Card, Group, NumberInput, Select, SimpleGrid, Stack, Text } from '@/ui'

import { ABILITY_ORDER, abilityName } from '@/domain'

/**
 * The standard array, as the SRD prints it.
 *
 * Positional against ABILITY_ORDER: the first number goes to Strength because
 * that is the first input on the screen, not because the array has anything to
 * say about which ability wants a 15.
 */
const STANDARD_ARRAY = [15, 14, 13, 12, 10, 8]

const METHODS = [
  { value: 'standard-array', label: 'Standard array' },
  { value: 'point-buy', label: 'Point buy' },
  { value: 'rolled', label: 'Rolled' },
  { value: 'manual', label: 'Manual' },
]

export type Scores = Record<string, number>

export interface AbilityScoresFormProps {
  /** The scores as they stand, so changing them starts from what they are. */
  scores?: Scores
  method?: string
  pending: boolean
  /** The server's per-field complaints, pointed at the input that caused one. */
  fields?: readonly ApiFieldError[]
  submitLabel: string
  onSubmit: (changes: Change[]) => void
}

/**
 * The six ability scores, and how they were arrived at.
 *
 * The numbers entered here are the *base* array, before any racial bonus. The
 * server applies those on every projection, which is what lets a player change
 * their mind about a race without re-entering six numbers -- so this form must
 * not "helpfully" pre-add them.
 *
 * The scores are one selection and are written as one entry, which is the
 * whole reason this is a form on the abilities tab rather than six fields on a
 * create page. They used to travel with creation, where they had no entry of
 * their own and so nothing to point at and change.
 *
 * It emits the addressed changes rather than a shape of its own: the six
 * scores answer a prompt that names no picks, so what settles them is what the
 * log stores -- `abilities.str set 15` -- and inventing a wrapper here would
 * be a second spelling of the same thing.
 */
export function AbilityScoresForm({
  scores,
  method = 'standard-array',
  pending,
  fields = [],
  submitLabel,
  onSubmit,
}: AbilityScoresFormProps) {
  const [chosen, setChosen] = useState<Scores>(() => scores ?? standardArrayScores())
  const [how, setHow] = useState(method)

  const errorFor = (field: string): string | undefined =>
    fields.find((f) => f.field === field)?.message

  const submit = () => {
    onSubmit([
      { path: 'abilities.method', op: 'set', value: { kind: 'slug', slug: how } },
      ...ABILITY_ORDER.map<Change>((ability) => ({
        path: `abilities.${ability}`,
        op: 'set',
        value: { kind: 'int', int: chosen[ability] ?? 10 },
      })),
    ])
  }

  return (
    <Card withBorder padding="lg" radius="md">
      <Stack gap="md">
        <Text fw={600}>Set the six scores</Text>

        <Select
          label="How were the scores generated?"
          description="Recorded so that coming back here reopens the right editor."
          data={METHODS}
          value={how}
          allowDeselect={false}
          onChange={(value) => {
            if (value === null) return
            setHow(value)
            if (value === 'standard-array') setChosen(standardArrayScores())
          }}
        />

        <div>
          <Text size="xs" c="dimmed" mb="xs">
            The base array. Racial bonuses are added by the rules, not here.
          </Text>
          <SimpleGrid cols={{ base: 2, sm: 3 }} spacing="sm">
            {ABILITY_ORDER.map((ability) => (
              <NumberInput
                key={ability}
                label={abilityName(ability)}
                min={1}
                max={30}
                value={chosen[ability] ?? 10}
                error={errorFor(`abilities.${ability}`)}
                onChange={(value) =>
                  setChosen((current) => ({
                    ...current,
                    [ability]: typeof value === 'number' ? value : Number(value) || 10,
                  }))
                }
              />
            ))}
          </SimpleGrid>
        </div>

        <Group>
          <Button onClick={submit} loading={pending}>
            {submitLabel}
          </Button>
        </Group>
      </Stack>
    </Card>
  )
}

function standardArrayScores(): Scores {
  const out: Scores = {}
  ABILITY_ORDER.forEach((ability, i) => {
    out[ability] = STANDARD_ARRAY[i] ?? 10
  })
  return out
}
