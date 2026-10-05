/**
 * What an item is for, and where it is worn -- derived, because the catalogue
 * says neither. An item has a category and at most one of armor/weapon/gear;
 * nothing there says "consumable" or "goes on a finger".
 *
 * Everything here also writes the changes an inventory edit is posted as, so
 * the sheet and the build screen cannot come to disagree about what "put it
 * on" means.
 */

/** The part of a catalogue item (or magic item) this file reads. */
export interface ItemLike {
  slug: string
  category?: string
  armor?: { category?: string }
  weapon?: object
  gear?: { gearCategory?: string }
}

export interface StackLike {
  item?: string
  count: number
  custom?: { name: string }
}

export interface EquipmentLike {
  equipped: StackLike[]
  backpack: StackLike[]
  loot: StackLike[]
}

/** A change as the events route takes it; structurally `lib/api`'s `Change`. */
export interface EquipmentChange {
  path: string
  op: 'set'
  value: { kind: 'int'; int: number } | { kind: 'slugs'; slugs: string[] }
}

export type ItemGroup = 'wearable' | 'consumable' | 'gear'
export const ITEM_GROUPS: readonly ItemGroup[] = ['wearable', 'consumable', 'gear']

export type Slot = 'armor' | 'mainHand' | 'offHand' | 'neck' | 'ring' | 'worn'

/** Slots in the order the panel draws them, and how many items each holds. */
export const SLOTS: readonly { slot: Slot; capacity: number }[] = [
  { slot: 'armor', capacity: 1 },
  { slot: 'mainHand', capacity: 1 },
  { slot: 'offHand', capacity: 1 },
  { slot: 'neck', capacity: 1 },
  { slot: 'ring', capacity: 2 },
  { slot: 'worn', capacity: Infinity },
]

// ponytail: slug lists, because SRD standard gear and wondrous items carry no
// "used up" or body-part field. Homebrew the rules below miss lands in `gear`
// with no slot, and there are no head/cloak/boots slots. Add a catalogue
// `slot`/`consumable` field (wire.go -> convert -> dto, pack version bump)
// when either matters.
const USED_UP = new Set([
  'acid-vial', 'alchemists-fire-flask', 'antitoxin-vial', 'ball-bearings-bag-of-1000',
  'block-of-incense', 'caltrops', 'candle', 'chalk-1-piece', 'holy-water-flask',
  'ink-1-ounce-bottle', 'oil-flask', 'paper-one-sheet', 'parchment-one-sheet', 'perfume-vial',
  'piton', 'poison-basic-vial', 'rations-1-day', 'sealing-wax', 'soap', 'spike-iron', 'torch',
])
const CLOTHING = new Set([
  'clothes-common', 'clothes-costume', 'clothes-fine', 'clothes-travelers', 'robes', 'vestments',
])
const HELD = new Set(['weapon', 'wand', 'staff', 'rod'])
const FOCI = new Set(['arcane-foci', 'druidic-foci'])

// A pack may namespace its slugs ("dnd-2014/amulet"); the lists above are bare.
const bare = (slug: string) => slug.slice(slug.lastIndexOf('/') + 1)

/** Where an item goes when it is put on, or null when it is only carried. */
export function slotOf(item: ItemLike | undefined): Slot | null {
  if (item === undefined) return null
  const slug = bare(item.slug)
  if (item.armor?.category === 'shield') return 'offHand'
  // A namespaced pack qualifies the category too: "dnd-2014/armor".
  const category = bare(item.category ?? '')
  if (item.armor !== undefined || category === 'armor') return 'armor'
  if (item.weapon !== undefined || HELD.has(category)) return 'mainHand'
  const gear = item.gear?.gearCategory ?? ''
  if (FOCI.has(gear)) return 'mainHand'
  if (gear === 'holy-symbols') return slug === 'amulet' ? 'neck' : 'worn'
  if (category === 'ring' || slug === 'signet-ring') return 'ring'
  if (category === 'wondrous-items' || CLOTHING.has(slug)) return 'worn'
  return null
}

/** Whether an item may go in a slot: its own, or the off hand for anything held. */
export function fitsSlot(item: ItemLike | undefined, slot: Slot): boolean {
  const own = slotOf(item)
  return own === slot || (slot === 'offHand' && own === 'mainHand')
}

export function groupOf(item: ItemLike | undefined): ItemGroup {
  if (item === undefined) return 'gear'
  if (slotOf(item) !== null) return 'wearable'
  const used = item.gear?.gearCategory === 'ammunition' ||
    ['potion', 'scroll', 'ammunition'].includes(bare(item.category ?? '')) ||
    USED_UP.has(bare(item.slug))
  return used ? 'consumable' : 'gear'
}

/** One inventory row: an entity once, however many lists it is split across. */
export interface InventoryRow {
  /** The item slug, or `custom:<name>` for an item the catalogue does not hold. */
  key: string
  item?: string
  customName?: string
  count: number
  equipped: number
}

export function mergeStacks(equipment: EquipmentLike): InventoryRow[] {
  const rows = new Map<string, InventoryRow>()
  const add = (stack: StackLike, equipped: boolean) => {
    const key = stack.item ?? `custom:${stack.custom?.name ?? ''}`
    const row = rows.get(key) ?? {
      key,
      ...(stack.item === undefined ? { customName: stack.custom?.name ?? '' } : { item: stack.item }),
      count: 0,
      equipped: 0,
    }
    row.count += stack.count
    if (equipped) row.equipped += stack.count
    rows.set(key, row)
  }
  equipment.equipped.forEach((stack) => add(stack, true))
  equipment.backpack.forEach((stack) => add(stack, false))
  equipment.loot.forEach((stack) => add(stack, false))
  return [...rows.values()]
}

