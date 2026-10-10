/**
 * Where an item is worn, and what it is for.
 *
 * The catalogue says where: every item carries a `slot`, written on the pack's
 * row or derived by the server from what the item is. Whether an
 * item is used up it does not say, so that is still guessed here.
 *
 * Everything here also writes the changes an inventory edit is posted as, so
 * the sheet and the build screen cannot come to disagree about what "put it
 * on" means.
 */

/** The part of a catalogue item (or magic item) this file reads. */
export interface ItemLike {
  slug: string
  category?: string
  slot?: string
  gear?: { gearCategory?: string }
}

export interface StackLike {
  item?: string
  count: number
}

export interface EquipmentLike {
  equipped: StackLike[]
  backpack: StackLike[]
  loot: StackLike[]
  /** The equipped item in the Custom slot, the one placement the server records. */
  custom?: string
}

/** A change as the events route takes it; structurally `lib/api`'s `Change`. */
export interface EquipmentChange {
  path: string
  op: 'set'
  value: { kind: 'int'; int: number } | { kind: 'slugs'; slugs: string[] }
}

export type ItemGroup = 'wearable' | 'consumable' | 'gear'

/**
 * The DMG's "Wearing and Wielding Items" set, as the catalogue spells it:
 * one of each worn piece, two rings, a hand for each held thing -- and
 * `custom`, which the catalogue never writes: the sheet's one slot that takes
 * any wearable, so its occupant is stored on the character rather than derived.
 */
export type Slot =
  | 'head' | 'neck' | 'back' | 'body' | 'arms' | 'waist' | 'feet'
  | 'ring' | 'main-hand' | 'off-hand' | 'custom'

export const CUSTOM = 'custom'

/** Slots head to foot, then the hands, and how many items each holds. */
export const SLOTS: readonly { slot: Slot; capacity: number }[] = [
  { slot: 'head', capacity: 1 },
  { slot: 'neck', capacity: 1 },
  { slot: 'back', capacity: 1 },
  { slot: 'body', capacity: 1 },
  { slot: 'arms', capacity: 1 },
  { slot: 'waist', capacity: 1 },
  { slot: 'feet', capacity: 1 },
  { slot: 'main-hand', capacity: 1 },
  { slot: 'off-hand', capacity: 1 },
  { slot: 'ring', capacity: 2 },
  { slot: CUSTOM, capacity: 1 },
]

const capacityOf = (slot: Slot) => SLOTS.find((entry) => entry.slot === slot)?.capacity ?? Infinity

// ponytail: a slug list, because SRD gear carries no "used up" field. Homebrew
// it misses lands in `gear`. Add a catalogue `consumable` field beside `slot`
// when that matters.
const USED_UP = new Set([
  'acid-vial', 'alchemists-fire-flask', 'antitoxin-vial', 'ball-bearings-bag-of-1000',
  'block-of-incense', 'caltrops', 'candle', 'chalk-1-piece', 'holy-water-flask',
  'ink-1-ounce-bottle', 'oil-flask', 'paper-one-sheet', 'parchment-one-sheet', 'perfume-vial',
  'piton', 'poison-basic-vial', 'rations-1-day', 'sealing-wax', 'soap', 'spike-iron', 'torch',
])

// A pack may namespace its slugs ("dnd-2014/torch"); the list above is bare.
const bare = (slug: string) => slug.slice(slug.lastIndexOf('/') + 1)

/** Where an item goes when it is put on, or null when it is only carried. */
export function slotOf(item: ItemLike | undefined): Slot | null {
  const slot = item?.slot
  return slot !== undefined && slot !== CUSTOM && SLOTS.some((entry) => entry.slot === slot) ? (slot as Slot) : null
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
  /** The item slug. */
  key: string
  item?: string
  count: number
  equipped: number
}

export function mergeStacks(equipment: EquipmentLike): InventoryRow[] {
  const rows = new Map<string, InventoryRow>()
  const add = (stack: StackLike, equipped: boolean) => {
    const key = stack.item ?? ''
    const row = rows.get(key) ?? {
      key,
      ...(stack.item === undefined ? {} : { item: stack.item }),
      count: 0,
      equipped: 0,
    }
    row.count += stack.count
    if (equipped) row.equipped += stack.count
    rows.set(key, row)
  }
  // What is carried first: a row keeps its place when one of several is put
  // on, where leading with the worn would jump it to the top of the list.
  equipment.backpack.forEach((stack) => add(stack, false))
  equipment.loot.forEach((stack) => add(stack, false))
  equipment.equipped.forEach((stack) => add(stack, true))
  return [...rows.values()]
}

