import { Group, Stack, Text } from '@/ui'
import { useT } from '@/lib/i18n'

import type { Stat } from './options'

/**
 * A weapon's three numbers, each under its own caption: damage, to hit, range.
 *
 * They used to be one dimmed sentence -- "+5 to hit · 1d8+3 · 5 ft." -- which
 * is three facts a player reads off mid-turn written as prose, in an order
 * that changed between the Actions tab and the Equipment tab. Here they are
 * `StatColumns` in one order everywhere, and the two that are rolled are
 * coloured so the eye finds them without reading the captions.
 *
 * A value that is not known is not drawn: a weapon in the backpack has damage
 * and a range but no bonus to hit until somebody wields it.
 */
export function WeaponStats({ damage, hit, range, wrap = false }: {
  damage?: string | undefined
  hit?: string | undefined
  range?: string | undefined
  wrap?: boolean
}) {
  const t = useT()
  const stats: Stat[] = [
    { key: 'damage', label: t('weapon.damage'), value: damage ?? '', color: 'red', width: 132 },
    { key: 'hit', label: t('weapon.hit'), value: hit ?? '', color: 'violet', width: 40 },
    { key: 'range', label: t('weapon.range'), value: range ?? '', width: 92 },
  ].filter((stat) => stat.value !== '')
  if (stats.length === 0) return null
  return <StatColumns stats={stats} wrap={wrap} />
}

/**
 * Numbers as a stat block prints them: the value large, what it is small
 * underneath, a rule between one and the next. The value leads because it is
 * what is read; the caption is there for the first time and ignored after.
 *
 * `wrap` is for a card a third of the page wide, where a third column going
 * under the first is better than one cut off. In a list of rows a column is
 * at least its `width`, so that one row's damage sits over the next row's
 * whatever either says; a card stands alone and takes only what it needs.
 */
export function StatColumns({ stats, wrap = false }: { stats: readonly Stat[]; wrap?: boolean }) {
  if (stats.length === 0) return null
  return (
    <Group gap="sm" wrap={wrap ? 'wrap' : 'nowrap'} style={{ rowGap: 2, ...(wrap ? { minWidth: 0 } : { flexShrink: 0 }) }}>
      {stats.map((stat, at) => (
        <Stack key={stat.key} gap={0} {...(wrap || stat.width === undefined ? {} : { miw: stat.width })} {...(at > 0 ? { pl: 'sm', style: { borderLeft: '1px solid var(--mantine-color-default-border)' } } : {})}>
          <Text size="md" fw={700} lh={1.25} {...(stat.color ? { c: stat.color } : {})} style={{ whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }}>{stat.value}</Text>
          <Text size="xs" c="dimmed" style={{ whiteSpace: 'nowrap' }}>{stat.label}</Text>
        </Stack>
      ))}
    </Group>
  )
}
