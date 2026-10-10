import { useState } from 'react'

import { collectionOfKind, kindOf, slugOf, titleCase } from '@/domain'
import { appendEvents, bySlug, deleteEvent, getEntries, getEvents, getPrompts, getSpellFilterOptions, replaceEvent, searchSpellOffer } from '@/lib/api'
import type { CharacterEvent, Entry, Prompt, Sheet, Spell, SpellOfferSearch, SpellPage } from '@/lib/api'
import type { SpellSource } from '@/lib/api/characters'
import { useCatalogScope } from '@/lib/api/catalogScope'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Button, Group, Panel, Stack, Text, Title } from '@/ui'
import { SpellFilters } from '@/features/spells/SpellFilters'
import { EMPTY_SPELL_FILTERS, hasSpellFilters, spellFilterSearch } from '@/features/spells/filterSpells'
import { levelText } from '@/features/spells/spellText'

import { SpellChoiceRow } from './SpellChoiceRow'
import { spellChoiceName } from './promptNames'

const MODES = ['cantrips', 'known', 'spellbook', 'prepared', 'arcanum', 'mastery'] as const
const PAGE = 20

/**
 * The question a preparing class answers, found again from the sheet.
 *
 * The server poses `<class>/spell/prepared/<level>` once per class level and
 * stops posing it once answered; the answer is one entry in the log. Changing
 * what is prepared is therefore replacing that entry, or appending it the
 * first time, or deleting it when the last spell is unprepared -- the same
 * three writes the build screen makes, without its draft.
 */
interface Preparation {
  source: string
  prompt: Prompt
  replaces?: { seq: number; event: CharacterEvent }
  head: { seq: number; revision: number }
}

interface Row {
  slug: string
  label: string
  reason?: string
  isSelected: boolean
  disabled?: boolean
  onToggle?: () => void
}

/**
 * The sheet's Spells tab: what the character knows, drawn as the build
 * screen's rows -- icon, level, tags, school, and the spell's full text when a
 * row is opened, inline on a wide screen and as a page of its own on a phone.
 *
 * A class that prepares spells also gets to prepare them here. The owner's
 * screen passes `characterId`; without it (a sheet shared with a table) every
 * row is read-only. Each Add or Remove is one write, as taking off a piece of
 * equipment is: preparing is something done at a table after a rest, not a
 * form to fill in and submit.
 */
