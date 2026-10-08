import { useState } from 'react'

import { ELSEWHERE, ITEM_GROUPS, equip, fitsSlot, groupOf, mergeStacks, setCoin, setTotal, slotted, unequip } from '@/domain'
import type { InventoryRow, ItemGroup, Slot } from '@/domain'
import type { Change, Equipment, Identity, Item } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Avatar, Box, Button, Center, Grid, ModalSheet, Panel, Paper, Stack, TabRow, Text, UnstyledButton, characterAvatar } from '@/ui'

import { InventoryRows, Purse } from './Inventory'
import { ItemPicker } from './ItemPicker'

/** The paperdoll: worn pieces head to foot either side of the portrait, then what the hands hold. */
const LEFT: readonly Slot[] = ['head', 'neck', 'back', 'body']
const RIGHT: readonly Slot[] = ['arms', 'hands', 'waist', 'feet']
const HANDS: readonly Slot[] = ['main-hand', 'off-hand', 'ring']

type Card = Slot | typeof ELSEWHERE

/** What both tabs are drawn from. Read-only without `onChange`, which is how a sheet shared with a table is drawn. */
interface InventoryProps {
  equipment: Equipment
  items: ReadonlyMap<string, Item>
  name: (slug: string) => string
  disabled?: boolean
  onChange?: (changes: Change[]) => void
}

const rowName = (name: (slug: string) => string) => (row: InventoryRow) => row.customName ?? name(row.item ?? '')

/**
 * The sheet's Equipment tab: what is worn where, then everything that could be.
 *
 * With `onChange`, a slot is a button that offers what in the backpack fits.
 */
export function SheetEquipment({ equipment, items, name, identity, disabled = false, onChange }: InventoryProps & {
  identity: Pick<Identity, 'image' | 'classes'>
}) {
  const t = useT()
  const [picking, setPicking] = useState<Card | null>(null)
  const slots: Record<Card, string> = {
    head: t('equipment.slot.head'), neck: t('equipment.slot.neck'), back: t('equipment.slot.back'), body: t('equipment.slot.body'),
    arms: t('equipment.slot.arms'), hands: t('equipment.slot.hands'), waist: t('equipment.slot.waist'), feet: t('equipment.slot.feet'),
    'main-hand': t('equipment.slot.main-hand'), 'off-hand': t('equipment.slot.off-hand'), ring: t('equipment.slot.ring'),
    elsewhere: t('equipment.slot.elsewhere'),
  }
  const bySlot = slotted(equipment, items)
  const rows = mergeStacks(equipment).filter((row) => groupOf(items.get(row.item ?? '')) === 'wearable')
  // Nothing fits "elsewhere": it only ever holds what is already there.
  const fitting = picking === null || picking === ELSEWHERE ? [] : equipment.backpack.filter((stack, at, all) =>
    stack.item !== undefined && fitsSlot(items.get(stack.item), picking) &&
    all.findIndex((other) => other.item === stack.item) === at)

  const card = (slot: Card) => {
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
  }
  const half = { base: 6, sm: 4 }
  const elsewhere = bySlot.get(ELSEWHERE) ?? []

  return <Stack gap="md">
    <Box component="section" aria-label={t('equipment.slots')}>
      <Grid gap="xs">
        <Grid.Col span={half}><Stack gap="xs">{LEFT.map(card)}</Stack></Grid.Col>
        {/* The portrait is in the page header already; a phone's two columns leave it no room. */}
        <Grid.Col span={4} visibleFrom="sm">
          <Center h="100%">
            <Avatar image={identity.image} fallback={characterAvatar(identity.classes)} size={168} />
          </Center>
        </Grid.Col>
        <Grid.Col span={half}><Stack gap="xs">{RIGHT.map(card)}</Stack></Grid.Col>
        {HANDS.map((slot) => <Grid.Col key={slot} span={half}>{card(slot)}</Grid.Col>)}
        {elsewhere.length > 0 && <Grid.Col span={12}>{card(ELSEWHERE)}</Grid.Col>}
      </Grid>
    </Box>

    <Panel>
      <Stack gap="sm">
        <Text size="xs" c="dimmed" tt="uppercase">{t('equipment.group.wearable')}</Text>
        <InventoryRows
          rows={rows}
          name={rowName(name)}
          empty={t('sheet.empty')}
          disabled={disabled}
          {...(onChange ? { onTotal: (row: InventoryRow, total: number) => onChange(setTotal(equipment, row.item ?? '', total)) } : {})}
        />
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
        {picking !== ELSEWHERE && fitting.map((stack) => (
          <Button key={stack.item} variant="light" justify="space-between"
            onClick={() => { onChange(equip(equipment, items, stack.item ?? '', picking)); setPicking(null) }}>
            {name(stack.item ?? '')}
          </Button>
        ))}
      </Stack>}
    </ModalSheet>}
  </Stack>
}

/** The groups the Items tab lists: everything that is not worn, which Equipment already has. */
const CARRIED: readonly ItemGroup[] = ITEM_GROUPS.filter((group) => group !== 'wearable')

/**
 * The sheet's Items tab: what is carried but not worn, in two groups, then
 * the purse. With `onChange`, a row has a count stepper and Add item searches
 * the character's catalogue for something not owned yet.
 */
export function SheetItems({ equipment, items, name, disabled = false, onChange }: InventoryProps) {
  const t = useT()
  const [group, setGroup] = useState<ItemGroup>(CARRIED[0] ?? 'gear')
  const [adding, setAdding] = useState(false)
  const groups: Record<ItemGroup, string> = {
    wearable: t('equipment.group.wearable'), consumable: t('equipment.group.consumable'), gear: t('equipment.group.gear'),
  }
  const rows = mergeStacks(equipment)

  return <Stack gap="md">
    <Panel>
      <Stack gap="md">
        <TabRow tabs={CARRIED.map((each) => ({ value: each, label: groups[each] }))} value={group}
          onChange={(next) => setGroup(next as ItemGroup)}>
          <InventoryRows
            rows={rows.filter((row) => groupOf(items.get(row.item ?? '')) === group)}
            name={rowName(name)}
            empty={t('sheet.empty')}
            disabled={disabled}
            {...(onChange ? { onTotal: (row: InventoryRow, total: number) => onChange(setTotal(equipment, row.item ?? '', total)) } : {})}
          />
        </TabRow>
        {onChange && <Button variant="light" disabled={disabled} onClick={() => setAdding(true)}>{t('equipment.addItem')}</Button>}
        <Purse purse={equipment.purse} disabled={disabled}
          {...(onChange ? { onChange: (unit: string, amount: number) => onChange([setCoin(unit, amount)]) } : {})} />
      </Stack>
    </Panel>

    {onChange && <ItemPicker opened={adding} onClose={() => setAdding(false)} onPick={(hit) => {
      // One more of it, wherever it already is: a wearable just bought lands
      // in the backpack and is put on from the Equipment tab.
      const owned = rows.find((row) => row.item === hit.slug)?.count ?? 0
      onChange(setTotal(equipment, hit.slug, owned + 1))
      setAdding(false)
    }} />}
  </Stack>
}
