import { useParams } from 'react-router'

import type { Entry, Spell } from '@/lib/api'
import { bySlug, getCollection, getEntries } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Badge, Group, Page, Panel, SimpleGrid, Stack, Text, Title, pageState } from '@/ui'

import { castingTimeText, componentsAbbrev, durationText, levelText, rangeText } from './spellText'

/**
 * One spell, at full fidelity.
 *
 * The list serves summaries; this screen asks the same endpoint for the whole
 * entry with `?slugs=`, which is where the description, the material text and
 * the rule values past casting time live.
 */
export function SpellScreen() {
  const t = useT()
  const { slug = '' } = useParams()

  const loaded = useResource(`spell:${slug}`, async () => {
    const [spells, schools, classes] = await Promise.all([
      getEntries<Spell>('spells', [slug]),
      getCollection<Entry>('magic-schools'),
      getCollection<Entry>('classes'),
    ])
    return { spell: spells[0] ?? null, schools, classes }
  })

  const state = pageState(loaded, {
    title: t('spells.loadFailed'),
    fallback: t('error.unknown'),
    onRetry: loaded.reload,
  })
  if (state.kind !== 'ready' || loaded.data === null) {
    return <Page trail={[{ label: null }]} state={state} />
  }

  const { spell, schools, classes } = loaded.data
  if (spell === null) {
    // The endpoint drops a slug it does not know rather than failing, so an
    // unknown spell is a 200 with nothing in it.
    return (
      <Page
        trail={[{ label: slug }]}
        state={{ kind: 'failed', title: t('spells.loadFailed'), detail: t('spells.notFound') }}
      />
    )
  }

  const schoolName = bySlug(schools).get(spell.school ?? '')?.name
  const classNames = bySlug(classes)
  const componentsLine = [
    componentsAbbrev(t, spell.components),
    spell.components?.text === undefined ? '' : `(${spell.components.text})`,
  ]
    .filter((part) => part !== '')
    .join(' ')

  const facts: { key: string; label: string; value: string }[] = [
    { key: 'castingTime', label: t('spell.castingTime'), value: castingTimeText(t, spell.castingTime) },
    { key: 'range', label: t('spell.range'), value: rangeText(t, spell.range) },
    { key: 'duration', label: t('spell.duration'), value: durationText(t, spell.duration) },
    { key: 'components', label: t('spell.components'), value: componentsLine },
    {
      key: 'classes',
      label: t('spell.classes'),
      value: (spell.classes ?? [])
        .map((name) => classNames.get(name)?.name ?? name)
        .join(', '),
    },
  ]

  return (
    <Page
      trail={[{ label: spell.name }]}
      badge={
        <Group gap="xs">
          {spell.concentration === true && (
            <Badge size="sm" variant="light">
              {t('spell.concentration')}
            </Badge>
          )}
          {spell.ritual === true && (
            <Badge size="sm" variant="light" color="grape">
              {t('spell.ritual')}
            </Badge>
          )}
        </Group>
      }
      subtitle={[levelText(t, spell.level), schoolName].filter(Boolean).join(' · ')}
    >
      <Panel>
        <Stack gap="md">
          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
            {facts
              .filter((fact) => fact.value !== '')
              .map((fact) => (
                <div key={fact.key}>
                  <Text size="xs" c="dimmed" tt="uppercase">
                    {fact.label}
                  </Text>
                  <Text size="sm">{fact.value}</Text>
                </div>
              ))}
          </SimpleGrid>

          <Stack gap="sm">
            {(spell.desc ?? []).map((paragraph, index) => (
              <Text key={index} size="sm">
                {paragraph}
              </Text>
            ))}
          </Stack>

          {spell.higherLevel !== undefined && spell.higherLevel.length > 0 && (
            <Stack gap="sm">
              <Title order={4}>{t('spell.higherLevel')}</Title>
              {spell.higherLevel.map((paragraph, index) => (
                <Text key={index} size="sm">
                  {paragraph}
                </Text>
              ))}
            </Stack>
          )}
        </Stack>
      </Panel>
    </Page>
  )
}
