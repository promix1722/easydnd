import { describe, expect, it } from 'vitest'
import type { Entry, Item, Prompt } from '@/lib/api'
import { testT } from '@/test/i18n'

import { choosableOptions, equipmentTitle } from './options'

function prompt(overrides: Partial<Prompt>): Prompt {
  return {
    choice: { prompt: 'test', choose: 1, kind: 'proficiency', from: { kind: 'explicit', options: [] } },
    group: 'class',
    optional: false,
    heldOnly: false,
    event: { type: 'class' },
    ...overrides,
  }
}

const entries = new Map<string, Entry>([
  ['shortbow', { slug: 'shortbow', name: 'Shortbow', icon: 'data:image/webp;base64,Ym93', provenance: { packId: 'srd', packTitle: 'SRD', version: '1', digest: 'd', sources: [] } } as Item],
  ['arrow', { slug: 'arrow', name: 'Arrow', gear: { gearCategory: 'ammunition' } } as Entry],
  ['shortsword', { slug: 'shortsword', name: 'Shortsword' }],
])

describe('choosableOptions', () => {
  // The rogue's starting kit is the case the server's option keys exist for:
  // the bundle has no slug, so it is named by position.
  it('uses the key the server sent, never one it computes', () => {
    const got = choosableOptions(
      testT,
      prompt({
        choice: {
          prompt: 'rogue/starting-equipment/1',
          choose: 1,
          kind: 'equipment',
          from: {
            kind: 'explicit',
            options: [
              {
                key: 'shortbow+arrow',
                kind: 'bundle',
                items: [
                  { key: 'shortbow', kind: 'ref', ref: 'item:shortbow', count: 1 },
                  { key: 'arrow', kind: 'ref', ref: 'item:arrow', count: 20 },
                ],
              },
              { key: 'shortsword', kind: 'ref', ref: 'item:shortsword', count: 1 },
            ],
          },
        },
      }),
      entries,
    )

    expect(got.map((o) => o.key)).toEqual(['shortbow+arrow', 'shortsword'])
    // The arrows go without saying; the bundle wears its bow's badges.
    expect(got[0]?.label).toBe('Shortbow')
    expect(got[0]?.provenance?.packId).toBe('srd')
    expect(got[0]?.detail).toContain('Arrow ×20')
    expect(got[1]?.label).toBe('Shortsword')
  })

  it('disables what the character already has', () => {
    const got = choosableOptions(
      testT,
      prompt({
        held: ['shortsword'],
        choice: {
          prompt: 'p',
          choose: 1,
          kind: 'proficiency',
          from: {
            kind: 'explicit',
            options: [
              { key: 'shortbow', kind: 'ref', ref: 'item:shortbow' },
              { key: 'shortsword', kind: 'ref', ref: 'item:shortsword' },
            ],
          },
        },
      }),
      entries,
    )

    expect(got.find((o) => o.key === 'shortbow')?.disabled).toBe(false)
    const held = got.find((o) => o.key === 'shortsword')
    expect(held?.disabled).toBe(true)
    expect(held?.reason).toBe('already have')
  })

  // Expertise inverts it: doubling a proficiency requires having it.
  it('inverts the test when heldOnly is set', () => {
    const got = choosableOptions(
      testT,
      prompt({
        heldOnly: true,
        held: ['shortsword'],
        choice: {
          prompt: 'rogue-expertise-1/expertise/0/0',
          choose: 2,
          kind: 'proficiency',
          from: {
            kind: 'explicit',
            options: [
              { key: 'shortbow', kind: 'ref', ref: 'item:shortbow' },
              { key: 'shortsword', kind: 'ref', ref: 'item:shortsword' },
            ],
          },
        },
      }),
      entries,
    )

    expect(got.find((o) => o.key === 'shortsword')?.disabled).toBe(false)
    const missing = got.find((o) => o.key === 'shortbow')
    expect(missing?.disabled).toBe(true)
    expect(missing?.reason).toBe('not proficient')
  })

  // A set drawn from a collection lists nothing inline; the collection is the
  // option list and an entry's own slug is its key.
  it('falls back to the loaded collection for a non-explicit set', () => {
    const got = choosableOptions(
      testT,
      prompt({
        choice: {
          prompt: 'character/race',
          choose: 1,
          kind: 'race',
          from: { kind: 'collection', collection: 'race' },
        },
      }),
      entries,
    )
    expect(got.map((o) => o.key).sort()).toEqual(['arrow', 'shortbow', 'shortsword'])
  })

  it('labels an ability bonus the way a player reads it', () => {
    const got = choosableOptions(
      testT,
      prompt({
        choice: {
          prompt: 'half-elf/ability-bonus/0',
          choose: 2,
          kind: 'ability-bonus',
          from: {
            kind: 'explicit',
            options: [{ key: 'dex', kind: 'ability-bonus', ability: 'dex', bonus: 1 }],
          },
        },
      }),
      new Map(),
    )
    expect(got[0]?.label).toBe('Dexterity +1')
  })
})

