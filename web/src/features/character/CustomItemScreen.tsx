import { useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'

import { COINS, CUSTOM, SLOTS, equip } from '@/domain'
import type { Slot } from '@/domain'
import type { Entry, Item } from '@/lib/api'
import { appendEvents, getCollection, getEntries, getEvents, getGame, getSharedSheet, getSheet, grantCustomItem } from '@/lib/api'
import { characterPath, upsertCustomOption } from '@/lib/api/characters'
import type { CustomItem, CustomOption } from '@/lib/api/characters'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  Button, Checkbox, Group, ItemIcon, MultiSelect, NumberInput, Page, Panel, Select, SimpleGrid, Stack, Text, Textarea, TextInput,
  UnstyledButton, pageState,
} from '@/ui'

import { useSlotLabels } from './slotLabels'

/** Everything the form holds, flat: a field per control, so one setter serves them all. */
interface Draft {
  name: string
  description: string
  /** The pack's label for the icon, which is what is stored, and the picture it stands for, which is only shown. */
  icon: string
  picture: string | undefined
  category: string
  slot: string
  weight: number
  cost: number
  coin: string
  shape: 'none' | 'weapon' | 'armor'
  weaponCategory: string
  weaponRange: string
  dice: string
  damageType: string
  twoHandedDice: string
  near: number
  far: number
  properties: string[]
  armorCategory: string
  baseAC: number
  addsDex: boolean
  /** Empty for no cap. */
  maxDex: number | ''
  strength: number
  stealth: boolean
}

function draftOf(option: CustomOption | undefined, picture: string | undefined, slot: string): Draft {
  const item = option?.item
  const weapon = item?.weapon
  const armor = item?.armor
  return {
    name: option?.name ?? '', description: option?.description ?? '',
    icon: item?.icon ?? '', picture,
    category: item?.category ?? '', slot: item?.slot ?? slot,
    weight: item?.weight ?? 0, cost: item?.cost?.amount ?? 0, coin: item?.cost?.unit ?? 'gp',
    shape: weapon ? 'weapon' : armor ? 'armor' : 'none',
    weaponCategory: weapon?.category ?? 'simple', weaponRange: weapon?.range ?? 'melee',
    dice: weapon?.damage?.dice ?? '', damageType: weapon?.damage?.type ?? '', twoHandedDice: weapon?.twoHandedDamage?.dice ?? '',
    near: weapon?.normalRange ?? weapon?.throwNormalRange ?? 0, far: weapon?.longRange ?? weapon?.throwLongRange ?? 0,
    properties: weapon?.properties ?? [],
    armorCategory: armor?.category ?? 'light', baseAC: armor?.baseAC ?? 11,
    addsDex: armor?.addsDexBonus ?? true, maxDex: armor?.maxDexBonus ?? '',
    strength: armor?.strengthMinimum ?? 0, stealth: armor?.stealthDisadvantage ?? false,
  }
}

/** The form as the server reads an item. A melee weapon's two distances are how far it is thrown; a ranged one's are its range. */
function itemOf(d: Draft): CustomItem {
  const damage = (dice: string) => dice.trim() === '' ? undefined : { dice: dice.trim(), ...(d.damageType ? { type: d.damageType } : {}) }
  const one = damage(d.dice)
  const two = damage(d.twoHandedDice)
  const ranged = d.weaponRange === 'ranged'
  return {
    ...(d.icon ? { icon: d.icon } : {}),
    ...(d.category ? { category: d.category } : {}),
    ...(d.slot ? { slot: d.slot as Slot } : {}),
    ...(d.weight > 0 ? { weight: d.weight } : {}),
    ...(d.cost > 0 ? { cost: { amount: d.cost, unit: d.coin } } : {}),
    ...(d.shape === 'weapon' ? { weapon: {
      category: d.weaponCategory, range: d.weaponRange, properties: d.properties,
      ...(one ? { damage: one } : {}),
      ...(two ? { twoHandedDamage: two } : {}),
      ...(d.near > 0 ? (ranged ? { normalRange: d.near, longRange: Math.max(d.far, d.near) } : { throwNormalRange: d.near, throwLongRange: Math.max(d.far, d.near) }) : {}),
    } } : {}),
    ...(d.shape === 'armor' ? { armor: {
      category: d.armorCategory, baseAC: d.baseAC, addsDexBonus: d.addsDex, stealthDisadvantage: d.stealth,
      ...(d.addsDex && d.maxDex !== '' ? { maxDexBonus: d.maxDex } : {}),
      ...(d.strength > 0 ? { strengthMinimum: d.strength } : {}),
    } } : {}),
  }
}

/**
 * A new entry's id. Not `crypto.randomUUID`: that exists only in a secure
 * context, and the development host is plain HTTP.
 */
