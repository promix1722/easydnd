import { Fragment } from 'react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { COINS, equip, groupOf, setTotal, slotsFor, slotted } from '@/domain'
import type { InventoryRow, Slot } from '@/domain'
import type { Change, Equipment, Item, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { ACTION_ICON_SIZE, ActionIcon, Box, Group, IconDotsVertical, ITEM_ICON_SIZE, ItemIcon, Menu, NumberInput, Paper, SourceTags, Stack, Text, useIsDesktop } from '@/ui'

import { itemFactList, weaponNumbers } from './options'
import type { Stat } from './options'
import { WeaponStats } from './WeaponStats'
import { useSlotLabels } from './slotLabels'

/**
 * The menu entry that opens an item's own page. A relative link, so it lands
 * under whichever sheet is showing -- the owner's or one shared with a table.
 */
export function ItemDetails({ slug }: { slug: string }) {
  const t = useT()
  return <Menu.Item component={Link} to={`items/${encodeURIComponent(slug)}`}>{t('item.details')}</Menu.Item>
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
export function InventoryRows({ rows, equipment, items, name, lookup, empty, actions = [], disabled = false, onChange }: {
  rows: readonly InventoryRow[]
  equipment: Equipment
  items: ReadonlyMap<string, Item>
  name: (row: InventoryRow) => string
  /** The catalogue's word for a damage type or a weapon property. */
  lookup: (collection: string, slug: string) => string
  empty: string
  /** The sheet's actions, so a wielded weapon shows the numbers its action has. */
  actions?: readonly SheetAction[]
  disabled?: boolean
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

  return <Stack gap={6}>
    {carried.map((row) => {
      const label = name(row)
      const item = row.item === undefined ? undefined : items.get(row.item)
      const count = row.count - row.equipped
      // A custom item has no slug to address a change to; it is edited where
      // it was written.
      const editable = row.item !== undefined && onChange !== undefined
      const group = groupOf(item)
      const word = (slug: string) => lookup(item?.weapon?.properties?.includes(slug) ? 'weapon-properties' : 'damage-types', slug)
      const numbers = weaponNumbers(t, item, actions, word)
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
            {row.item !== undefined && (
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
          )}
          </Group>
          {isDesktop && tags}
          </Stack>
        </Group>
      </Paper>
    })}
  </Stack>
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