it('describes armor statistics and every item of a bundle', () => {
  const loaded = new Map<string, import('@/lib/api').Item>([
    ['chain-mail', { slug: 'chain-mail', name: 'Chain Mail', armor: { baseAC: 16, strengthMinimum: 13, stealthDisadvantage: true }, desc: ['First paragraph.', 'Second paragraph.'] }],
    ['arrow', { slug: 'arrow', name: 'Arrow', weight: 1 }],
  ])
  const got = choosableOptions(testT, prompt({ choice: { prompt: 'fighter/starting-equipment/0', choose: 1, kind: 'equipment', from: { kind: 'explicit', options: [{ key: 'kit', kind: 'bundle', items: [
    { key: 'chain-mail', kind: 'ref', ref: 'item:chain-mail', count: 1 }, { key: 'arrow', kind: 'ref', ref: 'item:arrow', count: 20 },
  ] }] } } }), loaded)
  expect(got[0]?.detail).toContain('Second paragraph.')
  expect(got[0]?.detail).toContain('Armor class: 16')
  expect(got[0]?.detail).toContain('Strength required: 13')
  expect(got[0]?.detail).toContain('Arrow ×20')
})

// A warlock's four equipment questions were all headed "Starting equipment";
// each is named by what it offers, and the one with no alternative says "also".
describe('equipmentTitle', () => {
  const names = new Map([['item:component-pouch', 'Component pouch'], ['item:dagger', 'Dagger']])
  const asks = (from: Prompt['choice']['from'], kind = 'equipment'): Prompt => ({
    choice: { prompt: 'warlock/starting-equipment/0', choose: 1, kind, from },
    group: 'class', optional: true, heldOnly: false, event: { type: 'class' },
  } as Prompt)
  const focus = { key: 'arcane-foci', kind: 'nested' as const, choice: { prompt: 'x/1', choose: 1, kind: 'equipment', from: { kind: 'explicit' as const, category: 'dnd-2014/arcane-foci', options: [] } } }

  it('is titled by the slot the server put on the choice', () => {
    const slotted = asks({ kind: 'explicit', options: [{ key: 'component-pouch', kind: 'ref', ref: 'item:component-pouch', count: 1 }, focus] })
    slotted.choice.slot = 'off-hand'
    expect(equipmentTitle(testT, slotted, names)).toBe('Off hand')
    slotted.choice.slot = 'backup'
    expect(equipmentTitle(testT, slotted, names)).toBe('Backup weapon')
    // A word the client has no caption for is a pack's own business.
    slotted.choice.slot = 'saddle'
    expect(equipmentTitle(testT, slotted, names)).toBe('Component pouch or one of: Arcane Foci')
  })

  it('names a card by its options', () => {
    expect(equipmentTitle(testT, asks({ kind: 'explicit', options: [{ key: 'component-pouch', kind: 'ref', ref: 'item:component-pouch', count: 1 }, focus] }), names))
      .toBe('Component pouch or one of: Arcane Foci')
  })

  it('says "also" for a category with no alternative', () => {
    expect(equipmentTitle(testT, asks({ kind: 'explicit', category: 'simple-weapons', options: [{ key: 'dagger', kind: 'ref', ref: 'item:dagger', count: 1 }] }), names))
      .toBe('Also one of: Simple Weapons')
  })

  it('leaves other questions, and long lists, to their usual name', () => {
    expect(equipmentTitle(testT, asks({ kind: 'explicit', options: [] }), names)).toBeUndefined()
    expect(equipmentTitle(testT, asks({ kind: 'explicit', category: 'simple-weapons', options: [] }, 'proficiency'), names)).toBeUndefined()
  })
})