/** Every equipped unit as a slug, in the order it was put on. */
function equippedSlugs(equipment: EquipmentLike): string[] {
  return equipment.equipped.flatMap((stack) =>
    stack.item === undefined ? [] : Array<string>(stack.count).fill(stack.item))
}

/** Equipped items with no slot to show them in: an import, an old log, homebrew without the field. */
export const ELSEWHERE = 'elsewhere'

/**
 * Which equipped items sit in which slot.
 *
 * Derived rather than stored, bar one: the server keeps one equipped list and
 * the slug in its Custom slot. That one is seated first; the rest go by their
 * own shape, and a second held item takes the off hand. Nothing is hidden: a
 * slot worn past its capacity lists every occupant, and an item with no slot
 * at all is listed under `ELSEWHERE`, which the panel draws only when
 * something is there.
 */
export function slotted(equipment: EquipmentLike, items: ReadonlyMap<string, ItemLike>): Map<Slot | typeof ELSEWHERE, string[]> {
  const bySlot = new Map<Slot | typeof ELSEWHERE, string[]>()
  for (const { slot } of SLOTS) bySlot.set(slot, [])
  bySlot.set(ELSEWHERE, [])
  const room = (slot: Slot) => bySlot.get(slot)!.length < capacityOf(slot)
  const worn = equippedSlugs(equipment)
  const custom = equipment.custom === undefined ? -1 : worn.indexOf(equipment.custom)
  if (custom >= 0) bySlot.get(CUSTOM)!.push(...worn.splice(custom, 1))
  for (const slug of worn) {
    const own = slotOf(items.get(slug))
    const slot = own === null ? ELSEWHERE : own === 'main-hand' && !room(own) && room('off-hand') ? 'off-hand' : own
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

/** What the Custom slot holds: one slug, or none. */
const setCustom = (slug?: string): EquipmentChange => ({
  path: 'equipment.custom',
  op: 'set',
  value: { kind: 'slugs', slugs: slug === undefined ? [] : [slug] },
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

/**
 * Moves one `slug` from the backpack into `slot`, sending a full slot's first
 * occupant back. Into Custom, it also records the placement, which is the
 * only one the server does not derive.
 */
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
  let carried = count(equipment.backpack, slug)
  const displaced = occupants.length >= capacityOf(slot) ? occupants[0] : undefined
  if (displaced !== undefined) {
    after.splice(after.indexOf(displaced), 1)
    if (displaced === slug) carried += 1
    else changes.push(setCount('backpack', displaced, count(equipment.backpack, displaced) + 1))
  }
  after.push(slug)
  if (slot === CUSTOM) changes.push(setCustom(slug))
  // Callers offer only what the backpack holds; the floor is for a stale click.
  return [...equippedChanges(before, after), setCount('backpack', slug, Math.max(carried - 1, 0)), ...changes]
}

/**
 * Takes one `slug` off and puts it in the backpack. Out of Custom, the slot
 * is cleared too; anywhere else the server forgets a Custom occupant on its
 * own once the last of it is off.
 */
export function unequip(equipment: EquipmentLike, slug: string, from?: Slot): EquipmentChange[] {
  const before = equippedSlugs(equipment)
  const at = before.indexOf(slug)
  if (at < 0) return []
  const after = [...before]
  after.splice(at, 1)
  return [
    ...equippedChanges(before, after),
    setCount('backpack', slug, count(equipment.backpack, slug) + 1),
    ...(from === CUSTOM && equipment.custom === slug ? [setCustom()] : []),
  ]
}

/** Drops one worn `slug`: taken off, and not put in the backpack. */
export function discard(equipment: EquipmentLike, slug: string, from?: Slot): EquipmentChange[] {
  return unequip(equipment, slug, from).filter((change) => !change.path.startsWith('equipment.backpack.'))
}

/**
 * The slots a row's menu offers for an item: its own, Custom, and the off hand
 * for a held thing -- but only beside an occupied main hand, because seats are
 * derived and a lone held item is in the main hand whatever it was asked for.
 */
export function slotsFor(equipment: EquipmentLike, items: ReadonlyMap<string, ItemLike>, item: ItemLike | undefined): Slot[] {
  const own = slotOf(item)
  if (own === null) return []
  const held = own === 'main-hand' && (slotted(equipment, items).get('main-hand') ?? []).length > 0
  return [own, ...(held ? ['off-hand' as const] : []), CUSTOM]
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