export function SheetSpells({ sheet, characterId, onChanged }: {
  sheet: Sheet
  /** The character being edited. Absent on a sheet that is only being read. */
  characterId?: string | undefined
  /** Called after a write landed, so the screen refreshes the sheet. */
  onChanged?: (() => void) | undefined
}) {
  const t = useT()
  const scope = useCatalogScope()
  const sources = sheet.spells.sources ?? []
  const names = new Map(Object.entries(sheet.catalogNames ?? {}))
  const spells = bySlug(sheet.catalog?.spells ?? [])
  const named = (collection: string, slug: string) =>
    names.get(`${collection}:${slug}`) ?? (collection === 'spells' ? spells.get(slug)?.name : undefined) ?? titleCase(slug)

  // One spell open at a time, across every list on the tab. Its full text is
  // fetched when it opens: the sheet arrives with summaries only.
  const [opened, setOpened] = useState<{ key: string; slug: string } | null>(null)
  const details = useResource(`sheet-spell-open:${scope}:${opened?.slug ?? ''}`, () =>
    opened === null ? Promise.resolve([]) : getEntries<Spell>('spells', [opened.slug], scope))
  // School and class names, for the row's facts line and the detail's class list.
  const options = useResource(`sheet-spell-options:${scope}`, () => getSpellFilterOptions(scope))
  const entries = new Map<string, Entry>([...(options.data?.schools ?? []), ...(options.data?.classes ?? []), ...spells.values(), ...(details.data ?? [])]
    .map((entry) => [entry.slug, entry] as const))

  const preparing = characterId === undefined ? [] : sources.filter((source) => (source.preparationLimit ?? 0) > 0)
  const questions = useResource(`sheet-prepare:${characterId ?? ''}:${preparing.map((source) => source.source).join(',')}`, async (signal) => {
    if (characterId === undefined || preparing.length === 0) return []
    const log = await getEvents(characterId, signal)
    const head = { seq: log.seq, revision: log.revision ?? log.seq }
    let open: Prompt[] | null = null
    const found = await Promise.all(preparing.map(async (source): Promise<Preparation | null> => {
      const level = sheet.identity.classes?.find((taken) => taken.class === source.class)?.level ?? 0
      const id = `${slugOf(source.source)}/spell/prepared/${level}`
      const saved = log.events.find((event) => event.choices?.some((answer) => answer.prompt === id))
      if (saved?.seq !== undefined) {
        const prompt = (await getPrompts(characterId, signal, saved.seq)).prompts.find((each) => each.choice.prompt === id)
        return prompt === undefined ? null : { source: source.source, prompt, replaces: { seq: saved.seq, event: saved }, head }
      }
      open ??= (await getPrompts(characterId, signal)).prompts
      const prompt = open.find((each) => each.choice.prompt === id)
      return prompt === undefined ? null : { source: source.source, prompt, head }
    }))
    return found.filter((each): each is Preparation => each !== null)
  })

  // ponytail: no dry-run preview before the replace -- nothing in the log
  // depends on a preparation answer, so there is never anything to drop.
  const prepare = useAction(async (question: Preparation, picks: string[]) => {
    if (characterId === undefined) return
    const { prompt, replaces, head } = question
    const id = prompt.choice.prompt
    const event: CharacterEvent = {
      type: prompt.event.type,
      ...(prompt.event.ref !== undefined ? { ref: prompt.event.ref } : {}),
      ...(prompt.event.level !== undefined ? { level: prompt.event.level } : {}),
      choices: (replaces?.event.choices ?? [{ prompt: id, picks: [] }]).map((answer) => ({ prompt: answer.prompt, picks: answer.prompt === id ? picks : answer.picks })),
    }
    if (replaces === undefined) await appendEvents(characterId, head.seq, [event], head.revision)
    else if (picks.length === 0) await deleteEvent(characterId, replaces.seq, head.seq, false, head.revision)
    else await replaceEvent(characterId, replaces.seq, head.seq, event, false, head.revision)
    onChanged?.()
    questions.refresh()
  })

  // Nothing is disabled while a write is in flight: forty rows going grey for
  // half a second is a blink, not feedback. A second press before the first
  // lands is simply ignored.
  const open = (key: string, slug: string, isOpen: boolean) => setOpened(isOpen ? { key, slug } : null)
  const row = (slug: string, extra: Partial<Row> = {}): Row => ({ slug, label: named('spells', slug), isSelected: false, ...extra })
  const draw = (prefix: string, rows: Row[], headed: boolean) => <LevelGroups prefix={prefix} rows={rows} headed={headed} entries={entries} opened={opened?.key ?? null} onOpen={open} />

  return <Stack gap="md">
    {prepare.error !== null && <Text size="sm" c="red">{prepare.error}</Text>}
    {questions.error !== null && <Group><Text c="red">{questions.error}</Text><Button onClick={questions.reload}>{t('page.retry')}</Button></Group>}
    {sources.map((source) => {
      const question = questions.data?.find((each) => each.source === source.source)
      const known = new Set(source.known ?? [])
      return <Panel key={source.source}><Stack gap="md">
        <Text fw={600}>{source.source.startsWith('rule:custom-spells') ? t('spellRules.custom') : named(collectionOfKind(kindOf(source.source)) ?? 'classes', slugOf(source.source))}</Text>
        {MODES.map((mode) => {
          const list = source[mode] ?? []
          if (mode === 'prepared' && question !== undefined) return <PreparedSection key={mode} source={source} question={question} entries={entries}
            schools={options.data?.schools ?? []} classes={options.data?.classes ?? []}
            opened={opened?.key ?? null} onOpen={open} named={named} onPrepare={(picks) => { if (!prepare.pending) void prepare.run(question, picks) }} />
          // A known caster's prepared list is its known list said twice.
          if (list.length === 0 || mode === 'prepared' && source.preparationLimit === undefined && list.every((slug) => known.has(slug))) return null
          const title = spellChoiceName(t, mode === 'cantrips' ? 'cantrip' : mode, mode === 'prepared' ? source.preparationLimit ?? list.length : list.length)
          return <Stack key={mode} component="section" aria-label={title} gap="xs">
            <Text size="xs" c="dimmed" tt="uppercase">{title}</Text>
            {draw(`${source.source}:${mode}`, list.map((slug) => row(slug)), mode !== 'cantrips')}
          </Stack>
        })}
      </Stack></Panel>
    })}
  </Stack>
}

