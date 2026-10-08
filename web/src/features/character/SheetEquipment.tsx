import { useState } from 'react'

import { CUSTOM, ELSEWHERE, equip, fitsSlot, groupOf, mergeStacks, setCoin, slotted, unequip } from '@/domain'
import type { InventoryRow, ItemGroup, Slot } from '@/domain'
import type { Change, Equipment, Item } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Box, Button, Grid, ModalSheet, Panel, Paper, Stack, Text, UnstyledButton } from '@/ui'

import { InventoryRows, Purse } from './Inventory'

/**
 * A card on the paperdoll. One per slot, except the ring slot, which holds
 * two and is drawn as two cards; and `elsewhere`, drawn only when something
 * equipped has no slot to be shown in.
 */
type Card = Exclude<Slot, 'ring'> | 'ring:0' | 'ring:1' | typeof ELSEWHERE

/** Three columns: what is held, what is worn down the middle, what hangs or is slipped on. */
const COLUMNS: readonly (readonly Card[])[] = [
  ['main-hand', 'off-hand', 'arms', CUSTOM],
  ['head', 'body', 'waist', 'feet'],
  ['back', 'neck', 'ring:0', 'ring:1'],
]

const slotOfCard = (card: Card): Slot | typeof ELSEWHERE => card === 'ring:0' || card === 'ring:1' ? 'ring' : card

/** What both tabs are drawn from. Read-only without `onChange`, which is how a sheet shared with a table is drawn. */
interface InventoryProps {
  equipment: Equipment
  items: ReadonlyMap<string, Item>
  name: (slug: string) => string
  lookup: (collection: string, slug: string) => string
  disabled?: boolean
  onChange?: (changes: Change[]) => void
}

const rowName = (name: (slug: string) => string) => (row: InventoryRow) => row.customName ?? name(row.item ?? '')

/**
 * The sheet's Equipment tab: what is worn where, then everything that could be.
 *
 * With `onChange`, a card is a button that offers what in the backpack fits
 * -- Custom, any wearable.
 */
