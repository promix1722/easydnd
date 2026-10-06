import { useCatalogScope } from '@/lib/api/catalogScope'
import { useState } from 'react'
import type { ReactNode } from 'react'

import { slugOf } from '@/domain'
import { getEntries, getSpellFilterOptions, searchSpellOffer } from '@/lib/api'
import type { Change, Entry, Prompt, Spell, SpellLevels, SpellOfferSearch, SpellPage, SpellRule } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Button, Group, Stack, Text, Title } from '@/ui'
import { SpellFilters } from '@/features/spells/SpellFilters'
import { EMPTY_SPELL_FILTERS, hasSpellFilters, matchesSpellFilters, spellFilterSearch } from '@/features/spells/filterSpells'
import { levelText } from '@/features/spells/spellText'

import type { Block } from './blocks'
import type { SettledRow } from './settled'
import { choosableOptions } from './options'
import { SpellChoiceRow } from './SpellChoiceRow'
import { spellChoiceName } from './promptNames'
import { allocateLimitedSpells } from './spellAllocation'
import { SpellRulesSummary } from './SpellRulesSummary'

export interface SpellSubmission { prompt: Prompt; picks: string[]; replaces?: SettledRow; changes?: Change[] }

/**
 * One class-filtered pool; server prompt identities are only used when saving.
 *
 * The list of what can be picked is searched, sorted and paged by the server
 * (`searchSpellOffer`). This panel tells it the offer -- the options the open
 * prompts list, plus whatever an extra allowance lets in -- and is sent one
 * page of spells at a time. It never holds the spell list: that is every spell
 * in the rules with its artwork, and it used to be downloaded to do here what
 * the server now does.
 */
