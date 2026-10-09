import { describe, expect, it } from 'vitest'

import { CUSTOM, ELSEWHERE, discard, equip, groupOf, mergeStacks, setTotal, slotOf, slotsFor, slotted, unequip } from './equipment'
import type { ItemLike } from './equipment'

// The slot comes from the catalogue; what an item otherwise is only decides
// whether it is used up.
const ITEMS = new Map<string, ItemLike>([
  { slug: 'leather-armor', category: 'armor', slot: 'body' },
  { slug: 'chain-mail', category: 'armor', slot: 'body' },
  { slug: 'shield', category: 'armor', slot: 'off-hand' },
  { slug: 'dagger', category: 'weapon', slot: 'main-hand' },
  { slug: 'amulet', category: 'adventuring-gear', slot: 'neck', gear: { gearCategory: 'holy-symbols' } },
  { slug: 'ring-of-protection', category: 'ring', slot: 'ring' },
  { slug: 'ring-of-jumping', category: 'ring', slot: 'ring' },
  { slug: 'boots-of-speed', category: 'wondrous-items', slot: 'feet' },
  { slug: 'cloak-of-protection', category: 'wondrous-items', slot: 'back' },
  { slug: 'helm-of-telepathy', category: 'wondrous-items', slot: 'head' },
  { slug: 'gloves-of-missile-snaring', category: 'wondrous-items', slot: 'arms' },
  // The catalogue reads "hands" as arms; a client never sees the old word.
  { slug: 'old-gauntlets', category: 'wondrous-items', slot: 'hands' },
  { slug: 'bag-of-holding', category: 'wondrous-items' },
  { slug: 'arrow', category: 'adventuring-gear', gear: { gearCategory: 'ammunition' } },
  { slug: 'torch', category: 'adventuring-gear', gear: { gearCategory: 'standard-gear' } },
  { slug: 'potion-of-healing', category: 'potion' },
  { slug: 'thieves-tools', category: 'tools' },
  { slug: 'dnd-2014/torch', category: 'dnd-2014/adventuring-gear', gear: { gearCategory: 'standard-gear' } },
  // A slot this client does not know is no slot.
  { slug: 'homebrew-elbow-pad', category: 'wondrous-items', slot: 'elbow' },
].map((item) => [item.slug, item]))

