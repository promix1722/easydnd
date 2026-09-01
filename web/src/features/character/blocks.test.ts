import { describe, expect, it } from 'vitest'

import type { Prompt } from '@/lib/api'

import { blockOrder, blocksFor, groupByLevel, inheritPlace, keyFor, reclaimPlace, settledKey } from './blocks'
import type { SettledRow } from './settled'

function row(over: Partial<SettledRow> & { seq: number }): SettledRow {
  return {
    stage: 'class',
    label: 'Class chosen',
    value: 'Rogue',
    event: { type: 'class', seq: over.seq },
    ...over,
  }
}

function prompt(over: Partial<Prompt> & { choice: Prompt['choice'] }): Prompt {
  return {
    group: 'class',
    optional: false,
    event: { type: 'class' },
    heldOnly: false,
    ...over,
  }
}

const SKILLS = prompt({
  choice: {
    prompt: 'rogue/proficiency/0',
    choose: 2,
    kind: 'proficiency',
    from: { kind: 'explicit', options: [] },
  },
  level: 1,
})

const EXPERTISE = prompt({
  choice: {
    prompt: 'rogue/expertise/0',
    choose: 2,
    kind: 'proficiency',
    from: { kind: 'explicit', options: [] },
  },
  level: 3,
})

const RACE = prompt({
  choice: {
    prompt: 'character/race',
    choose: 1,
    kind: 'race',
    from: { kind: 'collection', collection: 'race' },
  },
  group: 'race',
})

describe('blocksFor', () => {
  it('puts what belongs to no level before what belongs to one', () => {
    const blocks = blocksFor([row({ seq: 4, level: 1 })], [RACE])

    // A race, a name and a background are not waiting on a level, so they are
    // not sorted behind one.
    expect(blocks.map((block) => block.key)).toEqual(['open:character/race', 'settled:4'])
  })

  it('reads up the levels', () => {
    const blocks = blocksFor([row({ seq: 4, level: 1 }), row({ seq: 6, level: 3 })], [EXPERTISE, SKILLS])

    expect(blocks.map((block) => block.key)).toEqual([
      'settled:4',
      'open:rogue/proficiency/0',
      'settled:6',
      'open:rogue/expertise/0',
    ])
  })

  it('keeps what was decided ahead of what is still asked, in the order each arrived', () => {
    const blocks = blocksFor([row({ seq: 4, level: 1 }), row({ seq: 5, level: 1 })], [SKILLS])

    expect(blocks.map((block) => block.key)).toEqual([
      'settled:4',
      'settled:5',
      'open:rogue/proficiency/0',
    ])
  })

  it('draws a level already taken as a statement, with nothing to open', () => {
    const blocks = blocksFor(
      [row({ seq: 5, level: 2, label: 'Level gained', event: { type: 'level', level: 2 } })],
      [],
    )

    expect(blocks[0]).toMatchObject({ kind: 'settled', changeable: false })
  })

  it('leaves everything where it was once it has been drawn', () => {
    const order = blockOrder()
    const first = blocksFor([], [EXPERTISE, SKILLS], order)

    // Level order decided this: the level-1 skills before the level-3
    // expertise, whatever order the server listed them in.
    expect(first.map((block) => block.key)).toEqual([
      'open:rogue/proficiency/0',
      'open:rogue/expertise/0',
    ])

    // A settled entry arriving between them does not push them around: it is
    // new, so it goes to the end. The list grows rather than reshuffling.
    const second = blocksFor([row({ seq: 4, level: 1 })], [EXPERTISE, SKILLS], order)
    expect(second.map((block) => block.key)).toEqual([
      'open:rogue/proficiency/0',
      'open:rogue/expertise/0',
      'settled:4',
    ])
  })

  it('gives an answer the place of the question it answered', () => {
    const order = blockOrder()
    blocksFor([], [SKILLS, EXPERTISE], order)

    // The skills question is answered, and its entry takes its place rather
    // than appearing at the bottom while the question vanishes from above.
    inheritPlace(order, 'open:rogue/proficiency/0', settledKey(7))
    const after = blocksFor([row({ seq: 7, level: 1 })], [EXPERTISE], order)

    expect(after.map((block) => block.key)).toEqual(['settled:7', 'open:rogue/expertise/0'])
  })

  it('holds a dropped answer place for the question that comes back', () => {
    const order = blockOrder()
    blocksFor([row({ seq: 7 })], [EXPERTISE], order)

    // The entry is dropped so that its question can be asked again, and the
    // question that comes back is a different block with a different key --
    // so the place is held for it rather than left at the bottom of the list.
    reclaimPlace(order, settledKey(7))
    const after = blocksFor([], [SKILLS, EXPERTISE], order)

    expect(after.map((block) => block.key)).toEqual([
      'open:rogue/proficiency/0',
      'open:rogue/expertise/0',
    ])
  })

  it('agrees with keyFor about what a block is called', () => {
    const settled = row({ seq: 4 })
    const [decided, open] = blocksFor([settled], [SKILLS])

    // The two spellings live in one file precisely so this holds: a screen
    // that opened a key the list never drew would show nothing, and say
    // nothing about why.
    expect(keyFor({ prompt: SKILLS, replaces: null })).toBe(open?.key)
    expect(keyFor({ prompt: SKILLS, replaces: settled })).toBe(decided?.key)
  })
})

