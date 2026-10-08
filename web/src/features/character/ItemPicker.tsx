import { useState } from 'react'

import { searchItems } from '@/lib/api'
import type { ItemHit, ItemPage } from '@/lib/api'
import { useCatalogScope } from '@/lib/api/catalogScope'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Button, Group, ModalSheet, Stack, Text, TextInput } from '@/ui'

const PAGE = 20

/**
 * The sheet's Add item picker: a name search over the character's catalogue,
 * equipment and magic items together, a page at a time -- the same shape as
 * the wizard's spell offer, because the compendium is never pulled whole.
 */
export function ItemPicker({ opened, onClose, onPick }: {
  opened: boolean
  onClose: () => void
  onPick: (hit: ItemHit) => void
}) {
  const t = useT()
  const scope = useCatalogScope()
  const [q, setQ] = useState('')
  const [lastPage, setLastPage] = useState<ItemPage | null>(null)
  const [more, setMore] = useState<{ key: string; items: ItemHit[]; loading: boolean }>({ key: '', items: [], loading: false })
  const searchKey = JSON.stringify([scope, q])
  const found = useResource(`items:${opened}:${searchKey}`, (signal) =>
    opened ? searchItems(q, PAGE, 0, signal, scope) : Promise.resolve<ItemPage>({ items: [], total: 0 }))
  // The last page stays up while the next search is in flight, so that typing
  // does not empty the list on every letter.
  if (found.data !== null && found.data !== lastPage) setLastPage(found.data)
  const page = found.data ?? lastPage
  const loaded = [...(page?.items ?? []), ...(more.key === searchKey ? more.items : [])]
  async function loadMore() {
    setMore({ key: searchKey, items: more.items, loading: true })
    const settle = (items: ItemHit[]) => setMore((now) => now.key === searchKey ? { key: searchKey, items: [...now.items, ...items], loading: false } : now)
    try { settle((await searchItems(q, PAGE, loaded.length, undefined, scope)).items) } catch { settle([]) }
  }

  return <ModalSheet opened={opened} onClose={onClose} title={t('equipment.addItem')}>
    <Stack gap="xs">
      <TextInput value={q} onChange={(event) => setQ(event.currentTarget.value)} placeholder={t('equipment.searchItems')} aria-label={t('equipment.searchItems')} />
      {found.error !== null && <Text size="sm" c="red">{found.error}</Text>}
      {page !== null && loaded.length === 0 && found.error === null && <Text size="sm" c="dimmed">{t('equipment.noItemsFound')}</Text>}
      {loaded.map((hit) => (
        <Button key={hit.slug} variant="default" justify="space-between" onClick={() => onPick(hit)}
          rightSection={<Group gap={6}>
            {hit.category && <Text size="xs" c="dimmed">{hit.category}</Text>}
            {hit.magic && <Text size="xs" c="dimmed">{t('equipment.magic')}</Text>}
          </Group>}>
          {hit.name}
        </Button>
      ))}
      {page !== null && loaded.length < page.total && (
        <Button variant="subtle" loading={more.loading} onClick={() => void loadMore()}>{t('spells.loadMore')}</Button>
      )}
    </Stack>
  </ModalSheet>
}
