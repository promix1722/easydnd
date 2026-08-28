import { useRef, useState } from 'react'
import type { PointerEvent as ReactPointerEvent, ReactNode } from 'react'

import { Box, Group, NO_SWIPE, Paper, Portal, SimpleGrid, Stack, Text, TOUCH_TARGET } from '@/ui'

import { ABILITY_ORDER } from '@/domain'
import { useT } from '@/lib/i18n'

import { abilityName } from './labels'
import { abilityUnder, DRAG_THRESHOLD, point, TARGET, travelled } from './scoreDrag'
import type { Drag, DragHandlers } from './scoreDrag'

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
 * Two ways to move one, and they are the same operation: drag it, or tap it and
 * tap where it goes. The second is not a fallback -- it is the keyboard's path
 * and the one a screen reader can follow, since every half of this surface is a
 * real `<button>`.
 *
 * **The drag is built on pointer events, not on HTML5 drag-and-drop**, and that
 * is the whole reason this works on a phone. `draggable` + `dragstart` is a
 * mouse protocol: no mobile browser fires it for a finger, so the surface that
 * says "drag a number onto an ability" did nothing at all on the device where
 * the app is actually used at a table. Pointer events are one API for finger,
 * mouse and stylus, so there is now one implementation instead of one that
 * worked and one that could not.
 *
 * Three things it has to do that the native protocol did for free:
 *
 * - **Find the drop target itself**, with `elementFromPoint` over the `TARGET`
 *   attribute. There is no `dragover`.
 * - **Hold the pointer**, with `setPointerCapture`, so the events keep arriving
 *   at the element the gesture started on even once the finger has left it.
 * - **Keep the page still**, with `touch-action: none` on the things you can
 *   pick up. Without it the browser starts scrolling and cancels the drag on
 *   the first vertical millimetre.
 *
 * A press that never travels `DRAG_THRESHOLD` is not a drag at all and falls
 * through to the click, which is what keeps the tap gesture exact.
 *
 * The surface also carries `NO_SWIPE`, and that is separate and still needed:
 * on a phone this sits in `ui/TabDeck`'s carousel, which otherwise takes the
 * pointer down on anything that is not a field and turns the drag into a swipe
 * to the next tab.
 *
 * Dropping onto a taken ability swaps rather than refuses: the two numbers you
 * are looking at are the two you meant, and an ability that had to be emptied
 * first would be a rule about the widget rather than about the character.
 *
 * **Dropping a number anywhere that is not an ability puts it back in the
 * pool.** Undoing a placement is half of dealing six numbers out and it used to
 * have exactly one gesture -- tap the number, tap it again -- which is not the
 * one anybody tries on a surface that says "drag". Dragging it off the grid is,
 * and there is nothing else a number let go over the hint text could sensibly
 * mean. The two taps still work.
 *
 * Both halves are drawn large. A number waiting to be placed used to be a
 * badge -- a few millimetres of target for the one gesture this whole surface
 * exists for -- and the thing it is dragged onto has to be big enough to let
 * go over. `TOUCH_TARGET` is the floor, and it is the usual one for something a
 * thumb has to hit.
 */
