import { useCatalogScope } from '@/lib/api/catalogScope'
import { useState } from 'react'

import { slugOf } from '@/domain'
import { getCollection, getEntries } from '@/lib/api'
import type { Change, Entry, Prompt, Spell, SpellRule } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Button, Group, Stack, Text, Title } from '@/ui'
import { SpellFilters } from '@/features/spells/SpellFilters'
import { EMPTY_SPELL_FILTERS, matchesSpellFilters } from '@/features/spells/filterSpells'
import { levelText } from '@/features/spells/spellText'

import type { Block } from './blocks'
import type { SettledRow } from './settled'
import { choosableOptions } from './options'
import { SpellChoiceRow } from './SpellChoiceRow'
import { spellChoiceName } from './promptNames'
import { allocateLimitedSpells } from './spellAllocation'
import { SpellRulesSummary } from './SpellRulesSummary'

export interface SpellSubmission { prompt: Prompt; picks: string[]; replaces?: SettledRow; changes?: Change[] }

/** One class-filtered pool; server prompt identities are only used when saving. */
export function SpellStagePanel({ blocks, active, names, loadSavedPrompt, onAnswers, pending, revision, onNext, cantripsOnly = false, rules = [] }: {
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
}) {
  const t = useT()
  const scope = useCatalogScope()
  const limit = 20
  const [shownCount, setShownCount] = useState(limit)
  const [filters, setFilters] = useState(EMPTY_SPELL_FILTERS)
  const [availableOnly, setAvailableOnly] = useState(true)
  const [opened, setOpened] = useState<string | null>(null)
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
    const [selectedEntries, summaries, schools, classes] = await Promise.all([
      getEntries<Spell>('spells', [...new Set(saved)], scope).then((items) => new Map<string, Entry>(items.map((item) => [item.slug, item]))),
      getCollection<Spell>('spells', scope), getCollection('magic-schools', scope), getCollection('classes', scope),
    ])
    return { questions, editQuestions, entries: new Map<string, Entry>([...[...summaries, ...schools, ...classes].map((item) => [item.slug, item] as const), ...(selectedEntries ?? [])]) }
  })
  const savedExtra = rules.filter((rule) => rule.purpose === 'custom-limit' && (cantripsOnly ? rule.maxLevel === 0 : rule.maxLevel > 0)).reduce((sum, rule) => sum + rule.count, 0)
  const extra = savedExtra + current.extra
  const browsing = active && !availableOnly
  const loadLibrary = browsing || current.custom.length > 0 || extra > 0
  const library = useResource(`spell-library:${scope}:${revision}:${loadLibrary}:${cantripsOnly}`, async () => {
    if (!loadLibrary) return { spells: [] as Spell[], entries: [] as Entry[] }
    const [summaries, schools, classes] = await Promise.all([
      getCollection<Spell>('spells', scope), getCollection('magic-schools', scope), getCollection('classes', scope),
    ])
    const spells = summaries.filter((spell) => cantripsOnly ? spell.level === 0 : spell.level > 0)
    return { spells, entries: [...schools, ...classes, ...spells] }
  })
  const entries = new Map<string, Entry>([
    ...(library.data?.entries ?? []).map((entry) => [entry.slug, entry] as const),
    ...(catalogue.data?.entries ?? []),
  ])
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
  const spellLevels = new Map([...entries].flatMap(([slug, entry]) => 'level' in entry ? [[slug, (entry as Spell).level] as const] : []))
  const allocate = (picks: string[]) => allocateLimitedSpells(allowances, picks, levelLimits, spellLevels)
  const allocation = allocate(picked)
  const selectedSlugs = new Set(selected.map((row) => row.slug))
  const eligibleKeys = new Set(allowances.flatMap(({ eligible }) => eligible))
  const candidates = [...pool.values()]
  if (browsing || extra > 0) {
    const offeredSlugs = new Set(candidates.map(({ slug }) => slug))
    for (const spell of library.data?.spells ?? []) {
      if (!offeredSlugs.has(spell.slug)) candidates.push({ slug: spell.slug, spell, option: { key: spell.slug, label: spell.name, disabled: true } })
    }
  }
  const fitsExtra = (spell: Spell | undefined) => spell !== undefined && rules.some((rule) => rule.purpose !== 'custom-limit' && !rule.automatic?.length
    && spell.level >= rule.minLevel && spell.level <= rule.maxLevel && (rule.listClasses ?? []).some((className) => spell.classes?.includes(className)))
  const available = candidates.filter(({ option, spell }) => !availableOnly || eligibleKeys.has(option.key) || extra > 0 && fitsExtra(spell))
    .filter(({ slug, option }) => !selectedSlugs.has(slug) || allowances.some(({ prompt, eligible }) => prompt.purpose === 'prepared' && eligible.includes(option.key) && !picked.includes(option.key))).filter(({ option, spell }) => spell === undefined
    ? option.label.toLocaleLowerCase().includes(filters.query.trim().toLocaleLowerCase()) : matchesSpellFilters(spell, filters))
    .sort((a, b) => (a.spell?.level ?? -1) - (b.spell?.level ?? -1) || a.option.label.localeCompare(b.option.label))
  const pageKey = JSON.stringify([filters, availableOnly, cantripsOnly, revision])
  const [previousPageKey, setPreviousPageKey] = useState(pageKey)
  if (previousPageKey !== pageKey) { setPreviousPageKey(pageKey); setShownCount(limit) }
  const visibleCount = previousPageKey === pageKey ? shownCount : limit
  const pageRows = available.slice(0, visibleCount)
  const detailSlugs = [...new Set([...pageRows.map((row) => row.slug), ...selected.map((row) => row.slug)])]
  const details = useResource(`spell-page:${scope}:${revision}:${JSON.stringify(detailSlugs)}`, () => getEntries<Spell>('spells', detailSlugs, scope))
  for (const spell of details.data ?? []) entries.set(spell.slug, spell)
  const levels = [...new Set(selected.map(({ spell }) => spell?.level ?? -1))].sort((a, b) => a - b)
  const named = (slugs: readonly string[]) => [...new Set(slugs)].flatMap((slug) => {
    const entry = entries.get(slug)
    return entry === undefined ? [] : [entry]
  })
  const eligibleSpells = candidates.flatMap(({ spell }) => spell === undefined ? [] : [spell])
  const schools = named(eligibleSpells.flatMap((spell) => spell.school === undefined ? [] : [spell.school]))
  const classes = named(eligibleSpells.flatMap((spell) => spell.classes ?? []))
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
  const filtered = !availableOnly || Object.entries(filters).some(([key, value]) => value !== EMPTY_SPELL_FILTERS[key as keyof typeof filters])

  function remove(block: Block | undefined, pick: string) {
    if (block === undefined) { setDraft({ ...draft, added: current.added.filter((key) => key !== pick), custom: current.custom.filter((key) => key !== pick) }); return }
    const remaining = (current.edits[block.key] ?? savedPicks(block)).filter((key) => key !== pick)
    setDraft({ ...draft, edits: { ...current.edits, [block.key]: remaining } })
  }
  function add(pick: string) {
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
          opened={opened === row.key} onOpen={(open) => setOpened(open ? row.key : null)} onToggle={() => remove(row.block, row.pick)}
        />)}
      </Stack>)}
      {selected.length === 0 && <Text size="sm" c="dimmed">{t('sheet.none')}</Text>}
    </Stack>
    {active && <Group>
      <Button disabled={!ready || busy} loading={pending} onClick={() => finish()}>{t('stagePanel.next')}</Button>
    </Group>}
    {catalogue.loading && <Text size="sm">{t('page.loadingEllipsis')}</Text>}
    {catalogue.error !== null && <Group><Text c="red">{catalogue.error}</Text><Button onClick={catalogue.reload}>{t('page.retry')}</Button></Group>}
    {active && <SpellFilters value={filters} onChange={setFilters} schools={schools} classes={classes}
      availableOnly={availableOnly} onAvailableOnlyChange={setAvailableOnly} />}
    {browsing && library.loading && <Text size="sm">{t('page.loadingEllipsis')}</Text>}
    {browsing && library.error !== null && <Group><Text c="red">{library.error}</Text><Button onClick={library.reload}>{t('page.retry')}</Button></Group>}
    <Group justify="space-between">
      <Text size="sm" c="dimmed" aria-live="polite">{t('spells.count', { count: available.length })}</Text>
      {filtered && <Button variant="subtle" onClick={() => { setFilters(EMPTY_SPELL_FILTERS); setAvailableOnly(true) }}>{t('prompt.resetSpellFilters')}</Button>}
    </Group>
    <Stack component="section" aria-label={t('prompt.availableSpells')} gap="xs">
      {pageRows.map((row) => <SpellChoiceRow key={row.option.key} {...row} spell={entries.get(row.slug) as Spell | undefined} entries={entries} isSelected={false} pending={busy}
        disabled={availableOnly ? customCount >= extra && allocate([...picked, row.option.key]) === null : customQuestion === undefined && !selectedBlocks.some((block) => block.kind === 'settled' && block.row.event.purpose === 'custom') && allocate([...picked, row.option.key]) === null}
        opened={opened === row.option.key} onOpen={(open) => setOpened(open ? row.option.key : null)} onToggle={() => add(row.option.key)}
      />)}
      {pageRows.length < available.length && <Group justify="center">
        <Button variant="light" loading={details.loading} onClick={() => setShownCount(visibleCount + limit)}>{t('spells.loadMore')}</Button>
      </Group>}
      {available.length === 0 && <Text size="sm" c="dimmed">{t('spells.empty')}</Text>}
    </Stack>
  </Stack>
}