const newId = () => Array.from(crypto.getRandomValues(new Uint8Array(12)), (byte) => byte.toString(16).padStart(2, '0')).join('')

const whole = (value: number | string) => typeof value === 'number' ? value : 0

/**
 * Writing an item the catalogue does not hold: its name, its picture, where it
 * is worn and every number a weapon or a suit of armor has.
 *
 * One screen for every way in. The sheet's Items and Equipment tabs open it at
 * `/characters/:id/custom-item`, an empty slot's menu with `?slot=` so the new
 * item goes straight on, and an item's own page with the entry's id to change
 * one. A game opens it at `/games/:id/characters/:character/custom-item`
 * with `?entry=`, where whoever runs the table gives the item to a player's
 * character -- one of the things a table hands over.
 *
 * It is a page and not a dialog, so its trail is the way back: the sheet, on
 * the tab that opened it (`?tab=`), or the game.
 */
export function CustomItemScreen() {
  const t = useT()
  const { id = '', character, option } = useParams()
  const [search] = useSearchParams()
  const inGame = character !== undefined
  const scope = inGame ? `/shared/${encodeURIComponent(character)}/catalog` : `${characterPath(id)}/catalog`
  const tab = search.get('tab')
  const back = inGame ? `/games/${id}` : `/characters/${id}${tab ? `?tab=${encodeURIComponent(tab)}` : ''}`
  const title = t(option === undefined ? 'customItem.add' : 'customItem.edit')

  const loaded = useResource(`custom-item:${scope}:${option ?? ''}`, async (signal) => {
    const [categories, damageTypes, properties, sheet, game, stored] = await Promise.all([
      getCollection<Entry>('equipment-categories', scope),
      getCollection<Entry>('damage-types', scope),
      getCollection<Entry>('weapon-properties', scope),
      inGame ? getSharedSheet(character, signal) : getSheet(id, signal),
      // A crumb that is a placeholder is still a way back.
      inGame ? getGame(id, signal).then((found) => found.name, () => '') : Promise.resolve(''),
      // The stored icon is a label; the item as the catalogue serves it has the picture.
      option === undefined ? Promise.resolve([]) : getEntries<Item>('equipment', [`custom-${option}`], scope),
    ])
    return { categories, damageTypes, properties, sheet, game, picture: stored[0]?.icon }
  })

  const sheetName = loaded.data === null ? null : (loaded.data.sheet.identity.name || t('common.unnamed'))
  const trail = [
    ...(inGame ? [{ label: loaded.data === null ? null : (loaded.data.game || t('common.unnamed')), to: back }, { label: sheetName }] : [{ label: sheetName, to: back }]),
    { label: title },
  ]
  const state = pageState(loaded, { title: t('customItem.loadFailed'), fallback: t('error.unknown'), onRetry: loaded.reload })
  if (state.kind !== 'ready' || loaded.data === null) return <Page trail={trail} state={state} />

  const stored = option === undefined ? undefined : loaded.data.sheet.customOptions?.find((each) => each.id === option && each.kind === 'item')
  if (option !== undefined && stored === undefined) return <Page trail={trail} state={{ kind: 'failed', title: t('customItem.loadFailed'), detail: t('item.notFound') }} />

  return (
    <Page trail={trail}>
      <ItemForm characterId={inGame ? character : id} {...(inGame ? { gameId: { game: id, entry: search.get('entry') ?? '' } } : {})} recipient={sheetName ?? ''} scope={scope} back={back}
        stored={stored} picture={loaded.data.picture} slot={search.get('slot') ?? ''}
        categories={loaded.data.categories} damageTypes={loaded.data.damageTypes} properties={loaded.data.properties} />
    </Page>
  )
}

