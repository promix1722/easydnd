import { describe, expect, it } from 'vitest'

import { equip, groupOf, mergeStacks, setTotal, slotOf, slotted, unequip } from './equipment'
import type { ItemLike } from './equipment'

const ITEMS = new Map<string, ItemLike>([
  { slug: 'leather-armor', category: 'armor', armor: { category: 'light' } },
  { slug: 'chain-mail', category: 'armor', armor: { category: 'heavy' } },
  { slug: 'shield', category: 'armor', armor: { category: 'shield' } },
  { slug: 'dagger', category: 'weapon', weapon: {} },
  { slug: 'amulet', category: 'adventuring-gear', gear: { gearCategory: 'holy-symbols' } },
  { slug: 'ring-of-protection', category: 'ring' },
  { slug: 'vestments', category: 'adventuring-gear', gear: { gearCategory: 'standard-gear' } },
  { slug: 'arrow', category: 'adventuring-gear', gear: { gearCategory: 'ammunition' } },
  { slug: 'torch', category: 'adventuring-gear', gear: { gearCategory: 'standard-gear' } },
  { slug: 'potion-of-healing', category: 'potion' },
  { slug: 'burglars-pack', category: 'adventuring-gear', gear: { gearCategory: 'equipment-packs' } },
  { slug: 'thieves-tools', category: 'tools' },
  // A namespaced pack qualifies both the slug and the category.
  { slug: 'dnd-2014/ring-of-jumping', category: 'dnd-2014/ring' },
  { slug: 'dnd-2014/torch', category: 'dnd-2014/adventuring-gear', gear: { gearCategory: 'standard-gear' } },
].map((item) => [item.slug, item]))

describe('equipment', () => {
  it.each([
    ['leather-armor', 'wearable', 'armor'],
    ['shield', 'wearable', 'offHand'],
    ['dagger', 'wearable', 'mainHand'],
    ['amulet', 'wearable', 'neck'],
    ['ring-of-protection', 'wearable', 'ring'],
    ['vestments', 'wearable', 'worn'],
    ['arrow', 'consumable', null],
    ['torch', 'consumable', null],
    ['potion-of-healing', 'consumable', null],
    ['burglars-pack', 'gear', null],
    ['thieves-tools', 'gear', null],
    ['dnd-2014/ring-of-jumping', 'wearable', 'ring'],
    ['dnd-2014/torch', 'consumable', null],
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
  })

  it('sends the slot\'s occupant back to the backpack', () => {
    expect(equip(rogue, ITEMS, 'chain-mail', 'armor')).toEqual([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: ['chain-mail'] } },
      { path: 'equipment.equipped.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.equipped.chain-mail', op: 'set', value: { kind: 'int', int: 1 } },
      { path: 'equipment.backpack.chain-mail', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.backpack.leather-armor', op: 'set', value: { kind: 'int', int: 2 } },
    ])
  })

  it('puts a second weapon in the off hand', () => {
    const armed = { ...rogue, equipped: [{ item: 'dagger', count: 2 }] }
    expect(slotted(armed, ITEMS).get('offHand')).toEqual(['dagger'])
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
