import type { ReactNode } from 'react'

import { COINS, equip, groupOf, setTotal, slotsFor, slotted } from '@/domain'
import type { InventoryRow, Slot } from '@/domain'
import type { Change, Equipment, Item, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { ACTION_ICON_SIZE, ActionIcon, Group, IconDotsVertical, ItemIcon, Menu, NumberInput, Paper, SourceTags, Stack, Text, useIsDesktop } from '@/ui'

import { itemFacts, weaponNumbers } from './options'
import { WeaponStats } from './WeaponStats'
import { useSlotLabels } from './slotLabels'

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
      const line = item === undefined ? undefined : itemFacts(t, item, word, numbers !== undefined)
      // Totals count the worn units too, and `setTotal` takes from the backpack first.
      const total = (all: number) => onChange?.(setTotal(equipment, row.item ?? '', all))
      // In the top right corner where there is room; on a phone that corner is
      // the name's, so the badges go under the text.
      const tags = <SourceTags provenance={item?.provenance} oneLine />
      return <Paper key={row.key} withBorder radius="md" p="xs">
        <Group justify="space-between" wrap="nowrap" gap="sm" align="flex-start">
          <ItemIcon icon={item?.icon} />
          <Stack gap={2} style={{ minWidth: 0, flex: 1 }}>
            <Group gap={8}>
              <Text size="sm" fw={500}>{label}</Text>
              {count > 1 && <Text size="sm" c="dimmed">×{count}</Text>}
            </Group>
            {!isDesktop && numbers !== undefined && <WeaponStats inline {...numbers} />}
            {line !== undefined && <Text size="xs" c="dimmed">{line}</Text>}
            {!isDesktop && tags}
          </Stack>
          {/* Columns where there is room for them; on a phone they are a line under the name, like the badges. */}
          {isDesktop && numbers !== undefined && <WeaponStats {...numbers} />}
          <Group gap={6} wrap="nowrap" style={{ flexShrink: 0 }}>
            {isDesktop && tags}
            {editable && (
            <ItemMenu name={label} disabled={disabled}>
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
            </ItemMenu>
            )}
          </Group>
        </Group>
      </Paper>
    })}
  </Stack>
}

/** The coins a character carries, by their full names: text to read, or five fields to edit. */
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
  if (onChange === undefined) {
    return <Group gap="sm">
      {Object.entries(purse ?? {}).filter(([, amount]) => amount !== 0).map(([unit, amount]) => (
        <Text key={unit} size="sm">{coins[unit] ?? unit}: {amount}</Text>
      ))}
    </Group>
  }
  return <Group gap="xs">
    {COINS.map((unit) => (
      <NumberInput key={`${unit}:${purse?.[unit] ?? 0}`} w={84} size="xs" min={0} allowDecimal={false} disabled={disabled}
        label={coins[unit]} defaultValue={purse?.[unit] ?? 0}
        // Written when the field is left, not per keystroke: every write is an
        // entry in the character's log.
        onBlur={(event) => {
          const amount = Number(event.currentTarget.value)
          if (Number.isInteger(amount) && amount >= 0 && amount !== (purse?.[unit] ?? 0)) onChange(unit, amount)
        }} />
    ))}
  </Group>
}
