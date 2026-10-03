import { useParams } from 'react-router'

import type { Entry, Spell } from '@/lib/api'
import { bySlug, getCollection, getEntries } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Badge, Group, Page, Panel, pageState } from '@/ui'

import { SpellIcon } from './spellIcon'
import { levelText } from './spellText'
import { SpellDetails } from './SpellDetails'

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

  return (
    <Page
      trail={[{ label: spell.name }]}
      mark={<SpellIcon slug={spell.slug} size={40} />}
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
        <SpellDetails spell={spell} entries={bySlug(classes)} />
      </Panel>
    </Page>
  )
}
