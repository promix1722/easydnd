import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import type { ReactNode } from 'react'

import { bySlug, getCollection, getEntries, searchItems } from '@/lib/api'
import type { Entry, Item, ItemFilters, ItemHit, ItemPage } from '@/lib/api'
import { useCatalogScope } from '@/lib/api/catalogScope'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Badge, Box, Button, ChoiceDetails, Group, ItemIcon, PageBody, Paper, Select, Stack, Text, Notification, TextInput, pageState, useDebouncedValue } from '@/ui'

import { ItemBody } from './ItemScreen'

const PAGE = 20
const NOTHING_OWNED: ReadonlyMap<string, number> = new Map()

/** One width for both selects, so the row does not re-wrap as a value is picked. */
const FILTER_WIDTH = { w: { base: '100%', sm: 220 }, miw: 0, maw: '100%' } as const

/**
 * A tab's Add button and the search it opens in place, under the list it adds
 * to: equipment and magic items together, a page at a time -- the spell
 * selection's shape, because the compendium is never pulled whole.
 *
 * `wearable` is the tab's half of the catalogue and is not the player's to
 * change: Equipment adds what has a slot, Items what has none, so a thing
 * added here turns up in the list above. A row opens to the item's full
 * description; each Add is one more in the backpack and one write.
 *
 * With `added`, an Add closes the search and says what was added, the way
 * giving an item at a game does: one thing, then back to the sheet. The notice
 * waits for `owned` to show the item, so it is never said of a write that
 * failed.
 */
export function AddItems({ label, wearable = null, owned = NOTHING_OWNED, disabled = false, onAdd, added, detailsTo, beside, children }: {
  label: string
  /** Drawn on the button's line while the search is closed: the other way to add something. */
  beside?: ReactNode
  /** Absent searches both halves: a DM hands out anything. */
  wearable?: boolean | null
  /** How many of each the receiver already has, printed beside a hit. */
  owned?: ReadonlyMap<string, number>
  disabled?: boolean
  onAdd: (hit: ItemHit) => void
  /** What to say once an added item is in `owned`. With it, an Add closes the search. */
  added?: (hit: ItemHit) => string
  /** Where a hit's own page is. With it a row is a link there and nothing opens in place. */
  detailsTo?: (hit: ItemHit) => string
  /** Drawn above the search once it is open: a game asks who the item is for. */
  children?: ReactNode
}) {
  const t = useT()
  const scope = useCatalogScope()
  const [opened, setOpened] = useState(false)
  const [q, setQ] = useState('')
  const [category, setCategory] = useState<string | null>(null)
  const [kind, setKind] = useState<string | null>(null)
  const [reading, setReading] = useState<string | null>(null)
  const [last, setLast] = useState<{ slug: string; had: number; text: string } | null>(null)
  const arrived = last !== null && (owned.get(last.slug) ?? 0) > last.had
  useEffect(() => {
    if (!arrived) return
    const timer = setTimeout(() => setLast(null), 4000)
    return () => clearTimeout(timer)
  }, [arrived])
  function add(hit: ItemHit) {
    onAdd(hit)
    if (added === undefined) return
    setLast({ slug: hit.slug, had: owned.get(hit.slug) ?? 0, text: added(hit) })
    setOpened(false)
  }
  // The box follows the keys; the search follows the pause after them. A hit
  // carries its artwork, so a request per letter is megabytes nobody reads.
  const [asked] = useDebouncedValue(q, 300)
  const filters: ItemFilters = { q: asked, wearable, category, magic: kind === null ? null : kind === 'magic' }

  const [lastPage, setLastPage] = useState<ItemPage | null>(null)
  const [more, setMore] = useState<{ key: string; items: ItemHit[]; loading: boolean }>({ key: '', items: [], loading: false })
  const searchKey = JSON.stringify([scope, filters])
  const found = useResource(`items:${opened}:${searchKey}`, (signal) =>
    opened ? searchItems(filters, PAGE, 0, signal, scope) : Promise.resolve<ItemPage>({ items: [], total: 0, categories: [] }))
  // The last page stays up while the next search is in flight, so that typing
  // does not empty the list on every letter.
  if (opened && found.data !== null && found.data !== lastPage) setLastPage(found.data)
  const page = opened ? found.data ?? lastPage : null
  const loaded = [...(page?.items ?? []), ...(more.key === searchKey ? more.items : [])]
  async function loadMore() {
    setMore({ key: searchKey, items: more.key === searchKey ? more.items : [], loading: true })
    const settle = (items: ItemHit[]) => setMore((now) => now.key === searchKey ? { key: searchKey, items: [...now.items, ...items], loading: false } : now)
    try { settle((await searchItems(filters, PAGE, loaded.length, undefined, scope)).items) } catch { settle([]) }
  }

  if (!opened) {
    return <Stack gap="sm">
      {arrived && <Notification withBorder role="status" color="green" title={t('game.added')} onClose={() => setLast(null)}>{last.text}</Notification>}
      <Group gap="sm"><Button variant="light" disabled={disabled} onClick={() => { setLast(null); setOpened(true) }}>{label}</Button>{beside}</Group>
    </Stack>
  }
  return (
    <Stack component="section" aria-label={label} gap="md">
      <Group justify="space-between">
        <Text size="xs" c="dimmed">{label}</Text>
        <Button variant="default" onClick={() => setOpened(false)}>{t('common.close')}</Button>
      </Group>
      {children}
      <TextInput value={q} onChange={(event) => setQ(event.currentTarget.value)}
        placeholder={t('equipment.searchItems')} aria-label={t('equipment.searchItems')} />
      <Group gap="sm">
        <Select {...FILTER_WIDTH} aria-label={t('equipment.filter.category')} placeholder={t('equipment.filter.allCategories')}
          data={(page?.categories ?? []).map((each) => ({ value: each.slug, label: each.name }))}
          value={category} onChange={setCategory} clearable />
        <Select {...FILTER_WIDTH} aria-label={t('equipment.filter.kind')} placeholder={t('equipment.filter.allKinds')}
          data={[{ value: 'mundane', label: t('equipment.filter.mundane') }, { value: 'magic', label: t('equipment.filter.magic') }]}
          value={kind} onChange={setKind} clearable />
      </Group>
      {found.error !== null && <Text size="sm" c="red">{found.error}</Text>}
      {page !== null && (
        <Box aria-busy={found.loading} style={{ opacity: found.loading ? 0.55 : 1, transition: 'opacity 120ms' }}>
          <Stack gap="xs">
            <Text size="sm" c="dimmed">{t('equipment.itemCount', { count: page.total })}</Text>
            {loaded.length === 0 && <Text size="sm" c="dimmed">{t('equipment.noItemsFound')}</Text>}
            {loaded.map((hit) => (
              <ItemRow key={hit.slug} hit={hit} owned={owned.get(hit.slug) ?? 0} disabled={disabled}
                opened={reading === hit.slug} onOpen={(open) => setReading(open ? hit.slug : null)} onAdd={() => add(hit)}
                {...(detailsTo ? { to: detailsTo(hit) } : {})} />
            ))}
            {loaded.length < page.total && (
              <Group justify="center">
                <Button variant="light" loading={more.loading} onClick={() => void loadMore()}>{t('spells.loadMore')}</Button>
              </Group>
            )}
          </Stack>
        </Box>
      )}
    </Stack>
  )
}