export function SpellStagePanel({ blocks, active, names, loadSavedPrompt, onAnswers, pending, revision, onNext, cantripsOnly = false, rules = [], children }: {
  blocks: readonly Block[]
  active: boolean
  names: ReadonlyMap<string, string>
  loadSavedPrompt: (row: SettledRow) => Promise<Prompt>
  onAnswers: (answers: SpellSubmission[]) => void
  pending: boolean
  revision: number
  onNext?: () => void
  cantripsOnly?: boolean
  rules?: readonly SpellRule[]
  /** Custom entries already on this tab, under what was selected. */
  children?: ReactNode
}) {
  const t = useT()
  const scope = useCatalogScope()
  const limit = 20
  const [filters, setFilters] = useState(EMPTY_SPELL_FILTERS)
  const [availableOnly, setAvailableOnly] = useState(true)
  const savedPicks = (block: Block) => block.kind === 'settled' ? block.row.event.choices?.[0]?.picks ?? [] : []
  const [draft, setDraft] = useState<{ revision: number; active: boolean; added: string[]; custom: string[]; extra: number; edits: Record<string, string[]> }>({ revision, active, added: [], custom: [], extra: 0, edits: {} })
  if (draft.revision !== revision || draft.active !== active) {
    setDraft({ revision, active, added: [], custom: [], extra: 0, edits: {} })
  }
  const current: { added: string[]; custom: string[]; extra: number; edits: Record<string, string[]> } = draft.revision === revision && draft.active === active ? draft : { added: [], custom: [], extra: 0, edits: {} }
  const edited = blocks.flatMap((block) => block.kind === 'settled' && current.edits[block.key] !== undefined ? [block] : [])
  const openQuestions = !active ? [] : blocks.flatMap((block) => block.kind === 'open' && block.prompt.purpose !== 'forget' && block.prompt.purpose !== 'replace' ? [block.prompt] : [])
  const forgotten = blocks.flatMap((block) => block.kind === 'settled' && block.row.event.purpose === 'forget'
    ? savedPicks(block).map((pick) => ({ pick, source: block.row.event.ref ?? block.row.event.choiceSource, level: block.row.event.level ?? 0 })) : [])
  const selectedBlocks = blocks.filter((block) => block.kind === 'settled' && block.row.event.purpose !== 'forget')
  const saved = [...selectedBlocks.flatMap((block) => savedPicks(block).map(slugOf)), ...current.custom]
  const catalogue = useResource(`spell-stage:${scope}:${JSON.stringify([saved, openQuestions, edited.map((block) => block.key), revision])}`, async () => {
    const editQuestions = await Promise.all(edited.map(async (block) => ({ key: block.key, prompt: await loadSavedPrompt(block.row), row: block.row })))
    const questions = [...editQuestions.map((item) => item.prompt), ...openQuestions]
      .sort((a, b) => Number(a.optional) - Number(b.optional) || (a.level ?? 0) - (b.level ?? 0))
    const [selectedEntries, options] = await Promise.all([
      getEntries<Spell>('spells', [...new Set(saved)], scope),
      getSpellFilterOptions(scope),
    ])
    return { questions, editQuestions, options, entries: [...options.schools, ...options.classes, ...selectedEntries] }
  })
  const savedExtra = rules.filter((rule) => rule.purpose === 'custom-limit' && (cantripsOnly ? rule.maxLevel === 0 : rule.maxLevel > 0)).reduce((sum, rule) => sum + rule.count, 0)
  const extra = savedExtra + current.extra
  const browsing = active && !availableOnly
  // A pick is drawn from the row it was picked off, which may be a page the
  // list has since left.
  const [seen, setSeen] = useState<ReadonlyMap<string, Spell>>(new Map())
  const [opened, setOpened] = useState<string | null>(null)
  const [openedSlug, setOpenedSlug] = useState<string | null>(null)
  // The prose of the one spell that is open; a row needs nothing but its summary.
  const details = useResource(`spell-open:${scope}:${openedSlug ?? ''}`, () => openedSlug === null ? Promise.resolve([]) : getEntries<Spell>('spells', [openedSlug], scope))
  const [more, setMore] = useState<{ key: string; spells: Spell[]; loading: boolean }>({ key: '', spells: [], loading: false })
  const [lastPage, setLastPage] = useState<SpellPage | null>(null)
  const entries = new Map<string, Entry>([...(catalogue.data?.entries ?? []), ...seen.values()].map((entry) => [entry.slug, entry] as const))
  const questions = catalogue.data?.questions ?? []
  const editQuestions = catalogue.data?.editQuestions ?? []
  const busy = pending || catalogue.loading || catalogue.error !== null
  const retained = (prompt: Prompt) => {
    const edit = editQuestions.find((item) => item.prompt.choice.prompt === prompt.choice.prompt)
    return edit === undefined ? [] : current.edits[edit.key] ?? []
  }
  const removed = edited.flatMap((block) => savedPicks(block).filter((pick) => !current.edits[block.key]?.includes(pick)))
  const editablePrompt = (prompt: Prompt): Prompt => ({ ...prompt, held: (prompt.held ?? []).filter((key) => !removed.includes(key)) })
  const standardQuestions = questions.filter((prompt) => prompt.purpose !== 'custom')
  const customQuestion = questions.find((prompt) => prompt.purpose === 'custom')
  const allowances = standardQuestions.map((prompt) => ({
    prompt: { ...prompt, choice: { ...prompt.choice, choose: Math.max(0, prompt.choice.choose - retained(prompt).length) } },
    eligible: choosableOptions(t, editablePrompt(prompt), entries).filter((option) => !option.disabled).map((option) => option.key),
  }))
  const pool = new Map(standardQuestions.flatMap((prompt) => {
    const refs = new Map(prompt.choice.from.options?.map((option) => [option.key, option.ref]))
    return choosableOptions(t, editablePrompt(prompt), entries).map((option) => {
      const slug = slugOf(refs.get(option.key) ?? option.key)
      return [option.key, { option, slug, spell: entries.get(slug) as Spell | undefined }] as const
    })
  }))
  const picked = current.added
  const selected = [
    ...selectedBlocks.flatMap((block) => (current.edits[block.key] ?? savedPicks(block)).filter((pick) => !forgotten.some((item) => item.pick === pick && block.kind === 'settled' && item.source === (block.row.event.ref ?? block.row.event.choiceSource) && item.level >= (block.row.event.level ?? 0) && block.row.event.purpose !== 'custom')).map((pick) => {
      const ref = block.kind === 'settled' ? block.row.event.selections?.find((option) => option.key === pick)?.ref : undefined
      return { key: `${block.key}:${pick}`, block, pick, slug: slugOf(ref ?? pick) }
    })),
    ...[...current.added, ...current.custom].map((pick) => ({ key: `draft:${pick}`, block: undefined, pick, slug: pool.get(pick)?.slug ?? pick })),
  ].map((row) => ({ ...row, spell: entries.get(row.slug) as Spell | undefined,
    custom: row.block?.kind === 'settled' ? row.block.row.event.purpose === 'custom' : current.custom.includes(row.pick),
    label: entries.get(row.slug)?.name ?? names.get(`spell:${row.slug}`) ?? names.get(row.slug) ?? row.slug }))
  const levelLimits = rules.flatMap((rule) => rule.maxLevelCount === undefined ? [] : [{
    source: rule.source, purpose: rule.purpose, level: rule.maxLevel,
    remaining: rule.maxLevelCount - selected.filter((row) => row.block?.kind === 'settled' && !row.custom && row.spell?.level === rule.maxLevel && row.block.row.event.choiceSource === rule.source && row.block.row.event.purpose === rule.purpose).length,
  }])
  const selectedSlugs = new Set(selected.map((row) => row.slug))
  const eligibleKeys = new Set(allowances.flatMap(({ eligible }) => eligible))
  const offered = new Map([...pool.values()].map((entry) => [entry.slug, entry] as const))
  // A spell already in the spellbook can still be prepared, so it stays listed
  // while a prepared allowance has room for it.
  const repickable = (slug: string, draft: readonly string[]) => {
    const key = offered.get(slug)?.option.key
    return key !== undefined && allowances.some(({ prompt, eligible }) => prompt.purpose === 'prepared' && eligible.includes(key) && !draft.includes(key))
  }
  const hidden = (slug: string) => selectedSlugs.has(slug) && !repickable(slug, picked)

  // The offer: what the open prompts list, plus what fits an extra allowance.
  // Browsing widens it to every spell of this stage's levels, unpickable ones
  // included.
  const stageLevels: SpellLevels = cantripsOnly ? { minLevel: 0, maxLevel: 0 } : { minLevel: 1, maxLevel: 9 }
  const fitting: SpellLevels[] = browsing ? [stageLevels] : extra <= 0 ? [] : rules.flatMap((rule) => {
    const classes = rule.listClasses ?? []
    const minLevel = Math.max(rule.minLevel, stageLevels.minLevel), maxLevel = Math.min(rule.maxLevel, stageLevels.maxLevel)
    return rule.purpose === 'custom-limit' || rule.automatic?.length || classes.length === 0 || minLevel > maxLevel ? [] : [{ minLevel, maxLevel, classes }]
  })
  const only = { slugs: [...new Set([...pool.values()].filter(({ option }) => browsing || eligibleKeys.has(option.key)).map(({ slug }) => slug))], fitting }
  // Saved picks are left out by the server. A draft pick is hidden here
  // instead, so that adding a spell does not ask for the page again.
  const exclude = [...new Set(selected.filter((row) => row.block !== undefined && !repickable(row.slug, [])).map((row) => row.slug))]
  const search: SpellOfferSearch = { ...spellFilterSearch(filters), only, exclude, limit }
  const searchKey = JSON.stringify([scope, revision, search])
  const nothingOffered = catalogue.data === null || only.slugs.length === 0 && fitting.length === 0
  const found = useResource(`spell-offer:${nothingOffered}:${searchKey}`, (signal) => nothingOffered
    ? Promise.resolve<SpellPage>({ spells: [], total: 0 }) : searchSpellOffer(search, signal, scope))
  // The last page that arrived stays up while the next search is in flight,
  // so that typing in the filter does not empty the list on every letter.
  if (found.data !== null && found.data !== lastPage) setLastPage(found.data)
  const page = found.data ?? lastPage
  const appended = more.key === searchKey ? more.spells : []
  const loaded = [...(page?.spells ?? []), ...appended]
  for (const spell of loaded) if (!entries.has(spell.slug)) entries.set(spell.slug, spell)
  for (const spell of details.data ?? []) entries.set(spell.slug, spell)

  const spellLevels = new Map([...entries].flatMap(([slug, entry]) => 'level' in entry ? [[slug, (entry as Spell).level] as const] : []))
  const allocate = (picks: string[]) => allocateLimitedSpells(allowances, picks, levelLimits, spellLevels)
  const allocation = allocate(picked)
  const pageRows = loaded.filter((spell) => !hidden(spell.slug)).map((spell) => ({ slug: spell.slug, spell,
    option: { ...(offered.get(spell.slug)?.option ?? { key: spell.slug, disabled: true }), label: spell.name } }))
  // The server counted every match; the draft picks hidden above are among them.
  const inOffer = (spell: Spell) => only.slugs.includes(spell.slug) || fitting.some((fit) => spell.level >= fit.minLevel && spell.level <= fit.maxLevel
    && (!fit.classes?.length || fit.classes.some((name) => spell.classes?.includes(name))))
  const hiddenMatches = [...selectedSlugs].filter((slug) => {
    const spell = entries.get(slug)
    return spell !== undefined && 'level' in spell && !exclude.includes(slug) && hidden(slug) && inOffer(spell as Spell) && matchesSpellFilters(spell as Spell, filters)
  }).length
  const availableCount = Math.max(0, (page?.total ?? 0) - hiddenMatches)
  const levels = [...new Set(selected.map(({ spell }) => spell?.level ?? -1))].sort((a, b) => a - b)
  async function loadMore() {
    setMore({ key: searchKey, spells: appended, loading: true })
    const settle = (spells: Spell[]) => setMore((now) => now.key === searchKey ? { key: searchKey, spells: [...now.spells, ...spells], loading: false } : now)
    try { settle((await searchSpellOffer({ ...search, offset: loaded.length }, undefined, scope)).spells) } catch { settle([]) }
  }
  const open = (key: string, slug: string, isOpen: boolean) => { setOpened(isOpen ? key : null); setOpenedSlug(isOpen ? slug : null) }
  const complete = allocation !== null && standardQuestions.every((prompt) => {
    const count = retained(prompt).length + (allocation.get(prompt.choice.prompt)?.length ?? 0)
    return prompt.optional && count === 0 || (prompt.upTo === true ? count > 0 && count <= prompt.choice.choose : count === prompt.choice.choose)
  })
  const customCount = selected.filter((row) => row.custom).length
  const changed = edited.length > 0 || picked.length > 0 || current.custom.length > 0 || current.extra !== 0
  const normalChanged = picked.length > 0 || edited.some((block) => block.row.event.purpose !== 'custom')
  const ready = levelLimits.every((limit) => limit.remaining >= 0) && (!normalChanged || standardQuestions.length > 0 && complete)
  const fixedCount = selected.length - picked.length - customCount
  const ruleTotal = rules.filter((rule) => rule.purpose !== 'custom-limit' && rule.purpose !== 'replace' && rule.purpose !== 'forget' && !rule.automatic?.length && (cantripsOnly ? rule.maxLevel === 0 : rule.maxLevel > 0)).reduce((sum, rule) => sum + rule.count, 0)
  const total = (ruleTotal || fixedCount + allowances.reduce((sum, { prompt }) => sum + prompt.choice.choose, 0)) + extra
  const filtered = !availableOnly || hasSpellFilters(filters)

  function remove(block: Block | undefined, pick: string) {
    if (block === undefined) { setDraft({ ...draft, added: current.added.filter((key) => key !== pick), custom: current.custom.filter((key) => key !== pick) }); return }
    const remaining = (current.edits[block.key] ?? savedPicks(block)).filter((key) => key !== pick)
    setDraft({ ...draft, edits: { ...current.edits, [block.key]: remaining } })
  }
  function add(pick: string, spell: Spell | undefined) {
    if (spell !== undefined) setSeen(new Map(seen).set(spell.slug, spell))
    if (allocate([...picked, pick]) !== null) {
      setDraft({ ...draft, added: [...picked, pick] }); return
    }
    if (availableOnly && customCount >= extra) return
    const savedCustom = selectedBlocks.find((block) => block.kind === 'settled' && block.row.event.purpose === 'custom')
    setDraft({ ...draft, custom: [...current.custom, pick], edits: savedCustom === undefined ? current.edits
      : { ...current.edits, [savedCustom.key]: current.edits[savedCustom.key] ?? savedPicks(savedCustom) } })
  }
  function finish() {
    if (!ready || busy) return
    if (!changed) { onNext?.(); return }
    if (allocation === null) return
    const submissions: SpellSubmission[] = questions.flatMap((prompt) => {
      const picks = [...retained(prompt), ...(prompt.purpose === 'custom' ? current.custom : allocation.get(prompt.choice.prompt) ?? [])]
      const edit = editQuestions.find((item) => item.prompt.choice.prompt === prompt.choice.prompt)
      if (edit === undefined) return picks.length === 0 ? [] : [{ prompt, picks }]
      return [{ prompt, picks, replaces: edit.row }]
    })
    if (current.extra !== 0) submissions.push({
      prompt: { choice: { prompt: 'custom/limit', kind: 'spell', choose: 0, from: { kind: 'explicit', options: [] } }, event: { type: 'change' }, group: 'class', optional: true, heldOnly: false }, picks: [],
      changes: [{ path: `spellLimits.${cantripsOnly ? 'cantrip' : 'known'}`, op: 'increment', value: { kind: 'int', int: current.extra } }],
    })
    onAnswers(submissions)
  }

  return <Stack gap="md">
    <SpellRulesSummary rules={rules} cantripsOnly={cantripsOnly} names={names} count={selected.length} total={total} extra={extra}
      pending={pending} onChangeLimit={active ? (delta) => setDraft({ ...draft, extra: Math.min(1000, Math.max(0, extra + delta)) - savedExtra }) : undefined} />
    <Stack component="section" aria-label={t(cantripsOnly ? 'stage.cantrips' : 'prompt.selectedSpells')} gap="xs">
      <Text fw={600}>{t(cantripsOnly ? 'stage.cantrips' : 'prompt.selectedSpells')}</Text>
      {levels.map((level) => <Stack component="section" key={level} aria-label={cantripsOnly || level < 0 ? undefined : levelText(t, level)} gap="xs">
        {!cantripsOnly && level >= 0 && <Title order={4}>{levelText(t, level)}</Title>}
        {selected.filter(({ spell }) => (spell?.level ?? -1) === level).map((row) => <SpellChoiceRow
          key={row.key} option={{ key: row.pick, label: row.label, disabled: false,
            ...(row.block?.kind === 'settled' && row.block.row.event.purpose === 'prepared' ? { reason: spellChoiceName(t, 'prepared', 1) } : {}) }}
          slug={row.slug} spell={entries.get(row.slug) as Spell | undefined} custom={row.custom} entries={entries} isSelected pending={pending}
          disabled={false}
          opened={opened === row.key} onOpen={(isOpen) => open(row.key, row.slug, isOpen)} onToggle={() => remove(row.block, row.pick)}
        />)}
      </Stack>)}
      {selected.length === 0 && <Text size="sm" c="dimmed">{t('sheet.none')}</Text>}
    </Stack>
    {children}
    {active && <Group>
      <Button disabled={!ready || busy} loading={pending} onClick={() => finish()}>{t('stagePanel.next')}</Button>
    </Group>}
    {catalogue.loading && <Text size="sm">{t('page.loadingEllipsis')}</Text>}
    {catalogue.error !== null && <Group><Text c="red">{catalogue.error}</Text><Button onClick={catalogue.reload}>{t('page.retry')}</Button></Group>}
    {active && catalogue.data !== null && <SpellFilters sourceOptions={catalogue.data.options} value={filters} onChange={setFilters} schools={catalogue.data.options.schools} classes={catalogue.data.options.classes}
      availableOnly={availableOnly} onAvailableOnlyChange={setAvailableOnly} />}
    {found.loading && page === null && <Text size="sm">{t('page.loadingEllipsis')}</Text>}
    {found.error !== null && <Group><Text c="red">{found.error}</Text><Button onClick={found.reload}>{t('page.retry')}</Button></Group>}
    <Group justify="space-between">
      <Text size="sm" c="dimmed" aria-live="polite">{t('spells.count', { count: availableCount })}</Text>
      {filtered && <Button variant="subtle" onClick={() => { setFilters(EMPTY_SPELL_FILTERS); setAvailableOnly(true) }}>{t('prompt.resetSpellFilters')}</Button>}
    </Group>
    <Stack component="section" aria-label={t('prompt.availableSpells')} gap="xs">
      {pageRows.map((row) => <SpellChoiceRow key={row.option.key} {...row} spell={entries.get(row.slug) as Spell | undefined} entries={entries} isSelected={false} pending={busy}
        disabled={availableOnly ? customCount >= extra && allocate([...picked, row.option.key]) === null : customQuestion === undefined && !selectedBlocks.some((block) => block.kind === 'settled' && block.row.event.purpose === 'custom') && allocate([...picked, row.option.key]) === null}
        opened={opened === row.option.key} onOpen={(isOpen) => open(row.option.key, row.slug, isOpen)} onToggle={() => add(row.option.key, row.spell)}
      />)}
      {loaded.length < (page?.total ?? 0) && <Group justify="center">
        <Button variant="light" loading={more.loading} onClick={() => void loadMore()}>{t('spells.loadMore')}</Button>
      </Group>}
      {availableCount === 0 && page !== null && <Text size="sm" c="dimmed">{t('spells.empty')}</Text>}
      {/* A spell the rules do not have is the last entry of the list of spells. */}
    </Stack>
  </Stack>
}
