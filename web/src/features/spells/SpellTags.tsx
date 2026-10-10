import type { Spell } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Badge, useIsDesktop } from '@/ui'

/** The same accessible marks in the compendium and character builder. */
export function SpellTags({ spell }: { spell: Spell }) {
  const t = useT()
  const isDesktop = useIsDesktop()
  return <>
    {spell.concentration === true && (
      <Badge size="sm" variant="light" aria-label={t('spell.concentration')}>
        {isDesktop ? t('spell.concentration') : t('spell.concentrationShort')}
      </Badge>
    )}
    {spell.ritual === true && (
      <Badge size="sm" variant="light" color="grape" aria-label={t('spell.ritual')}>
        {isDesktop ? t('spell.ritual') : t('spell.ritualShort')}
      </Badge>
    )}
  </>
}