/** Rows under a heading per spell level, as the build screen's selected list. */
function LevelGroups({ prefix, rows, headed, entries, opened, onOpen }: {
  prefix: string
  rows: Row[]
  headed: boolean
  entries: ReadonlyMap<string, Entry>
  opened: string | null
  onOpen: (key: string, slug: string, isOpen: boolean) => void
}) {
  const t = useT()
  const levelOf = (slug: string) => (entries.get(slug) as Spell | undefined)?.level ?? -1
  const levels = [...new Set(rows.map((each) => levelOf(each.slug)))].sort((a, b) => a - b)
  return <>
    {levels.map((level) => <Stack key={level} component="section" gap="xs" {...(headed && level > 0 ? { 'aria-label': levelText(t, level) } : {})}>
      {headed && level > 0 && <Title order={4}>{levelText(t, level)}</Title>}
      {rows.filter((each) => levelOf(each.slug) === level).sort((a, b) => a.label.localeCompare(b.label)).map((each) => {
        const key = `${prefix}:${each.slug}`
        return <SpellChoiceRow key={key} option={{ key: each.slug, label: each.label, disabled: false, ...(each.reason !== undefined ? { reason: each.reason } : {}) }}
          slug={each.slug} spell={entries.get(each.slug) as Spell | undefined} entries={entries} isSelected={each.isSelected} pending={false} disabled={each.disabled ?? false}
          opened={opened === key} onOpen={(isOpen) => onOpen(key, each.slug, isOpen)} onToggle={each.onToggle} />
      })}
    </Stack>)}
  </>
}

/**
 * What is prepared, with Remove, and what could be, with Add.
 *
 * The pool is the question's options -- the class list up to the level the
 * class can cast, or a wizard's spellbook -- searched, filtered and paged by
 * the server as the build screen's list is. Spells the rules prepare on their
 * own (a domain's) are in the prepared list but not in the question, and are
 * drawn without a Remove.
 */
