import { useState } from 'react'
import type { DragEvent, ReactNode } from 'react'

import { Group, Paper, SimpleGrid, Stack, Text, TOUCH_TARGET } from '@/ui'

import { ABILITY_ORDER } from '@/domain'
import { useT } from '@/lib/i18n'

import { abilityName } from './labels'

/** Which value sits on which ability: an ability, to a place in the pool. */
export type Placement = Record<string, number | null>

export interface ScoreAssignmentProps {
  /** The six numbers to be placed, in the order they were produced. */
  values: readonly number[]
  placed: Placement
  onPlace: (placed: Placement) => void
  /** Where a method puts a control of its own -- Rolled puts its dice here. */
  action?: ReactNode
}

/**
 * Six numbers, and the six abilities they have to be dealt out to.
 *
 * The array and the dice both produce a *set* of numbers and leave the
 * interesting decision -- which ability gets the 15 -- to the player, so both
 * are this: a pool you take from and six places to put things. Neither method
 * lets a number be typed, because in neither method is the number yours to
 * choose; what is yours is where it goes.
 *
 * Two ways to move one, and they are the same operation. Dragging is the
 * obvious one on a mouse and does not exist on a phone, so a value can also be
 * picked up with a tap or the keyboard and put down with a second one -- which
 * is what makes this operable at 390px, and what makes it operable at all
 * without a pointing device.
 *
 * Dropping onto a taken ability swaps rather than refuses: the two numbers you
 * are looking at are the two you meant, and an ability that had to be emptied
 * first would be a rule about the widget rather than about the character. A
 * number picked up and put back down where it came from returns to the pool,
 * which is the only way to undo a placement and is the one you would try.
 *
 * Both halves are drawn large. A number waiting to be placed used to be a
 * badge -- a few millimetres of target for the one gesture this whole surface
 * exists for -- and the thing it is dragged onto has to be big enough to let
 * go over. `TARGET` is the floor, and it is the usual one for something a
 * thumb has to hit.
 */
export function ScoreAssignment({ values, placed, onPlace, action }: ScoreAssignmentProps) {
  const t = useT()
  // What has been picked up and not yet put down. Null is the resting state,
  // and the only state a mouse ever sees.
  const [held, setHeld] = useState<number | null>(null)

  const abilityHolding = (place: number): string | undefined =>
    ABILITY_ORDER.find((ability) => placed[ability] === place)

  const put = (ability: string, place: number) => {
    const next: Placement = { ...placed }
    const from = abilityHolding(place)
    // Coming from another ability, the two trade; coming from the pool, the
    // number that was here goes back to it.
    if (from !== undefined) next[from] = placed[ability] ?? null
    next[ability] = place
    onPlace(next)
    setHeld(null)
  }

  /** Picking one up, or -- on the second press -- putting it back. */
  const take = (ability: string) => {
    const place = placed[ability]
    if (place === null || place === undefined) return
    if (held === place) {
      onPlace({ ...placed, [ability]: null })
      setHeld(null)
      return
    }
    setHeld(place)
  }

  const pool = values.flatMap((value, place) =>
    abilityHolding(place) === undefined ? [{ value, place }] : [],
  )

  return (
    <Stack gap="sm">
      <Group justify="space-between" align="center" wrap="nowrap">
        <Group gap={6} align="center" wrap="wrap">
          <Text size="xs" c="dimmed" tt="uppercase">
            {t('scores.toPlace')}
          </Text>
          {pool.map(({ value, place }) => (
            <Chip
              key={place}
              label={String(value)}
              held={held === place}
              onDragStart={dragging(place)}
              onClick={() => setHeld(held === place ? null : place)}
            />
          ))}
          {pool.length === 0 && (
            <Text size="sm" c="dimmed">
              {t('scores.allPlaced')}
            </Text>
          )}
        </Group>
        {action}
      </Group>

      <SimpleGrid cols={{ base: 2, sm: 3 }} spacing="sm">
        {ABILITY_ORDER.map((ability) => {
          const place = placed[ability] ?? null
          const value = place === null ? undefined : values[place]
          return (
            <Paper
              key={ability}
              withBorder
              p="md"
              radius="md"
              component="button"
              type="button"
              // The whole square is the target: a drop zone the size of the
              // number in it would be a game of its own.
              onDragOver={(event: DragEvent<HTMLElement>) => {
                event.preventDefault()
              }}
              onDrop={(event: DragEvent<HTMLElement>) => {
                event.preventDefault()
                // Anything at all can be dragged onto a page, and `Number('')`
                // is 0 -- so without checking that the payload names one of
                // the six places, dropping a selection of text here would deal
                // out whichever number happens to be first.
                const place = Number(event.dataTransfer.getData('text/plain'))
                if (Number.isInteger(place) && place >= 0 && place < values.length) {
                  put(ability, place)
                }
              }}
              onClick={() => {
                if (held === null || held === place) take(ability)
                else put(ability, held)
              }}
              draggable={value !== undefined}
              onDragStart={place === null ? undefined : dragging(place)}
              data-held={held !== null && held === place ? 'true' : undefined}
              style={{
                cursor: 'pointer',
                textAlign: 'left',
                width: '100%',
                minHeight: TOUCH_TARGET * 1.5,
                background: 'transparent',
                borderStyle: value === undefined ? 'dashed' : 'solid',
                ...(held !== null && held === place
                  ? { borderColor: 'var(--mantine-primary-color-filled)' }
                  : {}),
              }}
            >
              <Text size="xs" c="dimmed" tt="uppercase">
                {abilityName(t, ability)}
              </Text>
              {value === undefined ? (
                <Text size="xl" fw={600} c="dimmed">
                  --
                </Text>
              ) : (
                <Text size="xl" fw={600}>
                  {value}
                </Text>
              )}
            </Paper>
          )
        })}
      </SimpleGrid>

      <Text size="xs" c="dimmed">
        {t('scores.dragHint')}
      </Text>
    </Stack>
  )
}

function dragging(place: number) {
  return (event: DragEvent<HTMLElement>) => {
    event.dataTransfer.setData('text/plain', String(place))
    event.dataTransfer.effectAllowed = 'move'
  }
}

/** One number waiting to be placed. */
function Chip({
  label,
  held,
  onClick,
  onDragStart,
}: {
  label: string
  held: boolean
  onClick: () => void
  onDragStart: (event: DragEvent<HTMLElement>) => void
}) {
  return (
    <Paper
      withBorder
      radius="md"
      component="button"
      type="button"
      draggable
      onDragStart={onDragStart}
      onClick={onClick}
      data-held={held ? 'true' : undefined}
      style={{
        cursor: 'grab',
        minWidth: TOUCH_TARGET,
        minHeight: TOUCH_TARGET,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: held ? 'var(--mantine-primary-color-filled)' : 'transparent',
        ...(held ? { borderColor: 'var(--mantine-primary-color-filled)' } : {}),
      }}
    >
      <Text
        size="xl"
        fw={600}
        {...(held ? { c: 'var(--mantine-primary-color-contrast)' } : {})}
      >
        {label}
      </Text>
    </Paper>
  )
}
