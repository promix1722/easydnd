import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'

import type { Entry, Spell, SpellPage, SpellSearch } from '@/lib/api'
import { bySlug, getCollection, searchSpells } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import {
  Anchor,
  Badge,
  Box,
  Button,
  Checkbox,
  DataList,
  Group,
  Page,
  PageBody,
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
 *
 * **Two resources, not one, and the split is the whole design of this screen.**
 * `useResource` blanks itself when its key changes, which is right for a key
 * naming a different *thing* -- a different character, a different group. This
 * is the one screen in the app whose key carries adjustable filter state, and
 * there that behaviour is wrong: the schools and classes filling the Selects do
 * not depend on the search, so fetching them under a search-shaped key meant
 * every ticked checkbox threw away the controls that ticked it. The page-level
 * state then took the whole screen down to a spinner and rebuilt it -- filters,
 * search box, count and table -- to change which rows were in the table.
 *
 * So `options` is keyed on nothing and loads once, and only it gates the page.
 * `found` is keyed on the search and gates the results region alone, through
 * the same `PageBody` that `Page` would have used. The chrome never unmounts,
 * so the search box keeps its focus and its caret while you type into it.
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

  // What fills the Selects. Keyed on nothing, because it answers to nothing:
  // the list of schools and the list of classes are the same whatever is being
  // searched for. Both are served from the catalogue cache after the first
  // visit, so this is usually not a request at all.
  const options = useResource('spells:options', async () => {
    const [schools, classes] = await Promise.all([
      getCollection<Entry>('magic-schools'),
      getCollection<Entry>('classes'),
    ])
    return { schools, classes }
  })

  const found = useResource(`spells:${searchKey}`, (signal) => searchSpells(search, signal))

  // The last page that arrived, held across the gap while the next one is in
  // flight. Without it the table would empty itself on every keystroke pause
  // and every ticked box, which is the same flicker one resource ago -- just
  // confined to the table. With it the old rows stay under a dimmed panel and
  // are replaced when the new ones land.
  const [lastPage, setLastPage] = useState<SpellPage | null>(null)
  if (found.data !== null && found.data !== lastPage) setLastPage(found.data)

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

  const state = pageState(options, {
    title: t('spells.loadFailed'),
    fallback: t('error.unknown'),
    onRetry: options.reload,
  })
  if (state.kind !== 'ready' || options.data === null) {
    return <Page trail={[]} state={state} />
  }

  const { schools, classes } = options.data
  const schoolNames = bySlug(schools)

  // The results region's own state. It is `loading` only until the first page
  // has ever arrived: after that a slow search dims what is on screen rather
  // than removing it, which is what `lastPage` is for.
  const page = found.data ?? lastPage
  const results = pageState(
    { data: page, error: found.error, loading: found.loading && page === null },
    { title: t('spells.loadFailed'), fallback: t('error.unknown'), onRetry: found.reload },
  )
  const rows = page === null ? [] : [...page.spells, ...extra]

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

          {/* Everything below here, and nothing above it, answers to the
              search. `found.loading` dims it rather than replacing it: the
              rows on screen are the previous answer, not a wrong one, and a
              spinner where they were is what this screen used to do to the
              whole page. */}
          <PageBody state={results}>
            {page !== null && (
              <Box
                aria-busy={found.loading}
                style={{ opacity: found.loading ? 0.55 : 1, transition: 'opacity 120ms' }}
              >
                <Stack gap="md">
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
              </Box>
            )}
          </PageBody>
        </Stack>
      </Panel>
    </Page>
  )
}
