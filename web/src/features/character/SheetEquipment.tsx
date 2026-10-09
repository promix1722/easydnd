import { CUSTOM, ELSEWHERE, discard, groupOf, mergeStacks, setCoin, slotted, unequip } from '@/domain'
import type { InventoryRow, ItemGroup, Slot } from '@/domain'
import type { Change, Equipment, Item, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Box, Grid, Group, ITEM_ICON_SIZE, ItemIcon, Markdown, Menu, Panel, Paper, Stack, Text } from '@/ui'

import { InventoryRows, ItemMenu, Purse } from './Inventory'
import { itemFacts, weaponNumbers } from './options'
import { useSlotLabels } from './slotLabels'
import { WeaponStats } from './WeaponStats'
import type { Card } from './slotLabels'

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
  /** The sheet's actions: a wielded weapon shows the damage and bonus its action has. */
  actions?: readonly SheetAction[]
  disabled?: boolean
  onChange?: (changes: Change[]) => void
}

const rowName = (name: (slug: string) => string) => (row: InventoryRow) => row.customName ?? name(row.item ?? '')

/**
 * The sheet's Equipment tab: what is worn where, then everything that could be.
 *
 * A card is never pressed and an empty one does nothing: an item is put on
 * from its row's menu below. With `onChange`, a worn item has the same menu on
 * its card, to take it off or drop it.
 */
export function SheetEquipment({ equipment, items, name, lookup, actions = [], disabled = false, onChange }: InventoryProps) {
  const t = useT()
  const labels = useSlotLabels()
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
  const card = (each: Card) => {
    const worn = occupants(each)
    // Every card reserves one icon's height, and an item's text is cut to it,
    // so wearing something never moves the cards below.
    const body = <Stack gap={2}>
      <Text size="xs" c="dimmed" tt="uppercase">{labels[each]}</Text>
      <Stack gap={6} mih={ITEM_ICON_SIZE}>
        {worn.length === 0
          ? <Text size="sm" c="dimmed">{t('equipment.slotEmpty')}</Text>
          : worn.map((slug, at) => {
            const item = items.get(slug)
            const word = (ref: string) => lookup(item?.weapon?.properties?.includes(ref) ? 'weapon-properties' : 'damage-types', ref)
            const numbers = weaponNumbers(t, item, actions, word)
            const facts = item === undefined ? undefined : itemFacts(t, item, word, numbers !== undefined)
            const from = slotOfCard(each)
            const slot = from === ELSEWHERE ? undefined : from
            return <Group key={`${slug}:${at}`} gap={6} wrap="nowrap" align="flex-start">
              <ItemIcon icon={item?.icon} />
              <Stack gap={2} mah={ITEM_ICON_SIZE} style={{ minWidth: 0, flex: 1, overflow: 'hidden', overflowWrap: 'anywhere' }}>
                <Text size="sm" fw={500} truncate style={{ flexShrink: 0 }}>{name(slug)}</Text>
                {numbers !== undefined && <WeaponStats inline {...numbers} />}
                {facts !== undefined && <Text size="xs" c="dimmed" lineClamp={numbers === undefined ? 2 : 1} style={{ flexShrink: 0 }}>{facts}</Text>}
                {!!item?.desc?.length && <Text component="div" size="xs" lineClamp={1} style={{ flexShrink: 0 }}><Markdown size="xs" inline>{item.desc[0] ?? ''}</Markdown></Text>}
              </Stack>
              {onChange && <ItemMenu name={name(slug)} disabled={disabled}>
                <Menu.Item onClick={() => onChange(unequip(equipment, slug, slot))}>{t('equipment.takeOffNamed', { name: name(slug) })}</Menu.Item>
                <Menu.Item color="red" onClick={() => onChange(discard(equipment, slug, slot))}>{t('equipment.drop')}</Menu.Item>
              </ItemMenu>}
            </Group>
          })}
      </Stack>
    </Stack>
    return <Paper key={each} withBorder p="xs" radius="md">{body}</Paper>
  }
  const elsewhere = bySlot.get(ELSEWHERE) ?? []

  return <Stack gap="md">
    <Box component="section" aria-label={t('equipment.slots')}>
      <Grid gap="xs">
        {COLUMNS.map((column, at) => <Grid.Col key={at} span={{ base: 12, sm: 4 }} miw={160}><Stack gap="xs">{column.map(card)}</Stack></Grid.Col>)}
        {elsewhere.length > 0 && <Grid.Col span={12}>{card(ELSEWHERE)}</Grid.Col>}
      </Grid>
    </Box>

    <Panel>
      <Stack gap="sm">
        <Text size="xs" c="dimmed" tt="uppercase">{t('equipment.group.wearable')}</Text>
        <InventoryRows rows={rows} equipment={equipment} items={items} name={rowName(name)} lookup={lookup} actions={actions}
          empty={t('sheet.empty')} disabled={disabled} {...(onChange ? { onChange } : {})} />
      </Stack>
    </Panel>
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
