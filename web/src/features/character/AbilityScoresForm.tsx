import { useState } from 'react'

import type { ApiFieldError, Change } from '@/lib/api'
import { Button, Group, Select, SimpleGrid, Stack, Text } from '@/ui'

import { PointBuy } from './PointBuy'
import { ScoreAssignment } from './ScoreAssignment'
import type { Placement } from './ScoreAssignment'
import { ScoreStepper } from './ScoreStepper'

import {
  ABILITY_ORDER,
  POINT_BUY_MAX,
  POINT_BUY_MIN,
  STANDARD_ARRAY,
  rollAbilityScores,
} from '@/domain'

const METHODS = [
  { value: 'standard-array', label: 'Standard array' },
  { value: 'point-buy', label: 'Point buy' },
  { value: 'rolled', label: 'Rolled' },
  { value: 'manual', label: 'Manual' },
]

/** The two methods that deal out a set of numbers rather than take them. */
function dealsOut(method: string): boolean {
  return method === 'standard-array' || method === 'rolled'
}

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
 * Four methods, and the method decides what there is to do, because in the
 * rules it decides what is actually being chosen:
 *
 *   - **standard array** deals out six printed numbers, and the decision is
 *     which ability gets which;
 *   - **rolled** does the same with 4d6-drop-lowest, plus the one thing dice
 *     allow that a printed array does not, which is rolling again;
 *   - **point buy** is not a set of numbers at all but a budget, so it is
 *     priced steppers and a running total;
 *   - **manual** is the escape hatch -- an imported character, a DM's house
 *     rule -- and is the only one where a number is typed.
 *
 * The first three do not let a score be typed over, and that is the point
 * rather than an omission: in none of them is the number yours to pick. What
 * is yours is where it goes, or what you spend.
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
  const [how, setHow] = useState(method)
  // The pool the dealing methods deal from, and where each of its numbers has
  // been put. Scores that are already stored arrive placed: they were dealt
  // out once already, and the entry records where they landed.
  const [values, setValues] = useState<number[]>(() => dealt(method, scores))
  const [placed, setPlaced] = useState<Placement>(() => (scores ? inOrder() : nothingPlaced()))
  // Point buy holds numbers, because its steppers can only produce numbers.
  // Manual holds whatever has been typed, including nothing at all: a field
  // that refills itself with a 10 the moment it is cleared cannot be typed in.
  const [bought, setBought] = useState<Scores>(() => boughtFrom(method, scores))
  const [written, setWritten] = useState<Written>(() => ({ ...(scores ?? allAt(10)) }))

  const chosen = (): Scores => {
    if (dealsOut(how)) {
      return Object.fromEntries(
        ABILITY_ORDER.map((ability) => [ability, values[placed[ability] ?? -1] ?? 0]),
      )
    }
    if (how === 'point-buy') {
      return Object.fromEntries(
        ABILITY_ORDER.map((ability) => [ability, bought[ability] ?? POINT_BUY_MIN]),
      )
    }
    return Object.fromEntries(ABILITY_ORDER.map((ability) => [ability, written10(written, ability)]))
  }

  const ready = !dealsOut(how) || ABILITY_ORDER.every((ability) => placed[ability] !== null)

  const change = (next: string) => {
    setHow(next)
    // Each method starts from its own beginning. Carrying the last one's
    // numbers over would produce a point-buy character with a 17 in it, or an
    // array that is not the array.
    if (next === 'standard-array') {
      setValues([...STANDARD_ARRAY])
      setPlaced(nothingPlaced())
    } else if (next === 'rolled') {
      setValues(rollAbilityScores())
      setPlaced(nothingPlaced())
    } else if (next === 'point-buy') {
      setBought(allAt(POINT_BUY_MIN))
    } else {
      // Only where the method being left actually produced six scores. An
      // unplaced array has none, and `chosen` reports a place nobody has taken
      // as a 0 -- so this used to open manual entry on six zeros, below its
      // own minimum, which then saved as six 10s. Ten is where manual starts.
      setWritten(carried(chosen()))
    }
  }

  const submit = () => {
    const scored = chosen()
    onSubmit([
      { path: 'abilities.method', op: 'set', value: { kind: 'slug', slug: how } },
      ...ABILITY_ORDER.map<Change>((ability) => ({
        path: `abilities.${ability}`,
        op: 'set',
        value: { kind: 'int', int: scored[ability] ?? 10 },
      })),
    ])
  }

  // The server names a rejected score by its position in the entry --
  // `events[0].changes[3].value` -- because that is where it found it. The
  // form's own order is the same one it submits in, so the position is the
  // index of the ability plus one for the method change that leads.
  const errorFor = (at: number): string | undefined =>
    fields.find((f) => f.field.endsWith(`.changes[${at + 1}].value`))?.message

  return (
    <Stack gap="md">
      <Select
        label="How were the scores generated?"
        description="Recorded so that coming back here reopens the right editor."
        data={METHODS}
        value={how}
        allowDeselect={false}
        onChange={(value) => {
          if (value !== null) change(value)
        }}
      />

      {dealsOut(how) && (
        <ScoreAssignment
          values={values}
          placed={placed}
          onPlace={setPlaced}
          {...(how === 'rolled'
            ? {
                action: (
                  <Button
                    variant="light"
                    onClick={() => {
                      setValues(rollAbilityScores())
                      setPlaced(nothingPlaced())
                    }}
                  >
                    Roll again
                  </Button>
                ),
              }
            : {})}
        />
      )}

      {how === 'point-buy' && <PointBuy scores={bought} onChange={setBought} />}

      {how === 'manual' && (
        <div>
          <Text size="xs" c="dimmed" mb="xs">
            Anything from 1 to 30. Racial bonuses are added by the rules, not here.
          </Text>
          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
            {ABILITY_ORDER.map((ability, at) => {
              const score = written10(written, ability)
              return (
                <ScoreStepper
                  key={ability}
                  ability={ability}
                  value={written[ability] ?? ''}
                  canLower={score > MANUAL_MIN}
                  canRaise={score < MANUAL_MAX}
                  onStep={(by) =>
                    setWritten((current) => ({ ...current, [ability]: score + by }))
                  }
                  onValueChange={(value) =>
                    setWritten((current) => ({ ...current, [ability]: value }))
                  }
                  min={MANUAL_MIN}
                  max={MANUAL_MAX}
                  {...maybeError(errorFor(at))}
                />
              )
            })}
          </SimpleGrid>
        </div>
      )}

      <Group>
        <Button onClick={submit} loading={pending} disabled={!ready}>
          {ready ? submitLabel : 'Place all six'}
        </Button>
      </Group>
    </Stack>
  )
}

