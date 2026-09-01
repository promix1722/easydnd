import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'

import type { Entry, Spell, SpellSearch } from '@/lib/api'
import { bySlug, getCollection, searchSpells } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import {
  Anchor,
  Badge,
  Button,
  Checkbox,
  DataList,
  Group,
  Page,
  Panel,
  Select,
  Stack,
  Text,
  TextInput,
  pageState,
  useIsDesktop,
} from '@/ui'

import { SpellIcon } from './spellIcon'
import { castingTimeText, componentsAbbrev, levelText } from './spellText'

/**
 * The compendium's spells, searchable and filterable.
 *
 * Searching, filtering, sorting and paging all happen server-side -- see
 * internal/api/http/v1/catalog/search.go -- so this screen only says what it
 * wants and appends pages. The filters live in the URL rather than in state
 * so a filtered list can be shared and survives a reload, the same way the
 * character list carries `?folder=`.
 */

const PAGE_SIZE = 50
const LEVELS = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
const CASTING_TIMES = ['action', 'bonus-action', 'reaction', 'over-time']

export function SpellsScreen() {
  const t = useT()
  const [params, setParams] = useSearchParams()
  // A 390px card cannot host a name and two word-length badges, so the phone
  // gets one-letter marks with the full word as the accessible name.
  const isDesktop = useIsDesktop()

  function setParam(key: string, value: string | null) {
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous)
        if (value === null || value === '') next.delete(key)
        else next.set(key, value)
        return next
      },
      { replace: true },
    )
  }

  const query = (params.get('q') ?? '').trim()
  const level = params.get('level')
  const school = params.get('school')
  const casterClass = params.get('class')
  const time = params.get('time')
  const concentration = params.get('conc') === '1'
  const ritual = params.get('ritual') === '1'
  const noMaterial = params.get('nomat') === '1'

  // The search box writes to the URL through a short pause, not per
  // keystroke: the URL is the request now, and firing one per letter would
  // race four requests to answer the word.
  const [draft, setDraft] = useState(query)
  useEffect(() => {
    const trimmed = draft.trim()
    if (trimmed === query) return undefined
    const handle = setTimeout(() => setParam('q', trimmed === '' ? null : trimmed), 300)
    return () => clearTimeout(handle)
    // setParam is recreated per render; the timer only needs draft and query.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft, query])

  const search: SpellSearch = {
    ...(query === '' ? {} : { q: query }),
    ...(level === null ? {} : { level: Number(level) }),
    ...(school === null ? {} : { school }),
    ...(casterClass === null ? {} : { class: casterClass }),
    ...(time === null ? {} : { castingTime: time }),
    ...(concentration ? { concentration: true } : {}),
    ...(ritual ? { ritual: true } : {}),
    ...(noMaterial ? { material: false } : {}),
    limit: PAGE_SIZE,
  }
  const searchKey = JSON.stringify(search)

  const catalog = useResource(`spells:${searchKey}`, async (signal) => {
    const [page, schools, classes] = await Promise.all([
      searchSpells(search, signal),
      getCollection<Entry>('magic-schools'),
      getCollection<Entry>('classes'),
    ])
    return { page, schools, classes }
  })

  // Appended pages, reset during render when the search changes -- the same
  // shape useResource uses for its own key, and for the same reason: an
  // effect would paint the old rows once under the new search.
  const [extra, setExtra] = useState<Spell[]>([])
  const [shownKey, setShownKey] = useState(searchKey)
  const [loadingMore, setLoadingMore] = useState(false)
  if (shownKey !== searchKey) {
    setShownKey(searchKey)
    setExtra([])
  }

  const state = pageState(catalog, {
    title: t('spells.loadFailed'),
    fallback: t('error.unknown'),
    onRetry: catalog.reload,
  })
  if (state.kind !== 'ready' || catalog.data === null) {
    return <Page trail={[]} state={state} />
  }

  const { page, schools, classes } = catalog.data
  const schoolNames = bySlug(schools)
  const rows = [...page.spells, ...extra]

  async function loadMore() {
    setLoadingMore(true)
    try {
      const next = await searchSpells({ ...search, offset: rows.length })
      setExtra((previous) => [...previous, ...next.spells])
    } finally {
      setLoadingMore(false)
    }
  }

  return (
    <Page trail={[]}>
      <Panel>
        <Stack gap="md">
          <TextInput
            aria-label={t('spells.search')}
            placeholder={t('spells.search')}
            value={draft}
            onChange={(event) => setDraft(event.currentTarget.value)}
          />
          <Group gap="sm">
            <Select
              aria-label={t('spells.filter.level')}
              placeholder={t('spells.filter.allLevels')}
              data={LEVELS.map((value) => ({ value: String(value), label: levelText(t, value) }))}
              value={level}
              onChange={(value) => setParam('level', value)}
              clearable
            />
            <Select
              aria-label={t('spells.filter.school')}
              placeholder={t('spells.filter.allSchools')}
              data={schools.map((entry) => ({ value: entry.slug, label: entry.name }))}
              value={school}
              onChange={(value) => setParam('school', value)}
              clearable
            />
            <Select
              aria-label={t('spells.filter.class')}
              placeholder={t('spells.filter.allClasses')}
              data={classes.map((entry) => ({ value: entry.slug, label: entry.name }))}
              value={casterClass}
              onChange={(value) => setParam('class', value)}
              clearable
            />
            <Select
              aria-label={t('spells.filter.castingTime')}
              placeholder={t('spells.filter.anyTime')}
              data={CASTING_TIMES.map((kind) => ({
                value: kind,
                label:
                  kind === 'over-time'
                    ? t('spells.filter.overTime')
                    : castingTimeText(t, { kind }),
              }))}
              value={time}
              onChange={(value) => setParam('time', value)}
              clearable
            />
          </Group>
          <Group gap="md">
            <Checkbox
              label={t('spells.filter.concentration')}
              checked={concentration}
              onChange={(event) => setParam('conc', event.currentTarget.checked ? '1' : null)}
            />
            <Checkbox
              label={t('spells.filter.ritual')}
              checked={ritual}
              onChange={(event) => setParam('ritual', event.currentTarget.checked ? '1' : null)}
            />
            <Checkbox
              label={t('spells.filter.noMaterial')}
              checked={noMaterial}
              onChange={(event) => setParam('nomat', event.currentTarget.checked ? '1' : null)}
            />
          </Group>

          <Text size="sm" c="dimmed">
            {t('spells.count', { count: page.total })}
          </Text>

          <DataList
            items={rows}
            getKey={(spell) => spell.slug}
            leading={(spell) => <SpellIcon slug={spell.slug} size={32} />}
            badges={(spell) => (
              <>
                {spell.concentration === true && (
                  <Badge size="sm" variant="light" aria-label={t('spell.concentration')}>
                    {isDesktop ? t('spell.concentration') : t('spell.concentrationShort')}
                  </Badge>
                )}
                {spell.ritual === true && (
                  <Badge size="sm" variant="light" color="grape" aria-label={t('spell.ritual')}>
                    {isDesktop ? t('spell.ritual') : t('spell.ritualShort')}
                  </Badge>
                )}
              </>
            )}
            columns={[
              {
                key: 'name',
                header: t('spells.name'),
                primary: true,
                text: (spell) => spell.name,
                to: (spell) => `/spells/${spell.slug}`,
                render: (spell) => (
                  <Anchor component={Link} to={`/spells/${spell.slug}`}>
                    <Text size="sm">{spell.name}</Text>
                  </Anchor>
                ),
              },
              {
                key: 'level',
                header: t('spells.filter.level'),
                slot: 'badge',
                render: (spell) => (
                  <Badge size="sm" variant="default">
                    {levelText(t, spell.level)}
                  </Badge>
                ),
              },
              {
                key: 'school',
                header: t('spells.filter.school'),
                render: (spell) => schoolNames.get(spell.school ?? '')?.name ?? '',
              },
              {
                key: 'castingTime',
                header: t('spell.castingTime'),
                render: (spell) => castingTimeText(t, spell.castingTime),
              },
              {
                key: 'components',
                header: t('spell.components'),
                render: (spell) => componentsAbbrev(t, spell.components),
              },
            ]}
            empty={t('spells.empty')}
          />

          {rows.length < page.total && (
            <Group justify="center">
              <Button variant="light" loading={loadingMore} onClick={() => void loadMore()}>
                {t('spells.loadMore')}
              </Button>
            </Group>
          )}
        </Stack>
      </Panel>
    </Page>
  )
}
