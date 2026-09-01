import { describe, expect, it } from 'vitest'
import type { CharacterEvent } from '@/lib/api'
import { testT } from '@/test/i18n'

import { settledByStage } from './settled'

/**
 * A log captured from the real router: a half-elf rogue built through the API
 * to second level, under one entry per selection.
 *
 * Every entry carries the `source` the server wrote. That is the whole point
 * of the fixture -- nothing below infers a category from an event's type, and
 * a fixture without sources would pass whether it inferred or not.
 */
const EVENTS: CharacterEvent[] = [
  {
    seq: 1,
    type: 'init',
    source: 'identity',
    changes: [{ path: 'identity.name', op: 'set', value: { kind: 'string', string: 'Zephyr' } }],
  },
  {
    seq: 2,
    type: 'change',
    source: 'abilities',
    changes: [
      { path: 'abilities.method', op: 'set', value: { kind: 'slug', slug: 'point-buy' } },
      { path: 'abilities.str', op: 'set', value: { kind: 'int', int: 10 } },
      { path: 'abilities.dex', op: 'set', value: { kind: 'int', int: 15 } },
    ],
  },
  { seq: 3, type: 'race', source: 'race', ref: 'race:half-elf' },
  {
    seq: 4,
    type: 'race',
    source: 'race',
    ref: 'race:half-elf',
    choices: [{ prompt: 'half-elf/ability-bonus/0', picks: ['dex', 'con'] }],
  },
  { seq: 5, type: 'background', source: 'background', ref: 'background:acolyte' },
  { seq: 6, type: 'class', source: 'class', ref: 'class:rogue', level: 1 },
  { seq: 7, type: 'level', source: 'advance', ref: 'class:rogue', level: 2 },
  {
    seq: 9,
    type: 'class',
    source: 'class',
    level: 1,
    choices: [
      {
        prompt: 'barbarian/proficiency/0',
        picks: ['skill-animal-handling', 'skill-athletics'],
      },
    ],
  },
  // Answered by nobody the server could name: an imported log, a DM's ruling.
  { seq: 8, type: 'note', note: 'Joined the party in Waterdeep.' },
]

const NAMES = new Map([
  ['race:half-elf', 'Half-Elf'],
  ['background:acolyte', 'Acolyte'],
  ['class:rogue', 'Rogue'],
  ['class:barbarian', 'Варвар'],
  ['skill-animal-handling', 'Навык: Уход за животными'],
  ['skill-athletics', 'Навык: Атлетика'],
])

const settled = () => settledByStage(testT, { events: EVENTS, names: NAMES })

describe('settledByStage', () => {
  it('groups by the source the server wrote, not by the event type', () => {
    const rows = settled()

    // A `level` entry and a `class` entry are different types and the same
    // tab; a `change` entry could be either the six scores or a DM's ruling,
    // and only the server knows which prompt it settled.
    expect(rows.get('class')?.map((row) => row.value)).toEqual([
      'Rogue',
      'Rogue',
      'Навык: Уход за животными, Навык: Атлетика',
    ])
    expect(rows.get('abilities')?.map((row) => row.label)).toEqual(['Ability scores'])
    expect(rows.get('identity')?.map((row) => row.value)).toEqual(['Zephyr'])
  })

  it('carries the seq of the entry behind every row', () => {
    // One entry per selection is what makes a row changeable at all: a row
    // assembled from two entries would have no answer to "change this".
    expect(settled().get('race')?.map((row) => row.seq)).toEqual([3, 4])
  })

  it('reads an entry that carries answers by its prompt', () => {
    const rows = settled().get('race') ?? []

    expect(rows[1]).toMatchObject({
      label: 'Half Elf · Ability Bonus',
      value: 'Dexterity, Constitution',
    })
  })

  it('localizes the owner and kind of a settled prompt', () => {
    const row = settled().get('class')?.find((candidate) => candidate.seq === 9)

    expect(row?.label).toBe('Варвар · Proficiencies')
  })

  it('reads the six scores back as scores rather than as a patch', () => {
    expect(settled().get('abilities')?.[0]?.value).toBe(
      'Strength 10 · Dexterity 15 · Point Buy',
    )
  })

  it('keeps a level with the level it was', () => {
    expect(settled().get('class')?.map((row) => row.level)).toEqual([1, 2, 1])
  })

  it('gives no tab to an entry the server could not attribute', () => {
    // Not lost: /characters/:id/log is the unabridged record. This screen is
    // a constructor, and an entry answering no question constructs nothing.
    const rows = [...settled().values()].flat()
    expect(rows.map((row) => row.seq)).not.toContain(8)
  })
})