/**
 * One found item: pressed, it opens to the full description; Add beside it
 * needs no opening. The shape of `SpellChoiceRow`, down to the description
 * being inline on desktop and a full screen on a phone.
 */
function ItemRow({ hit, owned, disabled, opened, onOpen, onAdd, to }: {
  to?: string
  hit: ItemHit
  owned: number
  disabled: boolean
  opened: boolean
  onOpen: (open: boolean) => void
  onAdd: () => void
}) {
  const t = useT()
  const addLabel = t('list.rowAction', { label: t('common.add'), name: hit.name })
  const facts = [
    hit.categoryName,
    hit.cost && `${hit.cost.amount} ${hit.cost.unit}`,
    hit.weight ? t('item.pounds', { value: hit.weight }) : undefined,
  ].filter(Boolean)
  const addButton = <Button variant="light" aria-label={addLabel} disabled={disabled} onClick={onAdd} style={{ flexShrink: 0 }}>{t('common.add')}</Button>
  const face = {
    'aria-label': hit.name, variant: 'transparent', color: 'var(--mantine-color-text)', h: 'auto', py: 4, px: 'xs', justify: 'flex-start',
    style: { flex: 1, minWidth: 0 }, styles: { label: { width: '100%', whiteSpace: 'normal' } },
  } as const
  const body = (
    <Group gap="xs" wrap="nowrap" align="flex-start" w="100%">
      <ItemIcon icon={hit.icon} />
      <Stack gap={2} style={{ textAlign: 'left', minWidth: 0, flex: 1 }}>
        <Group gap="xs">
          <Text size="sm" fw={600}>{hit.name}</Text>
          {owned > 0 && <Text size="sm" c="dimmed">×{owned}</Text>}
          {hit.magic && <Badge size="sm" variant="default">{t('equipment.magic')}</Badge>}
        </Group>
        {facts.length > 0 && <Text size="xs" c="dimmed">{facts.join(' · ')}</Text>}
      </Stack>
    </Group>
  )
  return (
    <Paper component="article" aria-label={hit.name} withBorder radius="sm" px="xs" py={4}>
      <Group gap="xs" wrap="nowrap" align="center">
        {to !== undefined
          ? <Button component={Link} to={to} {...face}>{body}</Button>
          : <Button aria-expanded={opened} onClick={() => onOpen(!opened)} {...face}>{body}</Button>}
        {addButton}
      </Group>
      {opened && (
        <ChoiceDetails title={hit.name} onBack={() => onOpen(false)} actions={addButton}>
          <ItemDescription hit={hit} />
        </ChoiceDetails>
      )}
    </Paper>
  )
}

/** The opened row's body, asked for when it is opened: a search hit carries a row's worth and no prose. */
function ItemDescription({ hit }: { hit: ItemHit }) {
  const t = useT()
  const scope = useCatalogScope()
  const loaded = useResource(`item-details:${scope}:${hit.slug}`, async () => {
    const [items, properties, damageTypes] = await Promise.all([
      getEntries<Item>(hit.magic ? 'magic-items' : 'equipment', [hit.slug], scope),
      getCollection<Entry>('weapon-properties', scope),
      getCollection<Entry>('damage-types', scope),
    ])
    return { item: items[0] ?? null, words: bySlug([...properties, ...damageTypes]) }
  })
  const state = pageState(loaded, { title: t('item.loadFailed'), fallback: t('error.unknown'), onRetry: loaded.reload })
  return (
    <PageBody state={state}>
      {loaded.data !== null && (loaded.data.item === null
        ? <Text size="sm" c="dimmed">{t('item.notFound')}</Text>
        : <ItemBody item={loaded.data.item} words={loaded.data.words} />)}
    </PageBody>
  )
}