describe('groupByLevel', () => {
  it('cuts the ordered list into its levels, unlevelled first', () => {
    const blocks = blocksFor(
      [row({ seq: 4, level: 1 }), row({ seq: 6, level: 3 })],
      [EXPERTISE, SKILLS, RACE],
    )

    const groups = groupByLevel(blocks)
    expect(groups.map((group) => group.level)).toEqual([undefined, 1, 3])
    expect(groups.map((group) => group.blocks.map((block) => block.key))).toEqual([
      ['open:character/race'],
      ['settled:4', 'open:rogue/proficiency/0'],
      ['settled:6', 'open:rogue/expertise/0'],
    ])
  })

  it('keeps a pinned block with its level, wherever the order put it', () => {
    const order = blockOrder()
    blocksFor([row({ seq: 6, level: 3 })], [], order)
    // A level-1 entry arriving later is pinned at the end of the list --
    // and still grouped under level 1, ahead of the level-3 heading.
    const late = blocksFor([row({ seq: 6, level: 3 }), row({ seq: 7, level: 1 })], [], order)
    expect(late.map((block) => block.key)).toEqual(['settled:6', 'settled:7'])

    expect(groupByLevel(late).map((group) => group.level)).toEqual([1, 3])
  })
})

describe('changeable', () => {
  it('locks the ruleset, which is final, and nothing else', () => {
    const ruleset = row({
      seq: 3,
      stage: 'identity',
      event: {
        type: 'change',
        seq: 3,
        changes: [{ path: 'identity.ruleset', op: 'set', value: { kind: 'slug', slug: '2014' } }],
      },
    })
    const level = row({ seq: 9, event: { type: 'level', seq: 9 } })
    const desired = row({
      seq: 4,
      stage: 'identity',
      event: {
        type: 'change',
        seq: 4,
        changes: [{ path: 'identity.desiredLevel', op: 'set', value: { kind: 'int', int: 3 } }],
      },
    })

    const settled = blocksFor([ruleset, desired, level], []).filter(
      (block) => block.kind === 'settled',
    )
    const changeableBySeq = new Map(settled.map((block) => [block.row.seq, block.changeable]))
    expect(changeableBySeq.get(3)).toBe(false)
    // A bare level entry is a fact, not a control: nothing asked which class
    // it went into. The declared goal is the control that replaces it.
    expect(changeableBySeq.get(9)).toBe(false)
    expect(changeableBySeq.get(4)).toBe(true)
  })
})

describe('what a level grants', () => {
  const levelRow = (over: Partial<SettledRow> & { seq: number }) =>
    row({ ...over, stage: 'class' })

  it('keeps what a level granted changeable, while the level itself is a fact', () => {
    // Both arrive as `level` events. Locking them together took every
    // improvement, Expertise and feature pick off the screen with the levels.
    const improvement = levelRow({
      seq: 11,
      event: {
        type: 'level',
        seq: 11,
        ref: 'class:barbarian',
        level: 4,
        choices: [{ prompt: 'barbarian/ability-score-improvement/4/0', picks: ['wis', 'int'] }],
      },
    })
    const level = levelRow({ seq: 9, event: { type: 'level', seq: 9, ref: 'class:barbarian', level: 3 } })

    const settled = blocksFor([improvement, level], []).filter((b) => b.kind === 'settled')
    const changeable = new Map(settled.map((b) => [b.row.seq, b.changeable]))
    expect(changeable.get(11)).toBe(true)
    expect(changeable.get(9)).toBe(false)
  })
})
