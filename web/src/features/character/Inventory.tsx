import { Fragment } from 'react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { COINS, equip, groupOf, setTotal, slotsFor, slotted } from '@/domain'
import type { InventoryRow, Slot } from '@/domain'
import type { Change, Equipment, Item, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { ACTION_ICON_SIZE, ActionIcon, Box, Group, IconDotsVertical, ITEM_ICON_SIZE, ItemIcon, Markdown, Menu, NumberInput, Paper, SimpleGrid, SourceTags, Stack, Text, useIsDesktop } from '@/ui'

import { armorStats, itemFactList, itemFacts, weaponNumbers } from './options'
import type { Stat } from './options'
import { StatColumns, WeaponStats } from './WeaponStats'
import { useSlotLabels } from './slotLabels'

/**
 * The menu entry that opens an item's own page. A relative link, so it lands
 * under whichever sheet is showing -- the owner's or one shared with a table.
 */
export function ItemDetails({ slug }: { slug: string }) {
  const t = useT()
  return <Menu.Item component={Link} to={`items/${encodeURIComponent(slug)}`}>{t('item.details')}</Menu.Item>
}

const NO_ACTIONS: readonly SheetAction[] = []

/** The line a card gives an item's name, what is beside it and its menu, and the gap under it. */
export const NAME_LINE = 34

const RowStack = ({ children }: { children: ReactNode }) => <Stack gap={6}>{children}</Stack>
const CardGrid = ({ children }: { children: ReactNode }) => <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="xs">{children}</SimpleGrid>

/**
 * An item as a card: its name, a tag and its menu on one line, over one
 * icon's height of columns -- a weapon's three numbers, or armor's class and
 * the Strength it asks, with a disadvantage on Stealth as a line beneath.
 * What it is worn as and what could be worn are drawn by this one, so a
 * dagger looks the same in the hand and in the pack. Its other facts are on
 * its page; anything that is neither weapon nor armor says what it is in a
 * line, and nothing here says what it weighs.
 */
export function ItemCard({ item, label, count = 1, tag, menu, actions = NO_ACTIONS, lookup }: {
  item: Item | undefined
  label: string
  count?: number
  /** Beside the name: the slot a worn item is in. */
  tag?: ReactNode
  menu?: ReactNode
  /** The sheet's actions, for an item that is worn: a wielded weapon shows the numbers its action has. */
  actions?: readonly SheetAction[]
  lookup: (collection: string, slug: string) => string
}) {
  const t = useT()
  const word = (ref: string) => lookup(item?.weapon?.properties?.includes(ref) ? 'weapon-properties' : 'damage-types', ref)
  const numbers = weaponNumbers(t, item, actions, word)
  const armor = numbers === undefined ? armorStats(t, item) : undefined
  const facts = item === undefined || numbers !== undefined || armor !== undefined ? undefined : itemFacts(t, item, word, false, false)
  return <Stack gap={6}>
    <Group gap={6} wrap="nowrap" mih={NAME_LINE - 6}>
      <Group gap={8} wrap="nowrap" style={{ minWidth: 0, flex: 1 }}>
        <Text size="sm" fw={600} truncate>{label}</Text>
        {count > 1 && <Text size="sm" c="dimmed" style={{ flexShrink: 0 }}>×{count}</Text>}
      </Group>
      {tag}
      {menu}
    </Group>
    <Group gap="sm" wrap="nowrap" mih={ITEM_ICON_SIZE}>
      <ItemIcon icon={item?.icon} />
      <Stack gap={2} style={{ minWidth: 0, flex: 1, overflowWrap: 'anywhere' }}>
        {numbers !== undefined && <WeaponStats wrap {...numbers} />}
        {armor !== undefined && <StatColumns wrap stats={armor} />}
        {item?.armor?.stealthDisadvantage && <Text size="xs" c="orange.8">{t('equipment.stealth')}</Text>}
        {facts !== undefined && <Text size="xs" c="dimmed" lineClamp={2}>{facts}</Text>}
        {!!item?.desc?.length && <Text component="div" size="xs" lineClamp={1}><Markdown size="xs" inline>{item.desc[0] ?? ''}</Markdown></Text>}
      </Stack>
    </Group>
  </Stack>
}

/** The three dots on the right of a row or a worn item, and what they open. */
export function ItemMenu({ name, disabled = false, children }: { name: string; disabled?: boolean; children: ReactNode }) {
  const t = useT()
  return <Menu position="bottom-end">
    <Menu.Target>
      {/* Its own padding pulled back, so the dots end where the badges do. */}
      <ActionIcon variant="subtle" color="gray" mr={-6} aria-label={t('list.actions', { name })} disabled={disabled} style={{ flexShrink: 0 }}>
        <IconDotsVertical size={ACTION_ICON_SIZE} />
      </ActionIcon>
    </Menu.Target>
    <Menu.Dropdown>{children}</Menu.Dropdown>
  </Menu>
}

/**
 * Inventory rows, one bubble per entity: the name, its numbers on one line,
 * and where it was published. Everything a row has to say is on the row;
 * nothing opens.
 *
 * Only what is carried: a worn item is on its slot's card and nowhere else, so
 * a row counts the units that are not on the character and is not drawn when
 * there are none.
 *
 * With `onChange`, every row has a menu on the right: a wearable is put on,
 * with one entry per slot it could go in; a consumable is used (one fewer,
 * nothing else yet), and anything is dropped -- one or all of it, when there
 * is more than one. A count is printed only when it says something: one
 * dagger is "Dagger".
 */
export function InventoryRows({ rows, equipment, items, name, lookup, empty, disabled = false, cards = false, onChange }: {
  rows: readonly InventoryRow[]
  equipment: Equipment
  items: ReadonlyMap<string, Item>
  name: (row: InventoryRow) => string
  /** The catalogue's word for a damage type or a weapon property. */
  lookup: (collection: string, slug: string) => string
  empty: string
  disabled?: boolean
  /** Drawn as the cards worn items are, three across: the Equipment tab, where what could be worn sits under what is. */
  cards?: boolean
  onChange?: (changes: Change[]) => void
}) {
  const t = useT()
  const labels = useSlotLabels()
  const isDesktop = useIsDesktop()
  const carried = rows.filter((row) => row.count > row.equipped)
  if (carried.length === 0) return <Text size="sm" c="dimmed">{empty}</Text>
  // A new ring is the last one on, so it is drawn on the second card unless it is the only one.
  const rings = slotted(equipment, items).get('ring')?.length ?? 0
  const slotLabel = (slot: Slot) => labels[slot === 'ring' ? (rings === 0 ? 'ring:0' : 'ring:1') : slot]

  const Rows = cards ? CardGrid : RowStack
  return <Rows>
    {carried.map((row) => {
      const label = name(row)
      const item = row.item === undefined ? undefined : items.get(row.item)
      const count = row.count - row.equipped
      // A custom item has no slug to address a change to; it is edited where
      // it was written.
      const editable = row.item !== undefined && onChange !== undefined
      const group = groupOf(item)
      const word = (slug: string) => lookup(item?.weapon?.properties?.includes(slug) ? 'weapon-properties' : 'damage-types', slug)
      // What is carried shows what the catalogue says of any such weapon,
      // and no bonus to hit: an action is the attack of the one in the hand,
      // and a second dagger in the pack used to borrow it by its slug, so
      // one carried weapon had a bonus and the rapier beside it had none.
      const numbers = weaponNumbers(t, item, NO_ACTIONS, word)
      // A fact under its own caption, not a sentence of them: "Thrown range:
      // 20/60 ft. · Finesse, Light · Weight: 1 lb." was three kinds of thing
      // in one grey line. What the row draws as columns is not said twice,
      // and nothing carried says what it cost.
      const facts = item === undefined ? [] : itemFactList(t, item, word)
        .filter((fact) => fact.key !== 'cost' && !(numbers !== undefined && (fact.key === 'damage' || fact.key === 'range')))
      const properties = facts.find((fact) => fact.key === 'properties')
      const pairs = facts.filter((fact) => fact !== properties)
      // On a phone there is no room for columns, so the weapon's numbers lead
      // one list with everything else: captions down the left, values in line.
      const listed: Stat[] = [
        ...(numbers?.damage ? [{ key: 'damage', label: t('weapon.damage'), value: numbers.damage, color: 'red' }] : []),
        ...(numbers?.hit ? [{ key: 'hit', label: t('weapon.hit'), value: numbers.hit, color: 'violet' }] : []),
        ...(numbers?.range ? [{ key: 'range', label: t('weapon.range'), value: numbers.range }] : []),
        ...facts,
      ]
      // Totals count the worn units too, and `setTotal` takes from the backpack first.
      const total = (all: number) => onChange?.(setTotal(equipment, row.item ?? '', all))
      // In the bottom right corner where there is room, under the numbers and
      // the menu: beside them, numbers of different widths left the badges
      // at a different place on every row. A phone's row does not say which
      // book a dagger is from; its page does.
      const tags = <SourceTags provenance={item?.provenance} oneLine />
      const menu = row.item === undefined ? undefined : (
            <ItemMenu name={label}>
              <ItemDetails slug={row.item} />
              {editable && !disabled && <>
              {slotsFor(equipment, items, item).map((slot) => (
                <Menu.Item key={slot} onClick={() => onChange(equip(equipment, items, row.item ?? '', slot))}>
                  {t('equipment.wearIn', { slot: slotLabel(slot) })}
                </Menu.Item>
              ))}
              {group === 'consumable' && <Menu.Item onClick={() => total(row.count - 1)}>{t('equipment.use')}</Menu.Item>}
              {count > 1
                ? <>
                  <Menu.Item color="red" onClick={() => total(row.count - 1)}>{t('equipment.dropOne')}</Menu.Item>
                  <Menu.Item color="red" onClick={() => total(row.equipped)}>{t('equipment.dropAll')}</Menu.Item>
                </>
                : <Menu.Item color="red" onClick={() => total(row.equipped)}>{t('equipment.drop')}</Menu.Item>}
              </>}
            </ItemMenu>
      )
      if (cards) return <Paper key={row.key} withBorder radius="md" p="xs">
        <ItemCard item={item} label={label} count={count} menu={menu} lookup={lookup} />
      </Paper>
      return <Paper key={row.key} withBorder radius="md" p="xs">
        <Group justify="space-between" wrap="nowrap" gap="sm" align="flex-start">
          <ItemIcon icon={item?.icon} />
          <Stack gap={2} style={{ minWidth: 0, flex: 1 }}>
            <Group gap={8}>
              <Text size="sm" fw={500}>{label}</Text>
              {count > 1 && <Text size="sm" c="dimmed">×{count}</Text>}
            </Group>
            {isDesktop && properties !== undefined && <Text size="xs">{properties.value}</Text>}
            {isDesktop && pairs.length > 0 && <Group gap="md" style={{ rowGap: 0 }}>
              {pairs.map((fact) => <Text key={fact.key} size="xs" c="dimmed">{fact.label} <Text span fw={600} c="var(--mantine-color-text)">{fact.value}</Text></Text>)}
            </Group>}
            {!isDesktop && listed.length > 0 && <Box style={{ display: 'grid', gridTemplateColumns: 'max-content minmax(0, 1fr)', columnGap: 8, rowGap: 2 }}>
              {listed.map((fact) => <Fragment key={fact.key}>
                <Text size="xs" c="dimmed">{fact.label}</Text>
                <Text size="xs" {...(fact.color ? { c: fact.color, fw: 700 } : {})}>{fact.value}</Text>
              </Fragment>)}
            </Box>}
          </Stack>
          <Stack gap={4} align="flex-end" justify="space-between" mih={ITEM_ICON_SIZE} style={{ flexShrink: 0 }}>
          <Group gap="sm" wrap="nowrap" align="flex-start">
            {/* Columns where there is room for them; on a phone they lead the list under the name. */}
            {isDesktop && numbers !== undefined && <WeaponStats {...numbers} />}
            {menu}
          </Group>
          {isDesktop && tags}
          </Stack>
        </Group>
      </Paper>
    })}
  </Rows>
}

/**
 * The coins a character carries, by their full names: five fields, the same
 * for a reader as for the owner, and locked without `onChange`.
 */
export function Purse({ purse, disabled = false, onChange }: {
  purse: Record<string, number> | undefined
  disabled?: boolean
  onChange?: (unit: string, amount: number) => void
}) {
  const t = useT()
  const coins: Record<string, string> = {
    cp: t('equipment.coin.cp'), sp: t('equipment.coin.sp'), ep: t('equipment.coin.ep'),
    gp: t('equipment.coin.gp'), pp: t('equipment.coin.pp'),
  }
  return <Group gap="xs">
    {COINS.map((unit) => (
      <NumberInput key={`${unit}:${purse?.[unit] ?? 0}`} w={84} size="xs" min={0} allowDecimal={false} disabled={disabled}
        readOnly={onChange === undefined} label={coins[unit]} defaultValue={purse?.[unit] ?? 0}
        // Written when the field is left, not per keystroke: every write is an
        // entry in the character's log.
        onBlur={(event) => {
          const amount = Number(event.currentTarget.value)
          if (Number.isInteger(amount) && amount >= 0 && amount !== (purse?.[unit] ?? 0)) onChange?.(unit, amount)
        }} />
    ))}
  </Group>
}
