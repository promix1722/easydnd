import { useParams } from 'react-router'

import type { Entry, Item } from '@/lib/api'
import { bySlug, getCollection, getEntries, getSharedOwner, getSharedSheet, getSheet } from '@/lib/api'
import { characterPath } from '@/lib/api/characters'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { ItemIcon, joinProse, Markdown, Page, Panel, SimpleGrid, SourceTags, Stack, Text, pageState } from '@/ui'

/**
 * One item, at full length: what it is, every number it has, what it says.
 *
 * A row on the sheet has room for a name and a line, and a slot card for less
 * -- so the rest is here, a page of its own in the shape of a spell's, opened
 * from the row's menu. It is read from the *character's* catalogue rather than
 * the compendium's: an item on a sheet is that sheet's pack's item, homebrew
 * included, and the two routes that reach this say whose sheet it is --
 * `/characters/:id/items/:slug` for the owner, and the same under a group's
 * path for a sheet shared with a table.
 *
 * Captioned facts rather than the sheet's one dimmed line: there is room for a
 * caption here, and "Range" over "80/320 ft." is read faster than the same
 * words in a sentence.
 */
export function ItemScreen() {
  const t = useT()
  const { id = '', character, slug = '' } = useParams()
  // A shared sheet is `/groups/:id/characters/:character/...`; the owner's is
  // `/characters/:id/...`.
  const scope = character !== undefined ? `/shared/${encodeURIComponent(character)}/catalog` : `${characterPath(id)}/catalog`

  // An item is its character's: the trail runs through the sheet it was
  // opened from, and through the player first when the sheet is somebody
  // else's. The names are asked beside the item and may fail without taking
  // the page with them -- a crumb that is a placeholder is still a way back.
  const shared = character !== undefined
  // Three ways in: the owner's sheet, a group's read of it, and its public link.
  const sheetAt = !shared ? `/characters/${id}` : id ? `/groups/${id}/characters/${character}` : `/shared/${character}`
  const loaded = useResource(`item:${scope}:${slug}`, async (signal) => {
    const [equipment, magic, properties, damageTypes, sheetName, owner] = await Promise.all([
      getEntries<Item>('equipment', [slug], scope),
      getEntries<Item>('magic-items', [slug], scope),
      getCollection<Entry>('weapon-properties', scope),
      getCollection<Entry>('damage-types', scope),
      (shared ? getSharedSheet(character, signal) : getSheet(id, signal)).then((sheet) => sheet.identity.name, () => null),
      shared && id ? getSharedOwner(id, character, signal) : Promise.resolve(null),
    ])
    return { item: equipment[0] ?? magic[0] ?? null, words: bySlug([...properties, ...damageTypes]), sheetName, owner }
  })
  const above = [
    ...(shared && id ? [{ label: loaded.data === null ? null : (loaded.data.owner?.name || t('common.unnamed')) }] : []),
    { label: loaded.data === null ? null : (loaded.data.sheetName || t('common.unnamed')), to: sheetAt },
  ]

  const state = pageState(loaded, { title: t('item.loadFailed'), fallback: t('error.unknown'), onRetry: loaded.reload })
  if (state.kind !== 'ready' || loaded.data === null) return <Page trail={[...above, { label: null }]} state={state} />

  const { item, words } = loaded.data
  // The endpoint drops a slug it does not know rather than failing.
  if (item === null) return <Page trail={[...above, { label: slug }]} state={{ kind: 'failed', title: t('item.loadFailed'), detail: t('item.notFound') }} />

  const word = (ref: string) => words.get(ref)?.name ?? ref
  const feet = (near?: number, far?: number) => near ? t('vitals.feet', { distance: `${near}/${far ?? near}` }) : ''
  const damage = (dice?: { dice: string; type?: string }) => dice === undefined ? '' : [dice.dice, dice.type === undefined ? '' : word(dice.type)].filter(Boolean).join(' ')
  const armor = item.armor
  const weapon = item.weapon
  const facts = [
    { key: 'ac', label: t('item.armorClass'), value: armor === undefined ? '' : String(armor.baseAC) },
    { key: 'dex', label: t('item.dexterity'), value: armor?.addsDexBonus ? (armor.maxDexBonus === undefined ? t('equipment.dex') : t('equipment.dexCap', { count: armor.maxDexBonus })) : '' },
    { key: 'strength', label: t('item.strength'), value: armor?.strengthMinimum ? String(armor.strengthMinimum) : '' },
    { key: 'stealth', label: t('item.stealth'), value: armor?.stealthDisadvantage ? t('item.disadvantage') : '' },
    { key: 'damage', label: t('weapon.damage'), value: damage(weapon?.damage) },
    { key: 'twoHands', label: t('item.twoHanded'), value: damage(weapon?.twoHandedDamage) },
    { key: 'range', label: t('weapon.range'), value: feet(weapon?.normalRange, weapon?.longRange) },
    { key: 'thrown', label: t('item.thrown'), value: feet(weapon?.throwNormalRange, weapon?.throwLongRange) },
    { key: 'properties', label: t('item.properties'), value: (weapon?.properties ?? []).map(word).join(', ') },
    { key: 'weight', label: t('item.weight'), value: item.weight === undefined ? '' : t('item.pounds', { value: item.weight }) },
    { key: 'cost', label: t('item.cost'), value: item.cost === undefined ? '' : `${item.cost.amount} ${item.cost.unit}` },
  ].filter((fact) => fact.value !== '')

  return (
    <Page trail={[...above, { label: item.name }]} mark={<ItemIcon icon={item.icon} />}>
      <Panel>
        <Stack gap="md">
          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
            {facts.map((fact) => <div key={fact.key}>
              <Text size="xs" c="dimmed" tt="uppercase">{fact.label}</Text>
              <Text size="sm">{fact.value}</Text>
            </div>)}
            {item.provenance !== undefined && <div>
              <Text size="xs" c="dimmed" tt="uppercase">{t('spells.filter.source')}</Text>
              <SourceTags provenance={item.provenance} />
            </div>}
          </SimpleGrid>
          {!!item.desc?.length && <Markdown>{joinProse(item.desc)}</Markdown>}
        </Stack>
      </Panel>
    </Page>
  )
}
