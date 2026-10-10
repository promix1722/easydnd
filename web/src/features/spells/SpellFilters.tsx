import type { Entry, SourceOptions } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Checkbox, Group, SummaryMultiSelect, Select, Stack, TextInput } from '@/ui'

import { castingTimeText, levelText } from './spellText'

import type { SpellFilterValues } from './filterSpells'

const LEVELS = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
const CASTING_TIMES = ['action', 'bonus-action', 'reaction', 'over-time']

/** Shared controls; browsing stores filters in the URL, build choices keep them local. */
/**
 * One width for every filter, and it does not move: a control sized by its
 * placeholder is a different width in every language and changes again when a
 * value longer or shorter than the placeholder is picked, so the row re-wraps
 * under the cursor. Full width on a phone, where they stack.
 */
const FILTER_WIDTH = { w: { base: '100%', sm: 240 }, miw: 0, maw: '100%' } as const

export function SpellFilters({ value, onChange, schools, classes, availableOnly, onAvailableOnlyChange, sourceOptions, onVersionChange }: {
  sourceOptions?: SourceOptions
  onVersionChange?: (pack: string, version: string) => void
  value: SpellFilterValues
  onChange: (value: SpellFilterValues) => void
  schools: readonly Entry[]
  classes: readonly Entry[]
  availableOnly?: boolean
  onAvailableOnlyChange?: (value: boolean) => void
}) {
  const t = useT()
  return (
    <Stack gap="sm">
      <TextInput
        aria-label={t('spells.search')}
        placeholder={t('spells.search')}
        value={value.query}
        onChange={(event) => onChange({ ...value, query: event.currentTarget.value })}
      />
      <Group gap="sm">
        {sourceOptions && <>
          <SummaryMultiSelect
            {...FILTER_WIDTH}
            aria-label={t('spells.filter.pack')} placeholder={t('spells.filter.allPacks')}
            data={sourceOptions.packs.flatMap((p) => (onVersionChange ? p.versions : [p.version])
              .map((version) => ({ value: `${p.id}@${version}`, label: `${p.title} v${version}` })))}
            value={(value.packIds ?? []).flatMap((id) => {
              const pack = sourceOptions.packs.find((p) => p.id === id)
              return pack ? [`${id}@${pack.version}`] : []
            })} searchable clearable
            onChange={(releases) => {
              const selected = new Map(releases.map((release) => release.split('@') as [string, string]))
              const packIds = [...selected.keys()]
              onChange({ ...value, packIds, sources: (value.sources ?? []).filter((id) => !packIds.length || sourceOptions.sources.some((s) => s.id === id && packIds.includes(s.packId))) })
              for (const [id, version] of selected) {
                if (sourceOptions.packs.find((p) => p.id === id)?.version !== version) onVersionChange?.(id, version)
              }
            }}
          />
          <SummaryMultiSelect
            {...FILTER_WIDTH}
            aria-label={t('spells.filter.source')} placeholder={t('spells.filter.allSources')}
            data={sourceOptions.sources.filter((s) => !value.packIds?.length || value.packIds.includes(s.packId)).map((s) => {
              const pack = sourceOptions.packs.find((p) => p.id === s.packId)
              // Pack first, then the book inside it: the order the two filters stand in.
              return { value: s.id, label: `${pack ? `${pack.title} v${pack.version}` : s.packId} / ${s.name}` }
            })}
            value={value.sources ?? []} searchable clearable onChange={(sources) => onChange({ ...value, sources })}
          />

        </>}
        <Select
          {...FILTER_WIDTH}
          aria-label={t('spells.filter.level')}
          placeholder={t('spells.filter.allLevels')}
          data={LEVELS.map((level) => ({ value: String(level), label: levelText(t, level) }))}
          value={value.level}
          onChange={(level) => onChange({ ...value, level })}
          clearable
        />
        <Select
          {...FILTER_WIDTH}
          aria-label={t('spells.filter.school')}
          placeholder={t('spells.filter.allSchools')}
          data={schools.map((entry) => ({ value: entry.slug, label: entry.name }))}
          value={value.school}
          onChange={(school) => onChange({ ...value, school })}
          clearable
        />
        <Select
          {...FILTER_WIDTH}
          aria-label={t('spells.filter.class')}
          placeholder={t('spells.filter.allClasses')}
          data={classes.map((entry) => ({ value: entry.slug, label: entry.name }))}
          value={value.casterClass}
          onChange={(casterClass) => onChange({ ...value, casterClass })}
          clearable
        />
        <Select
          {...FILTER_WIDTH}
          aria-label={t('spells.filter.castingTime')}
          placeholder={t('spells.filter.anyTime')}
          data={CASTING_TIMES.map((kind) => ({
            value: kind,
            label: kind === 'over-time' ? t('spells.filter.overTime') : castingTimeText(t, { kind }),
          }))}
          value={value.time}
          onChange={(time) => onChange({ ...value, time })}
          clearable
        />
      </Group>
      <Group gap="md">
        {onAvailableOnlyChange !== undefined && <Checkbox
          label={t('spells.filter.availableForCharacter')}
          checked={availableOnly ?? true}
          onChange={(event) => onAvailableOnlyChange(event.currentTarget.checked)}
        />}
        <Checkbox
          label={t('spells.filter.concentration')}
          checked={value.concentration}
          onChange={(event) => onChange({ ...value, concentration: event.currentTarget.checked })}
        />
        <Checkbox
          label={t('spells.filter.ritual')}
          checked={value.ritual}
          onChange={(event) => onChange({ ...value, ritual: event.currentTarget.checked })}
        />
        <Checkbox
          label={t('spells.filter.noMaterial')}
          checked={value.noMaterial}
          onChange={(event) => onChange({ ...value, noMaterial: event.currentTarget.checked })}
        />
      </Group>
    </Stack>
  )
}