/** Every equipped unit as a slug, in the order it was put on. */
function equippedSlugs(equipment: EquipmentLike): string[] {
  return equipment.equipped.flatMap((stack) =>
    stack.item === undefined ? [] : Array<string>(stack.count).fill(stack.item))
}

/**
 * Which equipped items sit in which slot.
 *
 * Derived rather than stored: the server keeps one equipped list. A second
 * held item takes the off hand; whatever is equipped beyond a slot's capacity
 * -- or has no slot at all -- is shown under `worn` rather than hidden.
 */
export function slotted(equipment: EquipmentLike, items: ReadonlyMap<string, ItemLike>): Map<Slot, string[]> {
  const bySlot = new Map<Slot, string[]>(SLOTS.map(({ slot }) => [slot, []]))
  const room = (slot: Slot) =>
    bySlot.get(slot)!.length < SLOTS.find((entry) => entry.slot === slot)!.capacity
  for (const slug of equippedSlugs(equipment)) {
    const own = slotOf(items.get(slug))
    const slot = own !== null && room(own) ? own : own === 'mainHand' && room('offHand') ? 'offHand' : 'worn'
    bySlot.get(slot)!.push(slug)
  }
  return bySlot
}

const count = (list: StackLike[], slug: string) =>
  list.reduce((sum, stack) => (stack.item === slug ? sum + stack.count : sum), 0)

const setCount = (list: 'equipped' | 'backpack' | 'loot', slug: string, int: number): EquipmentChange => ({
  path: `equipment.${list}.${slug}`,
  op: 'set',
  value: { kind: 'int', int },
})

/**
 * The equipped list, written whole *and* per slug.
 *
 * Both, because the server reads them at different moments: the list form is
 * applied before pack rules (Unarmored Defense asks what is worn), the counted
 * form after starting equipment -- and an import writes the counted form, so
 * only a later counted write can take such an item off again.
 */
function equippedChanges(before: string[], after: string[]): EquipmentChange[] {
  const touched = [...new Set([...before, ...after])].filter(
    (slug) => before.filter((each) => each === slug).length !== after.filter((each) => each === slug).length,
  )
  return [
    { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: after } },
    ...touched.map((slug) => setCount('equipped', slug, after.filter((each) => each === slug).length)),
  ]
}

/** Moves one `slug` from the backpack into `slot`, sending a full slot's first occupant back. */
export function equip(
  equipment: EquipmentLike,
  items: ReadonlyMap<string, ItemLike>,
  slug: string,
  slot: Slot,
): EquipmentChange[] {
  const before = equippedSlugs(equipment)
  const after = [...before]
  const changes: EquipmentChange[] = []
  const occupants = slotted(equipment, items).get(slot) ?? []
  const capacity = SLOTS.find((entry) => entry.slot === slot)?.capacity ?? Infinity
  let carried = count(equipment.backpack, slug)
  const displaced = occupants.length >= capacity ? occupants[0] : undefined
  if (displaced !== undefined) {
    after.splice(after.indexOf(displaced), 1)
    if (displaced === slug) carried += 1
    else changes.push(setCount('backpack', displaced, count(equipment.backpack, displaced) + 1))
  }
  after.push(slug)
  // Callers offer only what the backpack holds; the floor is for a stale click.
  return [...equippedChanges(before, after), setCount('backpack', slug, Math.max(carried - 1, 0)), ...changes]
}

/** Takes one `slug` off and puts it in the backpack. */
export function unequip(equipment: EquipmentLike, slug: string): EquipmentChange[] {
  const before = equippedSlugs(equipment)
  const at = before.indexOf(slug)
  if (at < 0) return []
  const after = [...before]
  after.splice(at, 1)
  return [...equippedChanges(before, after), setCount('backpack', slug, count(equipment.backpack, slug) + 1)]
}

/**
 * Sets how many of `slug` the character has in all.
 *
 * More goes into the backpack. Fewer comes out of the backpack first, then
 * the loot, and only then off the character's back.
 */
export function setTotal(equipment: EquipmentLike, slug: string, total: number): EquipmentChange[] {
  const backpack = count(equipment.backpack, slug)
  const loot = count(equipment.loot, slug)
  const before = equippedSlugs(equipment)
  const worn = before.filter((each) => each === slug).length
  const delta = Math.max(total, 0) - (backpack + loot + worn)
  if (delta === 0) return []
  if (delta > 0) return [setCount('backpack', slug, backpack + delta)]
  let drop = -delta
  const changes: EquipmentChange[] = []
  const take = (have: number) => {
    const taken = Math.min(have, drop)
    drop -= taken
    return have - taken
  }
  if (backpack > 0) changes.push(setCount('backpack', slug, take(backpack)))
  if (drop > 0 && loot > 0) changes.push(setCount('loot', slug, take(loot)))
  if (drop > 0) {
    const after = [...before]
    for (; drop > 0 && after.includes(slug); drop -= 1) after.splice(after.lastIndexOf(slug), 1)
    changes.push(...equippedChanges(before, after))
  }
  return changes
}

export const COINS = ['cp', 'sp', 'ep', 'gp', 'pp'] as const

export function setCoin(unit: string, amount: number): EquipmentChange {
  return { path: `equipment.purse.${unit}`, op: 'set', value: { kind: 'int', int: Math.max(amount, 0) } }
}
