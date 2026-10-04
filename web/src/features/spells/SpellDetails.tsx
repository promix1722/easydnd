import type { Entry, Spell } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { joinProse, Markdown, SourceTags, SimpleGrid, Stack, Text, Title } from '@/ui'

import { castingTimeText, componentsAbbrev, durationText, rangeText } from './spellText'

/** Full spell rules, shared by the compendium and selection previews. */
export function SpellDetails({ spell, entries }: { spell: Spell; entries: ReadonlyMap<string, Entry> }) {
  const t = useT()
  const components = [componentsAbbrev(t, spell.components), spell.components?.text]
    .filter(Boolean).join(' · ')
  const facts = [
    { key: 'time', label: t('spell.castingTime'), value: castingTimeText(t, spell.castingTime) },
    { key: 'range', label: t('spell.range'), value: rangeText(t, spell.range) },
    { key: 'duration', label: t('spell.duration'), value: durationText(t, spell.duration) },
    { key: 'components', label: t('spell.components'), value: components },
    { key: 'classes', label: t('spell.classes'), value: (spell.classes ?? []).map((slug) => entries.get(slug)?.name ?? slug).join(', ') },
  ].filter((fact) => fact.value !== '')
  return (
    <Stack gap="md">
      <SourceTags provenance={spell.provenance} />
      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
        {facts.map((fact) => <div key={fact.key}>
          <Text size="xs" c="dimmed" tt="uppercase">{fact.label}</Text>
          <Text size="sm">{fact.value}</Text>
        </div>)}
      </SimpleGrid>
      <Markdown>{joinProse(spell.desc ?? [])}</Markdown>
      {(spell.higherLevel?.length ?? 0) > 0 && <Stack gap="sm">
        <Title order={4}>{t('spell.higherLevel')}</Title>
        <Markdown>{joinProse(spell.higherLevel!)}</Markdown>
      </Stack>}
    </Stack>
  )
}
