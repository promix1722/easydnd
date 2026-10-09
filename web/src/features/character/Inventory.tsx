import { COINS, equip, groupOf, setTotal, slotOf, unequip } from '@/domain'
import type { InventoryRow } from '@/domain'
import type { Change, Equipment, Item } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { ACTION_ICON_SIZE, ActionIcon, Badge, Group, IconDotsVertical, ItemIcon, Menu, NumberInput, Paper, SourceTags, Stack, Text } from '@/ui'

import { itemFacts } from './options'

/**
 * Inventory rows, one bubble per entity: the name, its numbers on one line,
 * and where it was published. Everything a row has to say is on the row;
 * nothing opens.
 *
 * With `onChange`, every row has a menu on the right: a wearable is worn or
 * taken off, a consumable is used (one fewer, nothing else yet), and anything
 * is dropped -- one or all of it, when there is more than one. A count is
 * printed only when it says something: one dagger is "Dagger".
 */
export function InventoryRows({ rows, equipment, items, name, lookup, empty, disabled = false, onChange }: {
  rows: readonly InventoryRow[]
  equipment: Equipment
  items: ReadonlyMap<string, Item>
  name: (row: InventoryRow) => string
  /** The catalogue's word for a damage type or a weapon property. */
  lookup: (collection: string, slug: string) => string
  empty: string
  disabled?: boolean
  onChange?: (changes: Change[]) => void
}) {
  const t = useT()
  if (rows.length === 0) return <Text size="sm" c="dimmed">{empty}</Text>

  return <Stack gap={6}>
    {rows.map((row) => {
      const label = name(row)
      const item = row.item === undefined ? undefined : items.get(row.item)
      // A custom item has no slug to address a change to; it is edited where
      // it was written.
      const editable = row.item !== undefined && onChange !== undefined
      const slot = slotOf(item)
      const group = groupOf(item)
      const line = item === undefined ? undefined : itemFacts(t, item, (slug) =>
        lookup(item.weapon?.properties?.includes(slug) ? 'weapon-properties' : 'damage-types', slug))
      const total = (count: number) => onChange?.(setTotal(equipment, row.item ?? '', count))
      return <Paper key={row.key} withBorder radius="md" p="xs">
        <Group justify="space-between" wrap="nowrap" gap="sm" align="flex-start">
          <ItemIcon icon={item?.icon} />
          <Stack gap={2} style={{ minWidth: 0, flex: 1 }}>
            <Group gap={8}>
              <Text size="sm" fw={500}>{label}</Text>
              {row.count > 1 && <Text size="sm" c="dimmed">×{row.count}</Text>}
              {row.equipped > 0 && <Badge size="xs" variant="light">{t('equipment.equippedMark')}</Badge>}
            </Group>
            {line !== undefined && <Text size="xs" c="dimmed">{line}</Text>}
            <SourceTags provenance={item?.provenance} oneLine />
          </Stack>
          <Group gap={6} wrap="nowrap" style={{ flexShrink: 0 }}>
            {editable && (
              <Menu position="bottom-end">
                <Menu.Target>
                  {/* Its own padding pulled back, so the dots end where the badges do. */}
                  <ActionIcon variant="subtle" color="gray" mr={-6} aria-label={t('list.actions', { name: label })} disabled={disabled}>
                    <IconDotsVertical size={ACTION_ICON_SIZE} />
                  </ActionIcon>
                </Menu.Target>
                <Menu.Dropdown>
                  {slot !== null && row.count > row.equipped && <Menu.Item onClick={() => onChange(equip(equipment, items, row.item ?? '', slot))}>{t('equipment.wear')}</Menu.Item>}
                  {slot !== null && row.equipped > 0 && <Menu.Item onClick={() => onChange(unequip(equipment, row.item ?? ''))}>{t('equipment.takeOffNamed', { name: label })}</Menu.Item>}
                  {group === 'consumable' && <Menu.Item onClick={() => total(row.count - 1)}>{t('equipment.use')}</Menu.Item>}
                  {row.count > 1
                    ? <>
                      <Menu.Item color="red" onClick={() => total(row.count - 1)}>{t('equipment.dropOne')}</Menu.Item>
                      <Menu.Item color="red" onClick={() => total(0)}>{t('equipment.dropAll')}</Menu.Item>
                    </>
                    : <Menu.Item color="red" onClick={() => total(0)}>{t('equipment.drop')}</Menu.Item>}
                </Menu.Dropdown>
              </Menu>
            )}
          </Group>
        </Group>
      </Paper>
    })}

  </Stack>
}

/** The coins a character carries: text to read, or five fields to edit. */
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
        <Text key={unit} size="sm">{amount} {coins[unit] ?? unit}</Text>
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