export function SheetEquipment({ equipment, items, name, lookup, disabled = false, onChange }: InventoryProps) {
  const t = useT()
  const [picking, setPicking] = useState<Card | null>(null)
  const labels: Record<Card, string> = {
    head: t('equipment.slot.head'), neck: t('equipment.slot.neck'), back: t('equipment.slot.back'), body: t('equipment.slot.body'),
    arms: t('equipment.slot.arms'), waist: t('equipment.slot.waist'), feet: t('equipment.slot.feet'),
    'main-hand': t('equipment.slot.main-hand'), 'off-hand': t('equipment.slot.off-hand'),
    'ring:0': t('equipment.slot.ring1'), 'ring:1': t('equipment.slot.ring2'),
    custom: t('equipment.slot.custom'), elsewhere: t('equipment.slot.elsewhere'),
  }
  const bySlot = slotted(equipment, items)
  // A ring card shows its own ring; every other card its whole slot, so that
  // a slot worn past its capacity still lists every occupant.
  const occupants = (card: Card): string[] => {
    const worn = bySlot.get(slotOfCard(card)) ?? []
    if (card === 'ring:0') return worn.slice(0, 1)
    if (card === 'ring:1') return worn.slice(1, 2)
    return worn
  }
  const rows = mergeStacks(equipment).filter((row) => groupOf(items.get(row.item ?? '')) === 'wearable')
  // Nothing fits "elsewhere": it only ever holds what is already there.
  const slot = picking === null ? null : slotOfCard(picking)
  const fitting = slot === null || slot === ELSEWHERE ? [] : equipment.backpack.filter((stack, at, all) =>
    stack.item !== undefined && fitsSlot(items.get(stack.item), slot) &&
    all.findIndex((other) => other.item === stack.item) === at)

  const card = (each: Card) => {
    const worn = occupants(each)
    const body = <Stack gap={2}>
      <Text size="xs" c="dimmed" tt="uppercase">{labels[each]}</Text>
      {worn.length === 0
        ? <Text size="sm" c="dimmed">{t('equipment.slotEmpty')}</Text>
        : worn.map((slug, at) => <Text key={`${slug}:${at}`} size="sm" fw={500}>{name(slug)}</Text>)}
    </Stack>
    return <Paper key={each} withBorder p="xs" radius="md">
      {onChange
        ? <UnstyledButton w="100%" disabled={disabled} aria-label={labels[each]} onClick={() => setPicking(each)}>{body}</UnstyledButton>
        : body}
    </Paper>
  }
  const elsewhere = bySlot.get(ELSEWHERE) ?? []

  return <Stack gap="md">
    <Box component="section" aria-label={t('equipment.slots')}>
      <Grid gap="xs">
        {COLUMNS.map((column, at) => <Grid.Col key={at} span={4}><Stack gap="xs">{column.map(card)}</Stack></Grid.Col>)}
        {elsewhere.length > 0 && <Grid.Col span={12}>{card(ELSEWHERE)}</Grid.Col>}
      </Grid>
    </Box>

    <Panel>
      <Stack gap="sm">
        <Text size="xs" c="dimmed" tt="uppercase">{t('equipment.group.wearable')}</Text>
        <InventoryRows rows={rows} equipment={equipment} items={items} name={rowName(name)} lookup={lookup}
          empty={t('sheet.empty')} disabled={disabled} {...(onChange ? { onChange } : {})} />
      </Stack>
    </Panel>

    {onChange && <ModalSheet opened={picking !== null} onClose={() => setPicking(null)} title={picking === null ? '' : labels[picking]}>
      {picking !== null && slot !== null && <Stack gap="xs">
        {occupants(picking).map((slug, at) => (
          <Button key={`${slug}:${at}`} variant="default" justify="space-between"
            onClick={() => { onChange(unequip(equipment, slug, slot === ELSEWHERE ? undefined : slot)); setPicking(null) }}>
            {t('equipment.takeOffNamed', { name: name(slug) })}
          </Button>
        ))}
        {fitting.length === 0 && <Text size="sm" c="dimmed">{t('equipment.nothingFits')}</Text>}
        {slot !== ELSEWHERE && fitting.map((stack) => (
          <Button key={stack.item} variant="light" justify="space-between"
            onClick={() => { onChange(equip(equipment, items, stack.item ?? '', slot)); setPicking(null) }}>
            {name(stack.item ?? '')}
          </Button>
        ))}
      </Stack>}
    </ModalSheet>}
  </Stack>
}

/**
 * The sheet's Items tab, read top-down: the purse, then what is used up, then
 * everything else carried but not worn. With `onChange`, the purse is five
 * fields and each row has a menu. Nothing here adds an item yet.
 */
export function SheetItems({ equipment, items, name, lookup, disabled = false, onChange }: InventoryProps) {
  const t = useT()
  const rows = mergeStacks(equipment)
  const section = (group: ItemGroup, label: string) => <Panel>
    <Stack gap="sm">
      <Text size="xs" c="dimmed" tt="uppercase">{label}</Text>
      <InventoryRows rows={rows.filter((row) => groupOf(items.get(row.item ?? '')) === group)}
        equipment={equipment} items={items} name={rowName(name)} lookup={lookup}
        empty={t('sheet.empty')} disabled={disabled} {...(onChange ? { onChange } : {})} />
    </Stack>
  </Panel>

  return <Stack gap="md">
    <Panel>
      <Stack gap="sm">
        <Text size="xs" c="dimmed" tt="uppercase">{t('equipment.purse')}</Text>
        <Purse purse={equipment.purse} disabled={disabled}
          {...(onChange ? { onChange: (unit: string, amount: number) => onChange([setCoin(unit, amount)]) } : {})} />
      </Stack>
    </Panel>
    {section('consumable', t('equipment.group.consumable'))}
    {section('gear', t('equipment.group.gear'))}
  </Stack>
}
