import type { Sheet } from '@/lib/api'
import { Card, SimpleGrid, Stack, Text } from '@/ui'

import { signed, titleCase } from '@/domain'

/**
 * The two headline rows under the ability cards, six to a line.
 *
 * The abilities come first because they are what the rest is derived from,
 * and the two rows below them are read in the order a table asks for them.
 * The first is **the body's state**: what it can take, what it rolls back
 * with, and the three numbers every attack and check passes through. The
 * second is **what it can do at range**: the spellcasting numbers, then what
 * it notices, how far it moves and how far it sees.
 *
 * Hit points, temporary hit points and Hit Dice sit together at the head of
 * the first row because they are one subject -- the pool, the shield on top of
 * it, and the dice that refill it -- and reading them apart means reading the
 * same question three times. Hit Dice used to live in "Resources and gear"
 * beside the backpack, which is the wrong shelf: it is a fact about the body,
 * not about the kit.
 *
 * Six columns is the width of the ability row above, so every sheet is three
 * lines of six and a reader looking for a number finds it in the same place on
 * every character. That is what the spell cards are doing on a barbarian: they
 * read `n/a` rather than vanishing, because a row that changes length between
 * characters costs more than three quiet cards.
 *
 * The two absences are not the same and do not look the same. `n/a` is "this
 * does not apply to you" -- a barbarian has no spell save DC. `--` is "this
 * applies and is not known here", which is what an unset speed is. Neither is
 * a zero, because `0 ft.` is a claim.
 */

/** Distances arrive as plain numbers; the model's unit is feet. */
function feet(distance: number): string {
  return `${distance} ft.`
}

interface Vital {
  label: string
  value: string | number
  /** A second, quieter line -- the same shape as an ability card's score. */
  hint?: string
}

/**
 * The rows, in order, as lists of cards.
 *
 * Returned as rows rather than one flat list because the break between them
 * is meaning, not wrapping: a row is a subject, and six per line is what makes
 * the subject legible rather than what makes it fit.
 */
function rowsOf(sheet: Sheet): Vital[][] {
  const hitPoints = sheet.base.hitPoints
  const hitDice = (sheet.resources.hitDice ?? []).filter((pool) => pool.dice !== undefined)

  const body: Vital[] = [
    { label: 'Hit points', value: `${hitPoints.current} / ${hitPoints.max}` },
    { label: 'Temp HP', value: hitPoints.temporary ?? 0 },
    { label: 'Hit Dice', value: hitDice.length === 0 ? '--' : hitDice.map((p) => p.dice).join(' · ') },
    { label: 'Armor class', value: sheet.status.armorClass },
    { label: 'Initiative', value: signed(sheet.status.initiative) },
    { label: 'Proficiency', value: signed(sheet.status.proficiencyBonus) },
  ]

  const reach: Vital[] = []

  // Three cards per casting class, not one: attack bonus, save DC and the
  // ability they both come from are three different questions asked at three
  // different moments, and a spell that attacks never wants the DC.
  //
  // A multiclassed cleric/wizard really does have two of each, so the class
  // names them -- picking one set to show would be picking wrong half the
  // time.
  //
  // A character who casts nothing keeps the three cards and reads `n/a`, so
  // that every sheet is the same three rows of six and a reader looking for a
  // number always finds it in the same place. Note this is a *different*
  // absence from the `--` below: a barbarian has no spell save DC at all,
  // where a speed the projection has not sent yet is a number that exists and
  // is not known here.
  const casters = sheet.status.spellcasting ?? []
  if (casters.length === 0) {
    reach.push(
      { label: 'Spell attack bonus', value: 'n/a' },
      { label: 'Spell save DC', value: 'n/a' },
      { label: 'Spellcasting ability', value: 'n/a' },
    )
  }
  for (const caster of casters) {
    const whose = casters.length > 1 ? `${titleCase(caster.class)} ` : ''
    reach.push(
      { label: `${whose}Spell attack bonus`, value: signed(caster.attackBonus) },
      { label: `${whose}Spell save DC`, value: caster.saveDC },
      { label: `${whose}Spellcasting ability`, value: caster.ability.toUpperCase() },
    )
  }

  reach.push({ label: 'Passive Perception', value: sheet.status.passivePerception })

  // Walking leads and anything else is the hint: a character with a fly speed
  // still walks, and the walking number is the one asked for.
  const speeds = sheet.base.speeds ?? []
  const walking = speeds.find((speed) => speed.kind === 'walking')
  const others = speeds.filter((speed) => speed !== walking)
  reach.push({
    label: 'Speed',
    value: walking === undefined ? '--' : feet(walking.distance),
    ...(others.length > 0
      ? { hint: others.map((speed) => `${speed.kind} ${feet(speed.distance)}`).join(' · ') }
      : {}),
  })

  // The sense names the card, because "Vision 60 ft." says less than
  // "Darkvision 60 ft." and the label is the half with room for the word.
  const senses = sheet.base.senses ?? []
  const [first, ...rest] = senses
  reach.push({
    label: first === undefined ? 'Vision' : titleCase(first.kind),
    value: first === undefined ? 'Normal' : feet(first.distance),
    ...(rest.length > 0
      ? { hint: rest.map((sense) => `${titleCase(sense.kind)} ${feet(sense.distance)}`).join(' · ') }
      : {}),
  })

  return [body, reach]
}

export function Vitals({ sheet }: { sheet: Sheet }) {
  return (
    <>
      {rowsOf(sheet).map((row, at) => (
        <SimpleGrid key={at} cols={{ base: 2, sm: 3, lg: 6 }} spacing={{ base: 'xs', sm: 'sm' }}>
          {row.map((vital) => (
            // Spread rather than `hint={vital.hint}`: `exactOptionalPropertyTypes`
            // rejects an optional prop handed an explicit undefined.
            <Stat key={vital.label} {...vital} />
          ))}
        </SimpleGrid>
      ))}
    </>
  )
}

/** One headline number, in a bordered card. */
function Stat({ label, value, hint }: Vital) {
  return (
    <Card withBorder padding="xs" radius="md">
      <Stack gap={0}>
        <Text size="xs" c="dimmed">
          {label}
        </Text>
        <Text fw={700} size="lg">
          {value}
        </Text>
        {hint !== undefined && (
          <Text size="xs" c="dimmed">
            {hint}
          </Text>
        )}
      </Stack>
    </Card>
  )
}
