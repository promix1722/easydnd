import type { GameEntry } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Divider, SimpleGrid, Stack, Text, useIsDesktop } from '@/ui'
import { ABILITY_ORDER, signed, titleCase } from '@/domain'
import { abilityAbbr, senseName, speedName } from '../character/labels'

const STAT_COLUMNS = { base: 2, sm: 4, md: 7 } as const

export function CompactStats({ entry, expanded, detailsId }: { entry: GameEntry; expanded: boolean; detailsId: string }) {
  const t = useT()
  const desktop = useIsDesktop()
  const stats = entry.stats
  if (!stats) return null
  const values = [
    [t('game.hp'), `${entry.hp} / ${stats.max_hp}`],
    [t('game.tempHp'), entry.temp_hp ?? 0],
    [t('game.ac'), stats.armor_class],
    // A class is named only to tell two DCs apart; one caster's number needs no label.
    [t('game.spellDc'), (stats.spellcasting ?? []).map((caster, _, all) => all.length > 1 ? `${titleCase(caster.class)} ${caster.saveDC}` : caster.saveDC).join(' · ') || '—'],
    [t('vitals.speed'), (stats.speeds ?? []).map((speed) => `${speedName(t, speed.kind)} ${t('vitals.feet', { distance: speed.distance })}`).join(' · ') || '—'],
    [t('vitals.vision'), (stats.senses ?? []).map((sense) => `${senseName(t, sense.kind)} ${t('vitals.feet', { distance: sense.distance })}`).join(' · ') || t('vitals.normalVision')],
    [t('vitals.initiative'), entry.initiative ?? '—'],
  ]
  const field = ([label, value]: typeof values[number]) => (
    <Stack key={label} gap={0} style={{ minWidth: 0 }}>
      <Text size="xs" c="dimmed" style={{ overflowWrap: 'anywhere' }}>{label}</Text>
      <Text size="sm" fw={500} style={{ overflowWrap: 'anywhere' }}>{value}</Text>
    </Stack>
  )
  const abilities = <SimpleGrid cols={STAT_COLUMNS} spacing="sm" verticalSpacing="xs">
    {ABILITY_ORDER.map((ability) => (
      <Stack key={ability} gap={0}>
        <Text size="xs" c="dimmed">{abilityAbbr(t, ability)}</Text>
        <Text size="sm" fw={500}>
          {stats.abilities.scores[ability] ?? 10} ({signed(stats.abilities.modifiers[ability] ?? 0)})
        </Text>
      </Stack>
    ))}
  </SimpleGrid>
  return <>
    <Divider />
    <SimpleGrid cols={desktop ? STAT_COLUMNS : 5} spacing={desktop ? 'sm' : 'xs'} verticalSpacing="xs">
      {(desktop ? values : [
        // What is asked mid-turn, and five of them across a phone: current hit points without the maximum, and initiative as a letter.
        [t('game.hp'), entry.hp ?? '—'], values[1]!, values[2]!, [t('game.initiativeShort'), entry.initiative ?? '—'],
        [t('game.spellDc'), (stats.spellcasting ?? []).map((caster) => caster.saveDC).join(' · ') || '—'],
      ] satisfies typeof values).map(field)}
    </SimpleGrid>
    {desktop ? abilities : expanded && <Stack gap="xs" id={detailsId}>
        <SimpleGrid cols={2} spacing="xs">{values.slice(4, 6).map(field)}</SimpleGrid>
        {abilities}
      </Stack>}
  </>
}
