import { Group, Stack, Text } from '@/ui'
import { useT } from '@/lib/i18n'

/**
 * A weapon's three numbers, each under its own caption: damage, to hit, range.
 *
 * They used to be one dimmed sentence -- "+5 to hit · 1d8+3 · 5 ft." -- which
 * is three facts a player reads off mid-turn written as prose, in an order
 * that changed between the Actions tab and the Equipment tab. Here they are
 * columns in one order everywhere, and the two that are rolled are coloured so
 * the eye finds them without reading the captions.
 *
 * A value that is not known is not drawn: a weapon in the backpack has damage
 * and a range but no bonus to hit until somebody wields it.
 */
export function WeaponStats({ damage, hit, range, inline = false }: {
  damage?: string | undefined
  hit?: string | undefined
  range?: string | undefined
  /**
   * One line, caption beside value, for a place too narrow for three columns:
   * an equipment slot card is a third of the page and has a name to show too.
   */
  inline?: boolean
}) {
  const t = useT()
  const stats = [
    { key: 'damage', label: t('weapon.damage'), value: damage, color: 'red' },
    { key: 'hit', label: t('weapon.hit'), value: hit, color: 'violet' },
    { key: 'range', label: t('weapon.range'), value: range, color: undefined },
  ].filter((stat) => stat.value !== undefined && stat.value !== '')
  if (stats.length === 0) return null
  if (inline) return (
    // Wraps rather than clips: a third number cut off mid-caption is worse
    // than a second line, and whatever follows gives way to it.
    <Group gap="xs" style={{ rowGap: 0, minWidth: 0 }}>
      {stats.map((stat) => (
        <Text key={stat.key} size="xs" style={{ whiteSpace: 'nowrap' }}>
          <Text span c="dimmed" tt="uppercase">{stat.label}</Text>{' '}
          <Text span fw={700} {...(stat.color ? { c: stat.color } : {})} style={{ fontVariantNumeric: 'tabular-nums' }}>{stat.value}</Text>
        </Text>
      ))}
    </Group>
  )
  return (
    <Group gap="md" wrap="nowrap" style={{ flexShrink: 0 }}>
      {stats.map((stat) => (
        <Stack key={stat.key} gap={0} align="center">
          <Text size="xs" c="dimmed" tt="uppercase" style={{ whiteSpace: 'nowrap' }}>{stat.label}</Text>
          <Text size="sm" fw={700} {...(stat.color ? { c: stat.color } : {})} style={{ whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }}>{stat.value}</Text>
        </Stack>
      ))}
    </Group>
  )
}
