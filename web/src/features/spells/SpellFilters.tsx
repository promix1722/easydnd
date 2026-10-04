import type { Entry, SourceOptions } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Checkbox, Group, MultiSelect, Select, Stack, TextInput, sourceAbbreviation } from '@/ui'

import { castingTimeText, levelText } from './spellText'

import type { SpellFilterValues } from './filterSpells'

const LEVELS = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
const CASTING_TIMES = ['action', 'bonus-action', 'reaction', 'over-time']

/** Shared controls; browsing stores filters in the URL, build choices keep them local. */
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
          <MultiSelect
            w={{ base: '100%', sm: 240 }} miw={0} maw="100%"
            styles={{ inputField: { minWidth: 0 }, option: { whiteSpace: 'nowrap' } }}
            aria-label={t('spells.filter.pack')} placeholder={value.packIds?.length ? undefined : t('spells.filter.allPacks')}
            data={sourceOptions.packs.map((p) => ({ value: p.id, label: p.title }))}
            value={value.packIds ?? []} searchable clearable
            onChange={(packIds) => onChange({ ...value, packIds, sources: (value.sources ?? []).filter((id) => !packIds.length || sourceOptions.sources.some((s) => s.id === id && packIds.includes(s.packId))) })}
          />
          <MultiSelect
            w={{ base: '100%', sm: 240 }} miw={0} maw="100%"
            styles={{ inputField: { minWidth: 0 }, option: { whiteSpace: 'nowrap' } }}
            aria-label={t('spells.filter.source')} placeholder={value.sources?.length ? undefined : t('spells.filter.allSources')}
            data={sourceOptions.sources.filter((s) => !value.packIds?.length || value.packIds.includes(s.packId)).map((s) => ({ value: s.id, label: `${sourceAbbreviation(s.id)} / ${sourceOptions.packs.find((p) => p.id === s.packId)?.title ?? s.packId}` }))}
            value={value.sources ?? []} searchable clearable onChange={(sources) => onChange({ ...value, sources })}
          />
          {onVersionChange && sourceOptions.packs.filter((p) => p.versions.length > 1 && (!value.packIds?.length || value.packIds.includes(p.id))).map((p) => <Select key={p.id}
            maw="100%"
            label={`${p.title} · ${t('packs.version')}`} data={p.versions} value={p.version}
            onChange={(version) => { if (version) onVersionChange(p.id, version) }}
          />)}
        </>}
        <Select
          aria-label={t('spells.filter.level')}
          placeholder={t('spells.filter.allLevels')}
          data={LEVELS.map((level) => ({ value: String(level), label: levelText(t, level) }))}
          value={value.level}
          onChange={(level) => onChange({ ...value, level })}
          clearable
        />
        <Select
          aria-label={t('spells.filter.school')}
          placeholder={t('spells.filter.allSchools')}
          data={schools.map((entry) => ({ value: entry.slug, label: entry.name }))}
          value={value.school}
          onChange={(school) => onChange({ ...value, school })}
          clearable
        />
        <Select
          aria-label={t('spells.filter.class')}
          placeholder={t('spells.filter.allClasses')}
          data={classes.map((entry) => ({ value: entry.slug, label: entry.name }))}
          value={value.casterClass}
          onChange={(casterClass) => onChange({ ...value, casterClass })}
          clearable
        />
        <Select
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
