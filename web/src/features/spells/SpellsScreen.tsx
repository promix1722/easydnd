import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'

import type { Entry, Spell, SpellPage, SpellSearch } from '@/lib/api'
import { bySlug, getCollection, getSpellBrowseOptions, searchSpells, sourceOptions } from '@/lib/api'
import { useSpellIconGeneration } from './useSpellIconGeneration'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  Alert,
  SourceTags,
  Anchor,
  Badge,
  Box,
  Button,
  DataList,
  Group,
  IconWand,
  Page,
  PageBody,
  Panel,
  Stack,
  Text,
  pageState,
} from '@/ui'

import { SpellFilters } from './SpellFilters'
import type { SpellFilterValues } from './filterSpells'
import { SpellIconTools, SpellIconJobBadge } from './SpellIconTools'
import type { SpellIconTarget } from './SpellIconTools'
import { SpellTags } from './SpellTags'
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

export function SpellsScreen() {
  const t = useT()
  const [params, setParams] = useSearchParams()
  const pendingParams = useRef(params)
  useEffect(() => { pendingParams.current = params }, [params])

  function changeParams(update: (next: URLSearchParams) => void) {
    // useSearchParams does not queue functional updates. Keep the latest
    // requested URL until navigation commits so quick edits accumulate.
    const next = new URLSearchParams(pendingParams.current)
    update(next)
    pendingParams.current = next
    setParams(next, { replace: true })
  }
  const packQuery = params.get('packs') ?? ''
  const scope = packQuery ? `/packs/catalog?packs=${encodeURIComponent(packQuery)}` : 'browse'
  const versions = params.get('versions') ?? ''
  const packIds = (params.get('pack')?.split(',').filter(Boolean) ?? []).slice(-1)
  const sources = params.get('source')?.split(',').filter(Boolean) ?? []
  const spellURL = (spell: Spell) => {
    const context = spell.catalogPacks ?? packQuery
    return `/spells/${encodeURIComponent(spell.slug)}${context ? `?packs=${encodeURIComponent(context)}` : ''}`
  }

  function setParam(key: string, value: string | null) {
    changeParams((next) => {
      if (value === null || value === '') next.delete(key)
      else next.set(key, value)
    })
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

  function updateFilters(value: SpellFilterValues) {
    setDraft(value.query)
    // Router navigation may still be pending when another control changes.
    // Only write fields changed by this interaction so it cannot overwrite
    // another control's newer URL value with a previous render's value.
    const current = {
      pack: packIds.join(',') || null, source: sources.join(',') || null,
      level, school, class: casterClass, time,
      conc: concentration ? '1' : null,
      ritual: ritual ? '1' : null,
      nomat: noMaterial ? '1' : null,
    }
    changeParams((next) => {
      const fields = {
        pack: value.packIds?.join(',') || null, source: value.sources?.join(',') || null,
        level: value.level, school: value.school, class: value.casterClass, time: value.time,
        conc: value.concentration ? '1' : null,
        ritual: value.ritual ? '1' : null,
        nomat: value.noMaterial ? '1' : null,
      }
      for (const [key, field] of Object.entries(fields)) {
        if (field === current[key as keyof typeof current]) continue
        if (field === null) next.delete(key)
        else next.set(key, field)
      }
    })
  }

  // What the current filters ask the catalogue for, minus this screen's own
  // paging. `filters` is kept separate because the dev icon generator takes
  // the same shape -- it pages the whole filtered set server-side itself.
  const filters: SpellSearch = {
    ...(query === '' ? {} : { q: query }),
    ...(level === null ? {} : { level: Number(level) }),
    ...(school === null ? {} : { school }),
    ...(casterClass === null ? {} : { class: casterClass }),
    ...(time === null ? {} : { castingTime: time }),
    ...(concentration ? { concentration: true } : {}),
    ...(ritual ? { ritual: true } : {}),
    ...(noMaterial ? { material: false } : {}),
    ...(packIds.length ? { pack: packIds.join(',') } : {}),
    ...(sources.length ? { source: sources.join(',') } : {}),
    ...(scope === 'browse' && versions ? { versions } : {}),
  }
  const search: SpellSearch = { ...filters, limit: PAGE_SIZE }

  // The dev server's icon queue: fetched at mount and polled while running,
  // inert when import.meta.env.DEV folds to a literal false.
  const icons = useSpellIconGeneration()
  const [iconTarget, setIconTarget] = useState<SpellIconTarget | null>(null)
  const [iconToolsVisible, setIconToolsVisible] = useState(false)
  useEffect(() => {
    if (!import.meta.env.DEV) return undefined
    function toggleIconTools(event: KeyboardEvent) {
      // Physical U also works with the Russian keyboard layout.
      if (event.code !== 'KeyU' || !event.metaKey || !event.shiftKey || event.ctrlKey || event.altKey) return
      event.preventDefault()
      if (event.repeat || event.isComposing) return
      setIconToolsVisible((visible) => !visible)
      setIconTarget(null)
    }
    window.addEventListener('keydown', toggleIconTools)
    return () => window.removeEventListener('keydown', toggleIconTools)
  }, [])
  const searchKey = JSON.stringify([scope, search])
  const activeSearch = useRef(searchKey)
  useEffect(() => { activeSearch.current = searchKey }, [searchKey])

  // What fills the Selects. Keyed on nothing, because it answers to nothing:
  // the list of schools and the list of classes are the same whatever is being
  // searched for. Both are served from the catalogue cache after the first
  // visit, so this is usually not a request at all.
  const options = useResource(`spells:options:${scope}:${versions}`, async () => {
    if (scope === 'browse') return getSpellBrowseOptions(versions)
    const [schools, classes, spells] = await Promise.all([
      getCollection<Entry>('magic-schools', scope), getCollection<Entry>('classes', scope), getCollection<Spell>('spells', scope),
    ])
    return { schools, classes, ...sourceOptions(spells), unavailable: [] }
  })
  useEffect(() => {
    if (!options.data) return
    // Old multi-pack URLs retain the last choice, like selecting a new pack.
    if (params.get('pack') && params.get('pack') !== packIds.join(',')) setParam('pack', packIds.join(','))
    const valid = sources.filter((id) => options.data?.sources.some((s) => s.id === id && (!packIds.length || packIds.includes(s.packId))))
    if (valid.length !== sources.length) setParam('source', valid.join(','))
    // Source options follow the requested release; keep unrelated URL filters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [options.data, params.get('pack'), params.get('source')])

  const found = useResource(`spells:${searchKey}`, (signal) => searchSpells(search, signal, scope))

  // The last page that arrived, held across the gap while the next one is in
  // flight. Without it the table would empty itself on every keystroke pause
  // and every ticked box, which is the same flicker one resource ago -- just
  // confined to the table. With it the old rows stay under a dimmed panel and
  // are replaced when the new ones land.
  const [lastPage, setLastPage] = useState<SpellPage | null>(null)
  const [lastScope, setLastScope] = useState(scope)
  if (lastScope !== scope) { setLastScope(scope); setLastPage(null) }
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
    return <Page trail={[]}><PageBody state={state}>{null}</PageBody></Page>
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
      const next = await searchSpells({ ...search, offset: rows.length }, undefined, scope)
      if (activeSearch.current === searchKey) setExtra((previous) => [...previous, ...next.spells])
    } catch {
      if (activeSearch.current === searchKey) found.reload()
    } finally {
      setLoadingMore(false)
    }
  }

  return (
    <Page trail={[]}>

      <Panel>
        <Stack gap="md">
          <SpellFilters
            value={{ query: draft, level, school, casterClass, time, concentration, ritual, noMaterial, packIds, sources }}
            sourceOptions={options.data}
            {...(scope === 'browse' ? { onVersionChange: (pack: string, version: string) => {
              const selected = new Map(versions.split(',').filter(Boolean).map((s) => s.split('@') as [string, string]))
              selected.set(pack, version)
              setParam('versions', [...selected].map(([id, v]) => `${id}@${v}`).join(','))
            } } : {})}
            onChange={updateFilters}
            schools={schools}
            classes={classes}
          />

          {options.data.unavailable.length > 0 && <Alert color="orange">{t('spells.unavailablePacks')} {options.data.unavailable.map((p) => `${p.id}@${p.version}`).join(', ')}</Alert>}
          {/* Development-only artwork tooling: generates spell icons through
              a development route on the service, behind this screen's own
              filters. The guard is what a production bundle eliminates --
              the component and its queue talk to a route that is not
              registered there. */}
          {import.meta.env.DEV && iconToolsVisible && (
            <SpellIconTools
              generation={icons}
              total={page?.total ?? 0}
              bulk={{ scope, search: filters }}
              available={!found.loading && found.error === null && page !== null && draft.trim() === query}
              target={iconTarget}
              onTarget={setIconTarget}
              onCloseTarget={() => setIconTarget(null)}
            />
          )}
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
                    getKey={(spell) => `${spell.provenance?.packId}@${spell.provenance?.version}/${spell.slug}`}
                    leading={(spell) => {
                      // The queue reports a fresh revision when new art lands;
                      // it cache-busts the URL and re-arms a row that hid.
                      const revision = import.meta.env.DEV ? icons.state?.items?.[spell.slug]?.revision : undefined
                      return <SpellIcon slug={spell.slug} size={32} {...(revision === undefined ? {} : { revision })} />
                    }}
                    badges={(spell) => {
                      const job = import.meta.env.DEV ? icons.state?.items?.[spell.slug] : undefined
                      return (
                        <>
                          <SpellTags spell={spell} />
                          {import.meta.env.DEV && iconToolsVisible && job !== undefined && <SpellIconJobBadge job={job} />}
                        </>
                      )
                    }}
                    actions={(spell) =>
                      // Development-only -- the route it asks for is not
                      // registered in production, so a production bundle
                      // draws no actions column on this table at all.
                      import.meta.env.DEV && iconToolsVisible
                        ? [
                            {
                              key: 'icon',
                              label: t('spells.icons.generate'),
                              icon: <IconWand size={ACTION_ICON_SIZE} />,
                              onClick: () =>
                                setIconTarget({
                                  name: spell.name,
                                  request: {
                                    // The catalogue that listed this row, not
                                    // necessarily the screen's scope: a browse
                                    // row can belong to a selected pack.
                                    scope: (spell.catalogPacks ?? packQuery)
                                      ? `/packs/catalog?packs=${encodeURIComponent(spell.catalogPacks ?? packQuery)}`
                                      : 'browse',
                                    search: {},
                                    slug: spell.slug,
                                    replace: true,
                                  },
                                }),
                            },
                          ]
                        : []
                    }
                    columns={[
                      {
                        key: 'name',
                        header: t('spells.name'),
                        primary: true,
                        text: (spell) => spell.name,
                        to: (spell) => spellURL(spell),
                        render: (spell) => (
                          <Anchor component={Link} to={spellURL(spell)}>
                            <Text size="sm">{spell.name}</Text>
                          </Anchor>
                        ),
                      },
                      {
                        key: 'source',
                        header: t('spells.filter.source'),
                        slot: 'block',
                        render: (spell) => <SourceTags provenance={spell.provenance} />,
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
