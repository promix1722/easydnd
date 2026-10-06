import { useState } from 'react'

import { ITEM_GROUPS, SLOTS, equip, fitsSlot, groupOf, mergeStacks, setCoin, setTotal, slotted, unequip } from '@/domain'
import type { InventoryRow, ItemGroup, Slot } from '@/domain'
import type { Change, Equipment, Item } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, ModalSheet, Panel, Paper, SimpleGrid, Stack, TabRow, Text, UnstyledButton } from '@/ui'

import { InventoryRows, Purse } from './Inventory'

/**
 * The sheet's Equipment tab: what is worn where, then everything owned in
 * three groups.
 *
 * Read-only without `onChange`, which is how a sheet shared with a table is
 * drawn. With it, a slot is a button that offers what in the backpack fits.
 */
export function SheetEquipment({ equipment, items, name, disabled = false, onChange }: {
  equipment: Equipment
  items: ReadonlyMap<string, Item>
  name: (slug: string) => string
  disabled?: boolean
  onChange?: (changes: Change[]) => void
}) {
  const t = useT()
  const [group, setGroup] = useState<ItemGroup>('wearable')
  const [picking, setPicking] = useState<Slot | null>(null)
  const slots: Record<Slot, string> = {
    armor: t('equipment.slot.armor'), mainHand: t('equipment.slot.mainHand'), offHand: t('equipment.slot.offHand'),
    neck: t('equipment.slot.neck'), ring: t('equipment.slot.ring'), worn: t('equipment.slot.worn'),
  }
  const groups: Record<ItemGroup, string> = {
    wearable: t('equipment.group.wearable'), consumable: t('equipment.group.consumable'), gear: t('equipment.group.gear'),
  }
  const bySlot = slotted(equipment, items)
  const rows = mergeStacks(equipment)
  const rowName = (row: InventoryRow) => row.customName ?? name(row.item ?? '')
  const fitting = picking === null ? [] : equipment.backpack.filter((stack, at, all) =>
    stack.item !== undefined && fitsSlot(items.get(stack.item), picking) &&
    all.findIndex((other) => other.item === stack.item) === at)

  return <Stack gap="md">
    <SimpleGrid component="section" aria-label={t('equipment.slots')} cols={{ base: 2, sm: 3 }} spacing="xs">
      {SLOTS.map(({ slot }) => {
        const worn = bySlot.get(slot) ?? []
        const body = <Stack gap={2}>
          <Text size="xs" c="dimmed" tt="uppercase">{slots[slot]}</Text>
          {worn.length === 0
            ? <Text size="sm" c="dimmed">{t('equipment.slotEmpty')}</Text>
            : worn.map((slug, at) => <Text key={`${slug}:${at}`} size="sm" fw={500}>{name(slug)}</Text>)}
        </Stack>
        return <Paper key={slot} withBorder p="xs" radius="md">
          {onChange
            ? <UnstyledButton w="100%" disabled={disabled} aria-label={slots[slot]} onClick={() => setPicking(slot)}>{body}</UnstyledButton>
            : body}
        </Paper>
      })}
    </SimpleGrid>

    <Panel>
      <Stack gap="md">
        <TabRow tabs={ITEM_GROUPS.map((each) => ({ value: each, label: groups[each] }))} value={group}
          onChange={(next) => setGroup(next as ItemGroup)}>
          <InventoryRows
            rows={rows.filter((row) => groupOf(items.get(row.item ?? '')) === group)}
            name={rowName}
            empty={t('sheet.empty')}
            disabled={disabled}
            {...(onChange ? { onTotal: (row: InventoryRow, total: number) => onChange(setTotal(equipment, row.item ?? '', total)) } : {})}
          />
        </TabRow>
        <Purse purse={equipment.purse} disabled={disabled}
          {...(onChange ? { onChange: (unit: string, amount: number) => onChange([setCoin(unit, amount)]) } : {})} />
      </Stack>
    </Panel>

    {onChange && <ModalSheet opened={picking !== null} onClose={() => setPicking(null)} title={picking === null ? '' : slots[picking]}>
      {picking !== null && <Stack gap="xs">
        {(bySlot.get(picking) ?? []).map((slug, at) => (
          <Button key={`${slug}:${at}`} variant="default" justify="space-between"
            onClick={() => { onChange(unequip(equipment, slug)); setPicking(null) }}>
            {t('equipment.takeOffNamed', { name: name(slug) })}
          </Button>
        ))}
        {fitting.length === 0 && <Text size="sm" c="dimmed">{t('equipment.nothingFits')}</Text>}
        {fitting.map((stack) => (
          <Button key={stack.item} variant="light" justify="space-between"
            onClick={() => { onChange(equip(equipment, items, stack.item ?? '', picking)); setPicking(null) }}>
            {name(stack.item ?? '')}
          </Button>
        ))}
      </Stack>}
    </ModalSheet>}
  </Stack>
}