export function ScoreAssignment({ values, placed, onPlace, action }: ScoreAssignmentProps) {
  const t = useT()
  // What has been picked up and not yet put down, by a tap. Null is the
  // resting state.
  const [held, setHeld] = useState<number | null>(null)
  // The drag in flight, or null. `over` is what is under the finger right now,
  // and `at` is where to draw the number that is following it.
  const [drag, setDrag] = useState<Drag | null>(null)
  // A drag ends in a `click` on whatever the gesture started on, which would
  // otherwise pick the number straight back up again.
  const dropped = useRef(false)

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

  /** Sending a number back to the pool, from wherever it is. */
  const release = (place: number) => {
    const from = abilityHolding(place)
    if (from === undefined) return
    onPlace({ ...placed, [from]: null })
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

  /** The handlers that make one number draggable, wherever it is drawn. */
  const draggable = (place: number | null): DragHandlers => {
    if (place === null) return {}
    return {
      onPointerDown: (event: ReactPointerEvent<HTMLElement>) => {
        // The primary button only, and never a right-click's context menu.
        if (event.pointerType === 'mouse' && event.button !== 0) return
        event.currentTarget.setPointerCapture?.(event.pointerId)
        setDrag({ place, from: point(event), at: point(event), over: null, moved: false })
      },
      // Read from the render the handler was made in rather than through an
      // updater. The updater is the obvious place for `drag` and the wrong one:
      // it must be pure, `put` is not, and StrictMode calls it twice to say so.
      // `from` never changes within a gesture, so a handler one render stale
      // still measures the travel correctly.
      onPointerMove: (event: ReactPointerEvent<HTMLElement>) => {
        if (drag === null || drag.place !== place) return
        const at = point(event)
        const moved = drag.moved || travelled(drag.from, at) >= DRAG_THRESHOLD
        setDrag({ ...drag, at, moved, over: moved ? abilityUnder(at) : null })
      },
      onPointerUp: () => {
        if (drag !== null && drag.moved) {
          // Dropped on an ability, it goes there. Dropped anywhere else, it
          // goes back to the pool -- which is the gesture for taking a number
          // off a character, and the one everybody tries first. Refusing it
          // and snapping back would leave the two taps as the only way, on a
          // surface whose whole instruction is "drag".
          if (drag.over !== null) put(drag.over, place)
          else release(place)
          dropped.current = true
        }
        setDrag(null)
      },
      onPointerCancel: () => setDrag(null),
    }
  }

  /** True once, for the click that ends a drag. */
  const wasDrop = () => {
    if (!dropped.current) return false
    dropped.current = false
    return true
  }

  const pool = values.flatMap((value, place) =>
    abilityHolding(place) === undefined ? [{ value, place }] : [],
  )

  return (
    // The mark goes on the whole surface rather than on each control: the drift
    // that has to be tolerated happens between a press and its release, and the
    // release is not always over the thing that was pressed.
    <Stack gap="sm" {...{ [NO_SWIPE]: true }}>
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        {/* The heading on a line of its own. Beside the numbers it read as one
            of them -- a seventh thing in a row of six -- and on a narrow screen
            it was the word the wrap broke around. */}
        <Stack gap={6}>
          <Text size="xs" c="dimmed" tt="uppercase">
            {t('scores.toPlace')}
          </Text>
          {/* Three across on a phone, six in a row on a desktop -- a grid
              rather than a wrap, because what wrapping chose was whatever fit,
              and what fitted was four numbers and half of "Roll again". Three
              and three leaves the row's other half to the control that shares
              it. */}
          <SimpleGrid cols={{ base: 3, sm: 6 }} spacing={6} w="fit-content">
            {pool.map(({ value, place }) => (
              <Chip
                key={place}
                label={String(value)}
                held={held === place || drag?.place === place}
                drag={draggable(place)}
                onClick={() => {
                  if (wasDrop()) return
                  setHeld(held === place ? null : place)
                }}
              />
            ))}
          </SimpleGrid>
          {pool.length === 0 && (
            <Text size="sm" c="dimmed">
              {t('scores.allPlaced')}
            </Text>
          )}
        </Stack>
        {/* Never squeezed. It is the only way to ask for six more numbers, and
            a `nowrap` row will shrink a button to its ellipsis before it gives
            up a pixel of what sits beside it. */}
        {action !== undefined && <Box style={{ flexShrink: 0 }}>{action}</Box>}
      </Group>

      <SimpleGrid cols={{ base: 2, sm: 3 }} spacing="sm">
        {ABILITY_ORDER.map((ability) => {
          const place = placed[ability] ?? null
          const value = place === null ? undefined : values[place]
          const lit = (held !== null && held === place) || drag?.over === ability
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
              {...{ [TARGET]: ability }}
              {...draggable(place)}
              onClick={() => {
                if (wasDrop()) return
                if (held === null || held === place) take(ability)
                else put(ability, held)
              }}
              style={{
                cursor: 'pointer',
                textAlign: 'left',
                width: '100%',
                minHeight: TOUCH_TARGET * 1.5,
                background: 'transparent',
                borderStyle: value === undefined ? 'dashed' : 'solid',
                ...(lit ? { borderColor: 'var(--mantine-primary-color-filled)' } : {}),
                // The browser must not pan the page while a finger is dragging
                // a number across it: a scroll cancels the pointer stream.
                touchAction: 'none',
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

      {/* The number itself, following the finger.

          In a `Portal`, and that is the whole reason it is visible at all: a
          `position: fixed` element is positioned against the nearest
          *transformed* ancestor rather than the viewport, and on a phone this
          surface sits inside a carousel whose track is translated on every
          frame. Drawn in place, the ghost was laid out against that track and
          clipped by its overflow -- the drag worked and nothing moved.

          `pointer-events: none` is load-bearing too: this sits under the
          pointer, and `elementFromPoint` would otherwise find it instead of the
          ability underneath. */}
      {drag?.moved === true && (
        <Portal>
          <Box
            aria-hidden
            style={{
              position: 'fixed',
              left: drag.at.x,
              top: drag.at.y,
              transform: 'translate(-50%, -50%)',
              pointerEvents: 'none',
              zIndex: 400,
              minWidth: TOUCH_TARGET,
              minHeight: TOUCH_TARGET,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              borderRadius: 'var(--mantine-radius-md)',
              background: 'var(--mantine-primary-color-filled)',
              color: 'var(--mantine-primary-color-contrast)',
              fontWeight: 600,
            }}
          >
            {values[drag.place]}
          </Box>
        </Portal>
      )}
    </Stack>
  )
}

/** One number waiting to be placed. */
function Chip({
  label,
  held,
  drag,
  onClick,
}: {
  label: string
  held: boolean
  drag: DragHandlers
  onClick: () => void
}) {
  return (
    <Paper
      withBorder
      radius="md"
      component="button"
      type="button"
      onClick={onClick}
      data-held={held ? 'true' : undefined}
      {...drag}
      style={{
        cursor: 'grab',
        touchAction: 'none',
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