describe('equipment', () => {
  it.each([
    ['leather-armor', 'wearable', 'body'],
    ['shield', 'wearable', 'off-hand'],
    ['dagger', 'wearable', 'main-hand'],
    ['amulet', 'wearable', 'neck'],
    ['ring-of-protection', 'wearable', 'ring'],
    ['boots-of-speed', 'wearable', 'feet'],
    ['cloak-of-protection', 'wearable', 'back'],
    ['helm-of-telepathy', 'wearable', 'head'],
    ['gloves-of-missile-snaring', 'wearable', 'arms'],
    ['old-gauntlets', 'gear', null],
    ['bag-of-holding', 'gear', null],
    ['arrow', 'consumable', null],
    ['torch', 'consumable', null],
    ['potion-of-healing', 'consumable', null],
    ['thieves-tools', 'gear', null],
    ['dnd-2014/torch', 'consumable', null],
    ['homebrew-elbow-pad', 'gear', null],
    ['homebrew-unknown', 'gear', null],
  ])('%s is %s in slot %s', (slug, group, slot) => {
    expect(groupOf(ITEMS.get(slug))).toBe(group)
    expect(slotOf(ITEMS.get(slug))).toBe(slot)
  })

  const rogue = {
    equipped: [{ item: 'leather-armor', count: 1 }],
    backpack: [{ item: 'leather-armor', count: 1 }, { item: 'dagger', count: 2 }, { item: 'chain-mail', count: 1 }],
    loot: [],
  }

  it('shows an item held in two lists once', () => {
    expect(mergeStacks(rogue).find((row) => row.item === 'leather-armor')).toMatchObject({ count: 2, equipped: 1 })
    // Putting on one of two daggers leaves the row where it was.
    const order = (equipment: typeof rogue) => mergeStacks(equipment).map((row) => row.item)
    expect(order({ ...rogue, equipped: [...rogue.equipped, { item: 'dagger', count: 1 }] })).toEqual(order(rogue))
  })

  it('sends the slot\'s occupant back to the backpack', () => {
    expect(equip(rogue, ITEMS, 'chain-mail', 'body')).toEqual([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: ['chain-mail'] } },
      { path: 'equipment.equipped.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.equipped.chain-mail', op: 'set', value: { kind: 'int', int: 1 } },
      { path: 'equipment.backpack.chain-mail', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.backpack.leather-armor', op: 'set', value: { kind: 'int', int: 2 } },
    ])
  })

  it('puts a second weapon in the off hand', () => {
    const armed = { ...rogue, equipped: [{ item: 'dagger', count: 2 }] }
    expect(slotted(armed, ITEMS).get('off-hand')).toEqual(['dagger'])
    // The off hand is offered only beside a held main hand: alone, a held item sits in the main hand whatever was asked.
    expect(slotsFor(rogue, ITEMS, ITEMS.get('dagger'))).toEqual(['main-hand', CUSTOM])
    expect(slotsFor(armed, ITEMS, ITEMS.get('dagger'))).toEqual(['main-hand', 'off-hand', CUSTOM])
  })

  it('wears two rings, lists a slot worn past its capacity, and keeps the slotless in sight', () => {
    const laden = {
      ...rogue,
      equipped: [
        { item: 'ring-of-protection', count: 1 }, { item: 'ring-of-jumping', count: 1 },
        { item: 'leather-armor', count: 1 }, { item: 'chain-mail', count: 1 },
        { item: 'torch', count: 1 },
      ],
    }
    const worn = slotted(laden, ITEMS)
    expect(worn.get('ring')).toEqual(['ring-of-protection', 'ring-of-jumping'])
    expect(worn.get('body')).toEqual(['leather-armor', 'chain-mail'])
    expect(worn.get(ELSEWHERE)).toEqual(['torch'])
    expect(slotted(rogue, ITEMS).get(ELSEWHERE)).toEqual([])
  })

  // Custom is the one slot the server stores: any wearable fits, its occupant is
  // seated before the rest are placed by shape, and moving in or out of it
  // writes the placement as well as the lists.
  it('seats the stored Custom occupant first and takes any wearable', () => {
    const odd = { ...rogue, equipped: [{ item: 'leather-armor', count: 1 }, { item: 'torch', count: 1 }], custom: 'torch' }
    expect(slotted(odd, ITEMS).get(CUSTOM)).toEqual(['torch'])
    expect(slotted(odd, ITEMS).get(ELSEWHERE)).toEqual([])
    // Any wearable, not anything: a bag has no slot and gets no slot.
    expect(slotsFor(rogue, ITEMS, ITEMS.get('cloak-of-protection'))).toEqual(['back', CUSTOM])
    expect(slotsFor(rogue, ITEMS, ITEMS.get('bag-of-holding'))).toEqual([])
    // Dropping what is worn takes it off and puts it nowhere.
    expect(discard(odd, 'torch', CUSTOM).map((change) => change.path)).toEqual(['equipment.equipped', 'equipment.equipped.torch', 'equipment.custom'])
    // A worn armor placed in Custom frees the body slot for another.
    const twice = { ...odd, equipped: [{ item: 'leather-armor', count: 1 }, { item: 'chain-mail', count: 1 }], custom: 'leather-armor' }
    expect(slotted(twice, ITEMS).get('body')).toEqual(['chain-mail'])

    expect(equip(rogue, ITEMS, 'dagger', CUSTOM)).toEqual([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: ['leather-armor', 'dagger'] } },
      { path: 'equipment.equipped.dagger', op: 'set', value: { kind: 'int', int: 1 } },
      { path: 'equipment.backpack.dagger', op: 'set', value: { kind: 'int', int: 1 } },
      { path: 'equipment.custom', op: 'set', value: { kind: 'slugs', slugs: ['dagger'] } },
    ])
    expect(unequip(odd, 'torch', CUSTOM)).toEqual([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: ['leather-armor'] } },
      { path: 'equipment.equipped.torch', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.backpack.torch', op: 'set', value: { kind: 'int', int: 1 } },
      { path: 'equipment.custom', op: 'set', value: { kind: 'slugs', slugs: [] } },
    ])
    // Off the body, not out of Custom: the server forgets the slot itself.
    expect(unequip(odd, 'torch')).toHaveLength(3)
  })

  it('takes an item off into the backpack', () => {
    expect(unequip(rogue, 'leather-armor')).toEqual([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: [] } },
      { path: 'equipment.equipped.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.backpack.leather-armor', op: 'set', value: { kind: 'int', int: 2 } },
    ])
  })

  it('drops from the backpack before the body', () => {
    expect(setTotal(rogue, 'leather-armor', 1)).toEqual([
      { path: 'equipment.backpack.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
    ])
    expect(setTotal(rogue, 'dagger', 3)).toEqual([
      { path: 'equipment.backpack.dagger', op: 'set', value: { kind: 'int', int: 3 } },
    ])
  })
})