/** The numbers a dealing method starts with, in the order they were produced. */
function dealt(method: string, scores?: Scores): number[] {
  if (scores !== undefined) return ABILITY_ORDER.map((ability) => scores[ability] ?? 10)
  return method === 'rolled' ? rollAbilityScores() : [...STANDARD_ARRAY]
}

/** Each ability holding the number that was stored against it. */
function inOrder(): Placement {
  return Object.fromEntries(ABILITY_ORDER.map((ability, place) => [ability, place]))
}

function nothingPlaced(): Placement {
  return Object.fromEntries(ABILITY_ORDER.map((ability) => [ability, null]))
}

function allAt(score: number): Scores {
  return Object.fromEntries(ABILITY_ORDER.map((ability) => [ability, score]))
}

/** What a typed score may be at all. The server bounds it identically. */
const MANUAL_MIN = 1
const MANUAL_MAX = 30

/** What is in the manual fields, which is whatever has been typed there. */
type Written = Record<string, number | string>

/**
 * The scores a method being left hands to manual entry, where it has any.
 *
 * Nothing, where any of the six is not a score -- an array with places still
 * empty reports those as zeros, and six zeros is not a set of numbers somebody
 * was part-way through typing. Manual starts at ten in that case, which is
 * where it starts from nothing.
 */
function carried(scores: Scores): Scores {
  const usable = ABILITY_ORDER.every((ability) => {
    const score = scores[ability] ?? 0
    return score >= MANUAL_MIN && score <= MANUAL_MAX
  })
  return usable ? { ...scores } : allAt(10)
}

function maybeError(error: string | undefined): { error?: string } {
  return error === undefined ? {} : { error }
}

/** One typed field as a score. A field left empty is an average one. */
function written10(written: Written, ability: string): number {
  const value = written[ability]
  const score = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(score) && score > 0 ? score : 10
}

/**
 * What point buy starts from.
 *
 * It will not start from stored scores it could not have bought -- a rolled 17
 * has no price -- so it starts from six 8s, which is where point buy starts
 * anyway.
 */
function boughtFrom(method: string, scores?: Scores): Scores {
  if (scores === undefined || method !== 'point-buy') return allAt(POINT_BUY_MIN)
  const buyable = ABILITY_ORDER.every((ability) => {
    const score = scores[ability] ?? 0
    return score >= POINT_BUY_MIN && score <= POINT_BUY_MAX
  })
  return buyable ? { ...scores } : allAt(POINT_BUY_MIN)
}
