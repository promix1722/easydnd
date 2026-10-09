import type { Entry, Spell } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { joinProse, Markdown, SimpleGrid, SourceTags, Stack, Text, Title } from '@/ui'

import { castingTimeText, componentsAbbrev, durationText, levelText, rangeText } from './spellText'

/**
 * Full spell rules, shared by the compendium and selection previews.
 *
 * On a spell's own page everything about it is inside this one box, level
 * and school included (`standalone`): they used to be the page's subtitle, a dimmed line floating above
 * the panel, apart from the facts they belong with. The book it is from is
 * here too, and only here -- a list of spells carries no source badges.
 *
 * `entries` names what the spell refers to by slug: its classes and its school.
 */
export function SpellDetails({ spell, entries, standalone = false }: {
  spell: Spell
  entries: ReadonlyMap<string, Entry>
  /** Drawn on its own, with no row above it that already says the level and school. */
  standalone?: boolean
}) {
  const t = useT()
  const components = [componentsAbbrev(t, spell.components), spell.components?.text]
    .filter(Boolean).join(' · ')
  const facts = [
    ...(standalone ? [
      { key: 'level', label: t('spells.filter.level'), value: levelText(t, spell.level) },
      { key: 'school', label: t('spells.filter.school'), value: entries.get(spell.school ?? '')?.name ?? '' },
    ] : []),
    { key: 'time', label: t('spell.castingTime'), value: castingTimeText(t, spell.castingTime) },
    { key: 'range', label: t('spell.range'), value: rangeText(t, spell.range) },
    { key: 'duration', label: t('spell.duration'), value: durationText(t, spell.duration) },
    { key: 'components', label: t('spell.components'), value: components },
    { key: 'classes', label: t('spell.classes'), value: (spell.classes ?? []).map((slug) => entries.get(slug)?.name ?? slug).join(', ') },
  ].filter((fact) => fact.value !== '')
  return (
    <Stack gap="md">
      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
        {facts.map((fact) => <div key={fact.key}>
          <Text size="xs" c="dimmed" tt="uppercase">{fact.label}</Text>
          <Text size="sm">{fact.value}</Text>
        </div>)}
        {spell.provenance !== undefined && <div>
          <Text size="xs" c="dimmed" tt="uppercase">{t('spells.filter.source')}</Text>
          <SourceTags provenance={spell.provenance} />
        </div>}
      </SimpleGrid>
      <Markdown>{joinProse(spell.desc ?? [])}</Markdown>
      {(spell.higherLevel?.length ?? 0) > 0 && <Stack gap="sm">
        <Title order={4}>{t('spell.higherLevel')}</Title>
        <Markdown>{joinProse(spell.higherLevel!)}</Markdown>
      </Stack>}
    </Stack>
  )
}