function PreparedSection({ source, question, entries, schools, classes, opened, onOpen, named, onPrepare }: {
  source: SpellSource
  question: Preparation
  entries: ReadonlyMap<string, Entry>
  schools: readonly Entry[]
  classes: readonly Entry[]
  opened: string | null
  onOpen: (key: string, slug: string, isOpen: boolean) => void
  named: (collection: string, slug: string) => string
  onPrepare: (picks: string[]) => void
}) {
  const t = useT()
  const scope = useCatalogScope()
  const { prompt } = question
  const options = prompt.choice.from.options ?? []
  const slugFor = new Map<string, string>(options.map((option) => [option.key, slugOf(option.ref ?? option.key)]))
  const keyFor = new Map<string, string>([...slugFor].map(([key, slug]) => [slug, key]))
  // What is prepared by choice is read off the sheet, not the saved entry:
  // the sheet is what a write refreshes first, and a row that flipped to
  // "always prepared" until the question was read again would be a lie.
  const picks = (source.prepared ?? []).flatMap((slug) => { const key = keyFor.get(slug); return key === undefined ? [] : [key] })
  const limit = prompt.choice.choose
  const title = spellChoiceName(t, 'prepared', limit)

  const [filters, setFilters] = useState(EMPTY_SPELL_FILTERS)
  const [more, setMore] = useState<{ key: string; spells: Spell[]; loading: boolean }>({ key: '', spells: [], loading: false })
  const search: SpellOfferSearch = { ...spellFilterSearch(filters), only: { slugs: [...slugFor.values()], fitting: [] }, exclude: picks.map((key) => slugFor.get(key) ?? key), limit: PAGE }
  const searchKey = JSON.stringify([scope, search])
  const found = useResource(`sheet-prepare-offer:${searchKey}`, (signal) => searchSpellOffer(search, signal, scope))
  // The last page stays up while the next is fetched -- after an Add, or a
  // keystroke in the filter -- so the list never empties under the reader and
  // the page never shrinks out from under their scroll position.
  const [lastPage, setLastPage] = useState<SpellPage | null>(null)
  if (found.data !== null && found.data !== lastPage) setLastPage(found.data)
  const page: SpellPage | null = found.data ?? lastPage
  // A spell the sheet already says is prepared leaves this list at once,
  // rather than when the server's next page (which omits it) arrives.
  const loaded = [...(page?.spells ?? []), ...(more.key === searchKey ? more.spells : [])].filter((spell) => !picks.includes(keyFor.get(spell.slug) ?? spell.slug))
  async function loadMore() {
    setMore({ key: searchKey, spells: loaded.slice(page?.spells.length ?? 0), loading: true })
    const settle = (spells: Spell[]) => setMore((now) => now.key === searchKey ? { key: searchKey, spells: [...now.spells, ...spells], loading: false } : now)
    try { settle((await searchSpellOffer({ ...search, offset: loaded.length }, undefined, scope)).spells) } catch { settle([]) }
  }

  const prepared: Row[] = (source.prepared ?? []).map((slug) => {
    const key = keyFor.get(slug)
    return key === undefined || !picks.includes(key)
      ? { slug, label: named('spells', slug), isSelected: false, reason: t('sheet.alwaysPrepared') }
      : { slug, label: named('spells', slug), isSelected: true, onToggle: () => onPrepare(picks.filter((each) => each !== key)) }
  })
  const full = picks.length >= limit
  const withPage = new Map<string, Entry>([...entries, ...loaded.filter((spell) => !entries.has(spell.slug)).map((spell) => [spell.slug, spell] as const)])
  const available: Row[] = loaded.map((spell) => {
    const key = keyFor.get(spell.slug) ?? spell.slug
    return { slug: spell.slug, label: spell.name, isSelected: false, disabled: full, onToggle: () => onPrepare([...picks, key]) }
  })

  return <>
    <Stack component="section" aria-label={title} gap="xs">
      <Group justify="space-between">
        <Text size="xs" c="dimmed" tt="uppercase">{title}</Text>
        <Text size="sm" c="dimmed">{t('prompt.spellsSelected', { count: picks.length, total: limit })}</Text>
      </Group>
      {prepared.length === 0 && <Text size="sm" c="dimmed">{t('sheet.none')}</Text>}
      <LevelGroups prefix={`${source.source}:prepared`} rows={prepared} headed entries={withPage} opened={opened} onOpen={onOpen} />
    </Stack>
    <Stack component="section" aria-label={t('prompt.availableSpells')} gap="xs">
      <Text size="xs" c="dimmed" tt="uppercase">{t('prompt.availableSpells')}</Text>
      <SpellFilters value={filters} onChange={setFilters} schools={schools} classes={classes} />
      {found.error !== null && <Group><Text c="red">{found.error}</Text><Button onClick={found.reload}>{t('page.retry')}</Button></Group>}
      <Group justify="space-between">
        <Text size="sm" c="dimmed" aria-live="polite">{t('spells.count', { count: page?.total ?? 0 })}</Text>
        {hasSpellFilters(filters) && <Button variant="subtle" onClick={() => setFilters(EMPTY_SPELL_FILTERS)}>{t('prompt.resetSpellFilters')}</Button>}
      </Group>
      <LevelGroups prefix={`${source.source}:available`} rows={available} headed={false} entries={withPage} opened={opened} onOpen={onOpen} />
      {loaded.length < (page?.total ?? 0) && <Group justify="center">
        <Button variant="light" loading={more.loading} onClick={() => void loadMore()}>{t('spells.loadMore')}</Button>
      </Group>}
      {page !== null && page.total === 0 && <Text size="sm" c="dimmed">{t('spells.empty')}</Text>}
    </Stack>
  </>
}
