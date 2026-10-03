import type { Equipment } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Group, Stack, Text } from '@/ui'
import { refName } from './refNames'

/** Fixed grants and confirmed choices, read from the projected inventory. */
export function EquipmentSummary({ equipment, names }: {
  equipment: Equipment
  names: ReadonlyMap<string, string>
}) {
  const t = useT()
  const items = [...equipment.equipped, ...equipment.backpack, ...equipment.loot]
  const coins = {
    cp: t('equipment.coin.cp'), sp: t('equipment.coin.sp'), ep: t('equipment.coin.ep'),
    gp: t('equipment.coin.gp'), pp: t('equipment.coin.pp'),
  }
  return <Stack component="section" aria-label={t('build.currentEquipment')} gap="xs">
    <Text fw={600}>{t('build.currentEquipment')}</Text>
    {items.length === 0 && <Text size="sm" c="dimmed">{t('sheet.empty')}</Text>}
    {items.map((item, index) => <Group key={`${item.item ?? 'custom'}:${index}`} justify="space-between">
      <Text size="sm">{item.custom?.name ?? refName(`item:${item.item ?? ''}`, names)}</Text>
      <Text size="sm">×{item.count}</Text>
    </Group>)}
    <Group gap="sm">
      {Object.entries(equipment.purse ?? {}).filter(([, amount]) => amount !== 0).map(([unit, amount]) => (
        <Text key={unit} size="sm">{amount} {coins[unit as keyof typeof coins] ?? unit}</Text>
      ))}
    </Group>
  </Stack>
}
