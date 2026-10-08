import { describe, expect, it } from 'vitest'
import type { CharacterEvent } from '@/lib/api'
import { createI18n, type Translate } from '@/lib/i18n'
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
  { seq: 7, type: 'level', source: 'class', ref: 'class:rogue', level: 2 },
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
    expect(rows.get('personal')?.map((row) => row.value)).toEqual(['Zephyr'])
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

describe('summarise', () => {
  it('reads the desired level and the ruleset back as decisions', () => {
    const rows = settledByStage(testT, {
      events: [
        {
          seq: 1,
          type: 'change',
          source: 'identity',
          changes: [
            { path: 'identity.desiredLevel', op: 'set', value: { kind: 'int', int: 5 } },
          ],
        },
        {
          seq: 2,
          type: 'change',
          source: 'identity',
          changes: [{ path: 'identity.ruleset', op: 'set', value: { kind: 'slug', slug: '2014' } }],
        },
      ],
      names: new Map(),
    })

    expect(rows.get('class')?.map((row) => [row.label, row.value])).toEqual([['Level', '5']])
    expect(rows.get('rules')?.map((row) => [row.label, row.value])).toEqual([['Rules', 'D&D 2014']])
  })

  // The slug read back as words is English in every language, and under a
  // rule pack it carries the pack's id: "Dnd 2014/neutral Good".
  it('names an alignment as the compendium does', () => {
    const rows = settledByStage(testT, {
      events: [
        {
          seq: 1,
          type: 'change',
          source: 'personality',
          changes: [
            { path: 'identity.alignment', op: 'set', value: { kind: 'slug', slug: 'dnd-2014/neutral-good' } },
          ],
        },
      ],
      names: new Map([['alignment:dnd-2014/neutral-good', 'Нейтрально-добрый']]),
    })

    expect([...rows.values()].flat().map((row) => row.value)).toEqual(['Нейтрально-добрый'])
  })

  // The wizard stores a sheet's traits as a list, and a list of slugs is
  // title-cased with its hyphens read as spaces.
  it('prints what the player wrote as they wrote it', () => {
    const rows = settledByStage(testT, {
      events: [
        {
          seq: 1,
          type: 'change',
          source: 'personality',
          changes: [
            {
              path: 'identity.ideals',
              op: 'set',
              value: { kind: 'slugs', slugs: ['кто-нибудь наконец вступился.', 'second line'] },
            },
          ],
        },
      ],
      names: new Map(),
    })

    expect([...rows.values()].flat().map((row) => row.value)).toEqual([
      'кто-нибудь наконец вступился. · second line',
    ])
  })

  // A question whose options are questions is answered in one entry now: the
  // branch, then what the branch offered. Printing both would name the choice
  // once as itself -- "Expertise, Skill Stealth, Skill Acrobatics".
  it('reads a branch answer as what was chosen inside it, not as the branch', () => {
    const rows = settledByStage(testT, {
      events: [
        {
          seq: 1,
          type: 'level',
          source: 'class',
          ref: 'class:rogue',
          level: 4,
          choices: [
            {
              prompt: 'rogue/ability-score-improvement/4',
              picks: ['rogue/ability-score-improvement/4/0'],
            },
            { prompt: 'rogue/ability-score-improvement/4/0', picks: ['dex', 'dex'] },
          ],
        },
      ],
      names: new Map(),
    })

    expect(rows.get('class')?.map((row) => row.value)).toEqual(['Dexterity, Dexterity'])
  })
})

it('renders resolved equipment bundles with localized names and quantities', () => {
  const events: CharacterEvent[] = [{ seq: 1, type: 'class', source: 'class', ref: 'class:fighter', choices: [{ prompt: 'fighter/starting-equipment/2', picks: ['crossbow-light+crossbow-bolt'] }], selections: [
    { kind: 'ref', key: 'crossbow-light', ref: 'item:crossbow-light', count: 1 },
    { kind: 'ref', key: 'crossbow-bolt', ref: 'item:crossbow-bolt', count: 20 },
  ] }]
  const names = new Map([['class:fighter', 'Воин'], ['item:crossbow-light', 'Арбалет, легкий'], ['item:crossbow-bolt', 'Болт для арбалета']])
  const rows = settledByStage(testT, { events, names }).get('equipment')!
  expect(rows[0]?.value).toContain('Арбалет, легкий')
  expect(rows[0]?.value).toContain('Болт для арбалета ×20')
  expect(rows[0]?.value).not.toContain('Crossbow')
})


it('uses localized source metadata for a saved feature heading', () => {
  const i18n = createI18n('ru')
  const t = i18n.t.bind(i18n) as Translate
  const events: CharacterEvent[] = [{
    seq: 1, type: 'level', source: 'class', ref: 'class:fighter',
    choiceSource: 'feature:fighter-fighting-style', choiceKind: 'feature',
    choices: [{ prompt: 'fighter-fighting-style/0', picks: ['fighter-fighting-style-archery'] }],
    selections: [{ kind: 'ref', key: 'fighter-fighting-style-archery', ref: 'feature:fighter-fighting-style-archery' }],
  }]
  const names = new Map([
    ['feature:fighter-fighting-style', 'Боевой стиль'],
    ['feature:fighter-fighting-style-archery', 'Стрельба'],
  ])
  const row = settledByStage(t, { events, names }).get('class')?.[0]
  expect(row?.label).toBe(`Боевой стиль · ${t('choice.feature', { count: 1 })}`)
  expect(row?.value).toBe('Стрельба')
  expect(row?.label).not.toMatch(/[A-Za-z]/)
})


it('routes saved class, race and background choices by their choice kind without moving the owners', () => {
  const events: CharacterEvent[] = [
    { seq: 1, type: 'class', source: 'class', ref: 'class:wizard' },
    { seq: 2, type: 'level', source: 'class', choiceKind: 'spell', choices: [{ prompt: 'wizard/spell/cantrip/1', picks: ['light'] }] },
    { seq: 3, type: 'race', source: 'race', choiceKind: 'spell', choices: [{ prompt: 'elf/spell/0', picks: ['mage-hand'] }] },
    { seq: 4, type: 'class', source: 'class', choiceKind: 'equipment', choices: [{ prompt: 'wizard/starting-equipment/0', picks: ['dagger'] }] },
    { seq: 5, type: 'background', source: 'background', choiceKind: 'equipment', choices: [{ prompt: 'acolyte/starting-equipment/0', picks: ['amulet'] }] },
    { seq: 6, type: 'level', source: 'class', choices: [{ prompt: 'wizard/spell/prepared/1', picks: ['alarm'] }] },
  ]
  const rows = settledByStage(testT, { events, names: new Map() })
  expect(rows.get('class')?.map((row) => row.seq)).toEqual([1])
  expect(rows.get('cantrips')?.map((row) => row.seq)).toEqual([2])
  expect(rows.get('spells')?.map((row) => row.seq)).toEqual([3, 6])
  expect(rows.get('equipment')?.map((row) => row.seq)).toEqual([4, 5])
  expect(rows.has('race')).toBe(false)
  expect(rows.has('background')).toBe(false)
})