function ItemForm({ characterId, gameId, recipient, scope, back, stored, picture, slot, categories, damageTypes, properties }: {
  characterId: string
  /** Set when the item is given at a game rather than written on the owner's sheet: the game and the entry it goes to. */
  gameId?: { game: string; entry: string }
  /** The character's name, for the game's word that the item reached them. */
  recipient: string
  scope: string
  back: string
  stored: CustomOption | undefined
  picture: string | undefined
  /** The slot whose menu opened the form, or '' for none: a new item made for it is put on. */
  slot: string
  categories: readonly Entry[]
  damageTypes: readonly Entry[]
  properties: readonly Entry[]
}) {
  const t = useT()
  const navigate = useNavigate()
  const labels = useSlotLabels()
  const [draft, setDraft] = useState(() => draftOf(stored, picture, slot === CUSTOM ? '' : slot))
  const set = (change: Partial<Draft>) => setDraft((was) => ({ ...was, ...change }))
  const [palette, setPalette] = useState<readonly Item[] | null>(null)
  const icons = useAction(async () => setPalette(await getCollection<Item>('item-icons', scope)))

  const save = useAction(async () => {
    const item = itemOf(draft)
    const name = draft.name.trim()
    if (gameId !== undefined) {
      await grantCustomItem(gameId.game, gameId.entry, { name, description: draft.description, item })
      // The game says it out loud: nothing on its page shows that the item landed.
      await navigate(back, { state: { given: { item: name, name: recipient } } })
      return
    } else {
      const optionId = stored?.id ?? newId()
      const log = await getEvents(characterId)
      const written = await upsertCustomOption(characterId, log.revision ?? log.seq, {
        ...(stored ?? { kind: 'item', source: '', selected: true, count: 1, placement: 'backpack' }),
        id: optionId, name, description: draft.description, item,
      })
      // Opened from an empty slot, a new item that still goes there is put on:
      // the same change the slot's own menu would have written.
      const into = slot === CUSTOM ? CUSTOM : slot !== '' && slot === draft.slot ? slot as Slot : undefined
      if (stored === undefined && into !== undefined) {
        await appendEvents(characterId, written.seq, [{ type: 'change', changes: equip(written.sheet.equipment, new Map(), `custom-${optionId}`, into) }], written.revision)
      }
    }
    await navigate(back)
  })

  const pick = (entries: readonly Entry[]) => entries.map((entry) => ({ value: entry.slug, label: entry.name }))
  const word = (key: 'simple' | 'martial' | 'melee' | 'ranged' | 'light' | 'medium' | 'heavy' | 'shield') => ({
    simple: t('customItem.simple'), martial: t('customItem.martial'), melee: t('customItem.melee'), ranged: t('customItem.ranged'),
    light: t('customItem.light'), medium: t('customItem.medium'), heavy: t('customItem.heavy'), shield: t('customItem.shield'),
  })[key]
  const slotName = (each: Slot) => each === 'ring' ? t('customItem.ring') : labels[each]

  return (
    <form onSubmit={(event) => { event.preventDefault(); void save.run() }}>
      <Stack gap="md">
        <Panel>
          <Stack gap="sm">
            <TextInput label={t('custom.name')} required maxLength={300} value={draft.name} onChange={(event) => set({ name: event.currentTarget.value })} />
            <Textarea label={t('customItem.description')} autosize minRows={3} maxLength={16000} value={draft.description}
              onChange={(event) => set({ description: event.currentTarget.value })} />
            <Group gap="sm" align="center">
              <ItemIcon icon={draft.picture} />
              <Button variant="default" loading={icons.pending} onClick={() => palette === null ? void icons.run() : setPalette(null)}>
                {t(palette === null ? 'customItem.chooseIcon' : 'customItem.hideIcons')}
              </Button>
              {draft.icon !== '' && <Button variant="subtle" onClick={() => set({ icon: '', picture: undefined })}>{t('customItem.noIcon')}</Button>}
            </Group>
            {icons.error !== null && <Text size="sm" c="red">{icons.error}</Text>}
            {palette !== null && (
              <SimpleGrid cols={{ base: 4, xs: 6, sm: 8, lg: 12 }} spacing="xs" role="group" aria-label={t('customItem.icons')}>
                {palette.map((icon) => (
                  <UnstyledButton key={icon.slug} aria-label={icon.slug} aria-pressed={draft.icon === icon.slug}
                    style={{ borderRadius: 10, outline: draft.icon === icon.slug ? '2px solid var(--mantine-primary-color-filled)' : 'none', justifySelf: 'center' }}
                    onClick={() => { set({ icon: icon.slug, picture: icon.icon }); setPalette(null) }}>
                    <ItemIcon icon={icon.icon} />
                  </UnstyledButton>
                ))}
              </SimpleGrid>
            )}
          </Stack>
        </Panel>

        <Panel>
          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
            <Select label={t('customItem.category')} clearable searchable data={pick(categories)} value={draft.category || null}
              onChange={(category) => set({ category: category ?? '' })} />
            <Select label={t('customItem.slot')} clearable placeholder={t('customItem.carried')}
              data={SLOTS.filter((each) => each.slot !== CUSTOM).map((each) => ({ value: each.slot, label: slotName(each.slot) }))}
              value={draft.slot || null} onChange={(next) => set({ slot: next ?? '' })} />
            <NumberInput label={t('customItem.weight')} min={0} max={100000} decimalScale={2} value={draft.weight} onChange={(weight) => set({ weight: whole(weight) })} />
            <Group gap="xs" wrap="nowrap" align="flex-end">
              <NumberInput label={t('item.cost')} min={0} max={1000000000} allowDecimal={false} value={draft.cost} onChange={(cost) => set({ cost: whole(cost) })} style={{ flex: 1 }} />
              <Select aria-label={t('customItem.coin')} w={90} allowDeselect={false} data={[...COINS]} value={draft.coin} onChange={(coin) => set({ coin: coin ?? 'gp' })} />
            </Group>
          </SimpleGrid>
        </Panel>

        <Panel>
          <Stack gap="sm">
            <Select label={t('customItem.shape')} allowDeselect={false} value={draft.shape}
              data={[{ value: 'none', label: t('customItem.gear') }, { value: 'weapon', label: t('customItem.weapon') }, { value: 'armor', label: t('customItem.armor') }]}
              onChange={(shape) => set({ shape: (shape ?? 'none') as Draft['shape'] })} />
            {draft.shape === 'weapon' && (
              <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
                <Select label={t('customItem.proficiency')} allowDeselect={false} value={draft.weaponCategory}
                  data={(['simple', 'martial'] as const).map((value) => ({ value, label: word(value) }))} onChange={(next) => set({ weaponCategory: next ?? 'simple' })} />
                <Select label={t('customItem.reach')} allowDeselect={false} value={draft.weaponRange}
                  data={(['melee', 'ranged'] as const).map((value) => ({ value, label: word(value) }))} onChange={(next) => set({ weaponRange: next ?? 'melee' })} />
                <TextInput label={t('weapon.damage')} placeholder="1d8" value={draft.dice} onChange={(event) => set({ dice: event.currentTarget.value })} />
                <Select label={t('customItem.damageType')} clearable data={pick(damageTypes)} value={draft.damageType || null}
                  onChange={(type) => set({ damageType: type ?? '' })} />
                <TextInput label={t('item.twoHanded')} placeholder="1d10" value={draft.twoHandedDice} onChange={(event) => set({ twoHandedDice: event.currentTarget.value })} />
                <MultiSelect label={t('item.properties')} data={pick(properties)} value={draft.properties} onChange={(next) => set({ properties: next })} />
                <NumberInput label={t(draft.weaponRange === 'ranged' ? 'customItem.rangeNormal' : 'customItem.thrownNormal')} min={0} max={10000} step={5} allowDecimal={false}
                  value={draft.near} onChange={(near) => set({ near: whole(near) })} />
                <NumberInput label={t(draft.weaponRange === 'ranged' ? 'customItem.rangeLong' : 'customItem.thrownLong')} min={0} max={10000} step={5} allowDecimal={false}
                  value={draft.far} onChange={(far) => set({ far: whole(far) })} />
              </SimpleGrid>
            )}
            {draft.shape === 'armor' && (
              <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
                <Select label={t('customItem.armorKind')} allowDeselect={false} value={draft.armorCategory}
                  data={(['light', 'medium', 'heavy', 'shield'] as const).map((value) => ({ value, label: word(value) }))} // The category's own rule for Dexterity, as a starting point: all of it, up to +2, or none.
                  onChange={(next) => set({ armorCategory: next ?? 'light', addsDex: next === 'light' || next === 'medium', maxDex: next === 'medium' ? 2 : '' })} />
                <NumberInput label={t(draft.armorCategory === 'shield' ? 'customItem.shieldBonus' : 'item.armorClass')} min={0} max={30} allowDecimal={false}
                  value={draft.baseAC} onChange={(ac) => set({ baseAC: whole(ac) })} />
                <Checkbox label={t('equipment.dex')} checked={draft.addsDex} onChange={(event) => set({ addsDex: event.currentTarget.checked })} />
                <NumberInput label={t('customItem.maxDex')} disabled={!draft.addsDex} min={0} max={10} allowDecimal={false}
                  value={draft.maxDex} onChange={(cap) => set({ maxDex: typeof cap === 'number' ? cap : '' })} />
                <NumberInput label={t('item.strength')} min={0} max={30} allowDecimal={false} value={draft.strength} onChange={(strength) => set({ strength: whole(strength) })} />
                <Checkbox label={t('equipment.stealth')} checked={draft.stealth} onChange={(event) => set({ stealth: event.currentTarget.checked })} />
              </SimpleGrid>
            )}
          </Stack>
        </Panel>

        {save.error !== null && <Text size="sm" c="red" role="alert">{save.error}</Text>}
        <Group gap="sm">
          <Button type="submit" loading={save.pending} disabled={draft.name.trim() === ''}>
            {t(stored !== undefined ? 'customItem.save' : gameId !== undefined ? 'customItem.give' : 'customItem.add')}
          </Button>
          <Button variant="default" disabled={save.pending} onClick={() => void navigate(back)}>{t('common.cancel')}</Button>
        </Group>
      </Stack>
    </form>
  )
}
