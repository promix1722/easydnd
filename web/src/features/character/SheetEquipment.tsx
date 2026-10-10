import { Link } from 'react-router'

import { CUSTOM, ELSEWHERE, discard, equip, groupOf, mergeStacks, setCoin, setTotal, slotsFor, slotted, unequip } from '@/domain'
import type { InventoryRow, ItemGroup, Slot } from '@/domain'
import type { Change, Equipment, Item, ItemHit, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Badge, Box, Button, Grid, Group, ITEM_ICON_SIZE, Menu, Panel, Paper, Stack, Text } from '@/ui'

import { InventoryRows, ItemCard, ItemDetails, ItemMenu, NAME_LINE, Purse } from './Inventory'
import { AddItems } from './ItemPicker'
import { useSlotLabels } from './slotLabels'
import type { Card } from './slotLabels'

/**
 * The way to the page that writes an item the catalogue does not hold, drawn
 * beside the tab's Add as its equal. A relative link, as an item's Details
 * is, naming the tab to come back to.
 */
function AddCustomItem({ tab }: { tab: 'equipment' | 'items' }) {
  const t = useT()
  return <Button component={Link} to={`custom-item?tab=${tab}`} variant="light">{t('customItem.add')}</Button>
}

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

const ownedOf = (equipment: Equipment) => new Map(mergeStacks(equipment).map((row) => [row.item ?? '', row.count]))
/** Each Add is one more in the backpack. */
const addOne = (equipment: Equipment, onChange: (changes: Change[]) => void) => (hit: ItemHit) =>
  onChange(setTotal(equipment, hit.slug, (ownedOf(equipment).get(hit.slug) ?? 0) + 1))

const rowName = (name: (slug: string) => string) => (row: InventoryRow) => name(row.item ?? '')

/**
 * The sheet's Equipment tab: what is worn where, then everything that could be.
 *
 * A card is never pressed. With `onChange`, each has a menu: a worn item's
 * takes it off or drops it, and an empty slot's lists by name what is carried
 * that could go there -- the same put-on its row's menu below offers -- and
 * ends with the way to write a custom item straight into the slot. Add
 * equipment under the rows searches the catalogue's wearable half in place.
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
    const slotName = <Badge variant="default" size="sm" tt="none" fw={400} c="dimmed" style={{ flexShrink: 0 }}>{labels[each]}</Badge>
    // A card is a name line over one icon's height whether or not anything
    // is in it, so wearing something never moves the cards below -- where
    // there are cards beside it to move; on a phone they are one column, and
    // an empty one is as short as its two lines. The slot and the menu sit
    // where a worn item's do, so the dots are in one place down a column.
    if (worn.length === 0) {
      const slot = slotOfCard(each)
      const fits = slot === ELSEWHERE ? [] : rows.filter((row) =>
        row.count > row.equipped && slotsFor(equipment, items, items.get(row.item ?? '')).includes(slot))
      return <Paper key={each} withBorder p="xs" radius="md">
        <Stack gap={6}>
          <Group gap={6} wrap="nowrap" justify="flex-end" mih={NAME_LINE - 6}>
            {slotName}
            {onChange && slot !== ELSEWHERE && <ItemMenu name={labels[each]} disabled={disabled}>
              {fits.map((row) => (
                <Menu.Item key={row.key} onClick={() => onChange(equip(equipment, items, row.item ?? '', slot))}>
                  {name(row.item ?? '')}
                </Menu.Item>
              ))}
              <Menu.Item component={Link} to={`custom-item?tab=equipment&slot=${slot}`}>{t('customItem.add')}</Menu.Item>
            </ItemMenu>}
          </Group>
          <Group justify="center" mih={{ base: 0, sm: ITEM_ICON_SIZE }}>
            <Text size="sm" c="dimmed">{t('equipment.slotEmpty')}</Text>
          </Group>
        </Stack>
      </Paper>
    }
    return <Paper key={each} withBorder p="xs" radius="md">
      <Stack gap="xs">
        {worn.map((slug, at) => {
          const from = slotOfCard(each)
          const slot = from === ELSEWHERE ? undefined : from
          // Every worn item opens its page; taking it off or dropping it is the owner's.
          return <ItemCard key={`${slug}:${at}`} item={items.get(slug)} label={name(slug)} tag={slotName} actions={actions} lookup={lookup} menu={
            <ItemMenu name={name(slug)}>
              <ItemDetails slug={slug} />
              {onChange && !disabled && <>
                <Menu.Item onClick={() => onChange(unequip(equipment, slug, slot))}>{t('equipment.takeOffNamed', { name: name(slug) })}</Menu.Item>
                <Menu.Item color="red" onClick={() => onChange(discard(equipment, slug, slot))}>{t('equipment.drop')}</Menu.Item>
              </>}
            </ItemMenu>
          } />
        })}
      </Stack>
    </Paper>
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
        <Text size="xs" c="dimmed">{t('equipment.group.wearable')}</Text>
        <InventoryRows cards rows={rows} equipment={equipment} items={items} name={rowName(name)} lookup={lookup}
          empty={t('sheet.empty')} disabled={disabled} {...(onChange ? { onChange } : {})} />
        {onChange && <AddItems label={t('equipment.addEquipment')} wearable owned={ownedOf(equipment)} disabled={disabled} onAdd={addOne(equipment, onChange)} beside={<AddCustomItem tab="equipment" />} />}
      </Stack>
    </Panel>
  </Stack>
}

/**
 * The sheet's Items tab, read top-down: the purse, then what is used up, then
 * everything else carried but not worn. With `onChange`, the purse is five
 * fields, each row has a menu, and Add item at the foot searches the half of
 * the catalogue that is carried rather than worn.
 */
export function SheetItems({ equipment, items, name, lookup, disabled = false, onChange }: InventoryProps) {
  const t = useT()
  const rows = mergeStacks(equipment)
  const section = (group: ItemGroup, label: string) => <Panel>
    <Stack gap="sm">
      <Text size="xs" c="dimmed">{label}</Text>
      <InventoryRows rows={rows.filter((row) => groupOf(items.get(row.item ?? '')) === group)}
        equipment={equipment} items={items} name={rowName(name)} lookup={lookup}
        empty={t('sheet.empty')} disabled={disabled} {...(onChange ? { onChange } : {})} />
    </Stack>
  </Panel>

  return <Stack gap="md">
    <Panel>
      <Stack gap="sm">
        <Text size="xs" c="dimmed">{t('equipment.purse')}</Text>
        <Purse purse={equipment.purse} disabled={disabled}
          {...(onChange ? { onChange: (unit: string, amount: number) => onChange([setCoin(unit, amount)]) } : {})} />
      </Stack>
    </Panel>
    {section('consumable', t('equipment.group.consumable'))}
    {section('gear', t('equipment.group.gear'))}
    {onChange && <Panel>
      <AddItems label={t('equipment.addItem')} wearable={false} owned={ownedOf(equipment)} disabled={disabled} onAdd={addOne(equipment, onChange)} beside={<AddCustomItem tab="items" />} />
    </Panel>}
  </Stack>
}
