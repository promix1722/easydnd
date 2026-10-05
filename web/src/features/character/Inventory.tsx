import { COINS } from '@/domain'
import type { InventoryRow } from '@/domain'
import { useT } from '@/lib/i18n'
import { ActionIcon, Badge, Group, NumberInput, Stack, Text } from '@/ui'

/**
 * Inventory rows, one per entity. Read-only without the callbacks -- which is
 * how a table reads a sheet shared with it.
 *
 * A count is printed only when it says something: one dagger is "Dagger".
 */
export function InventoryRows({ rows, name, empty, disabled = false, onTotal }: {
  rows: readonly InventoryRow[]
  name: (row: InventoryRow) => string
  empty: string
  disabled?: boolean
  onTotal?: (row: InventoryRow, total: number) => void
}) {
  const t = useT()
  if (rows.length === 0) return <Text size="sm" c="dimmed">{empty}</Text>
  return <Stack gap={6}>
    {rows.map((row) => {
      const label = name(row)
      // A custom item has no slug to address a change to; it is edited where
      // it was written.
      const editable = row.item !== undefined
      return <Group key={row.key} justify="space-between" wrap="nowrap" gap="sm">
        <Group gap={8} wrap="nowrap" style={{ minWidth: 0 }}>
          <Text size="sm">{label}</Text>
          {row.equipped > 0 && <Badge size="xs" variant="light">{t('equipment.equippedMark')}</Badge>}
        </Group>
        <Group gap={6} wrap="nowrap">
          {editable && onTotal && (
            <ActionIcon variant="default" size="sm" aria-label={t('equipment.fewer', { name: label })}
              disabled={disabled} onClick={() => onTotal(row, row.count - 1)}>−</ActionIcon>
          )}
          {(row.count > 1 || (editable && onTotal)) && <Text size="sm" miw={28} ta="center">{onTotal ? row.count : `×${row.count}`}</Text>}
          {editable && onTotal && (
            <ActionIcon variant="default" size="sm" aria-label={t('equipment.more', { name: label })}
              disabled={disabled} onClick={() => onTotal(row, row.count + 1)}>+</ActionIcon>
          )}
        </Group>
      </Group>
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
