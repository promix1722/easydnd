import { useState, type ReactNode } from 'react'

import { slugOf } from '@/domain'
import type { Choice, Entry, Spell } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, Group, Stack, Text, Title } from '@/ui'
import { SpellChoiceRow } from './SpellChoiceRow'
import { SpellFilters } from '@/features/spells/SpellFilters'
import { matchesSpellFilters, EMPTY_SPELL_FILTERS } from '@/features/spells/filterSpells'
import { levelText } from '@/features/spells/spellText'

import type { Choosable } from './options'

/** The draft answer stays above the filtered catalogue; opening a spell only previews it. */
export function SpellChoices({ choice, options, entries, picked, pending, onToggle, confirmation }: {
  choice: Choice
  options: readonly Choosable[]
  entries: ReadonlyMap<string, Entry>
  picked: readonly string[]
  pending: boolean
  onToggle: (key: string) => void
  confirmation: ReactNode
}) {
  const t = useT()
  const [filters, setFilters] = useState(EMPTY_SPELL_FILTERS)
  const [opened, setOpened] = useState<string | null>(null)
  const refs = new Map(choice.from.options?.map((option) => [option.key, option.ref]))
  const rows = options.map((option) => {
    const slug = slugOf(refs.get(option.key) ?? option.key)
    return { option, slug, spell: entries.get(slug) as Spell | undefined }
  })
  const named = (slugs: readonly string[]) => [...new Set(slugs)].flatMap((slug) => {
    const entry = entries.get(slug)
    return entry === undefined ? [] : [entry]
  })
  const schools = named(rows.flatMap(({ spell }) => spell?.school === undefined ? [] : [spell.school]))
  const classes = named(rows.flatMap(({ spell }) => spell?.classes ?? []))
  const selected = picked.flatMap((key) => {
    const row = rows.find(({ option }) => option.key === key)
    return row === undefined ? [] : [row]
  })
  const visible = rows.filter(({ option, spell }) => !picked.includes(option.key) && (spell === undefined
    ? option.label.toLocaleLowerCase().includes(filters.query.trim().toLocaleLowerCase())
    : matchesSpellFilters(spell, filters)))
    .sort((a, b) => (a.spell?.level ?? -1) - (b.spell?.level ?? -1) || a.option.label.localeCompare(b.option.label))
  const selectedLevels = [...new Set(selected.map(({ spell }) => spell?.level ?? -1))].sort((a, b) => a - b)
  const filtered = Object.entries(filters).some(([key, value]) => value !== EMPTY_SPELL_FILTERS[key as keyof typeof filters])

  function renderRow({ option, slug, spell }: typeof rows[number], isSelected: boolean) {
    return <SpellChoiceRow key={option.key} option={option} slug={slug} spell={spell} entries={entries}
      isSelected={isSelected} pending={pending} disabled={!isSelected && (option.disabled || picked.length >= choice.choose)}
      opened={opened === option.key} onOpen={(open) => setOpened(open ? option.key : null)} onToggle={() => onToggle(option.key)} />
  }

  return (
    <Stack gap="md">
      <Stack component="section" aria-label={t('prompt.selectedSpells')} gap="xs">
        <Text fw={600}>{t('prompt.selectedSpells')}</Text>
        <Text size="sm" aria-live="polite">{t('prompt.spellsSelected', { count: picked.length, total: choice.choose })}</Text>
        {selectedLevels.map((level) => (
          <Stack component="section" key={level} aria-label={level < 0 ? t('prompt.selectedSpells') : levelText(t, level)} gap="xs">
            {level >= 0 && <Title order={4}>{levelText(t, level)}</Title>}
            {selected.filter(({ spell }) => (spell?.level ?? -1) === level).map((row) => renderRow(row, true))}
          </Stack>
        ))}
      </Stack>
      {confirmation}
      <SpellFilters value={filters} onChange={setFilters} schools={schools} classes={classes} />
      <Group gap="sm" justify="space-between">
        <Text size="sm" c="dimmed" aria-live="polite">{t('spells.count', { count: visible.length })}</Text>
        {filtered && <Button variant="subtle" onClick={() => setFilters(EMPTY_SPELL_FILTERS)}>{t('prompt.resetSpellFilters')}</Button>}
      </Group>
      <Stack component="section" aria-label={t('prompt.availableSpells')} gap="xs">
        {visible.map((row) => renderRow(row, false))}
        {visible.length === 0 && <Text size="sm" c="dimmed">{t(options.length === 0 ? 'prompt.nothingOffered' : 'spells.empty')}</Text>}
      </Stack>
    </Stack>
  )
}
