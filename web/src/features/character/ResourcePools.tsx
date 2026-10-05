import { Fragment } from 'react'

import { ActionIcon, Box, Group, Pips, Text } from '@/ui'
import { useT } from '@/lib/i18n'

/** One spendable resource, as the sheet and the game entry both describe it. */
export interface ResourcePool {
  id: string
  name?: string
  group?: string
  max: number
  used: number
  dice?: string
  slotLevel?: number
}

/**
 * A character's consumables, one row of marks each: spell slots by level,
 * each named as one so the rows below do not read as slots, then every named pool. Read-only without `onChange`.
 * Names arrive localised from the pack, so nothing here knows what a ki point is.
 */
export function ResourcePools({ pools, onChange, disabled = false }: {
  pools: ResourcePool[]
  onChange?: (id: string, used: number) => void
  disabled?: boolean
}) {
  const t = useT()
  const shown = pools.filter((pool) => pool.max > 0)
  const slots = shown.filter((pool) => pool.group === 'spell-slots')
    .sort((a, b) => (a.slotLevel ?? 0) - (b.slotLevel ?? 0))
  const rest = shown.filter((pool) => pool.group !== 'spell-slots')
  // One grid for every row, so the buttons, the names and the marks each
  // start on one line however long a name is. The buttons lead: a thumb finds
  // them at the same place in every row, whatever is written after them.
  const row = (pool: ResourcePool, label: string) => (
    <Fragment key={pool.id}>
      {onChange && (pool.max >= 9999 ? <span /> : <Group gap={4} wrap="nowrap">
        <ActionIcon variant="default" size="sm" aria-label={t('pips.spend', { name: label })}
          disabled={disabled || pool.used >= pool.max} onClick={() => onChange(pool.id, pool.used + 1)}>−</ActionIcon>
        <ActionIcon variant="default" size="sm" aria-label={t('pips.restore', { name: label })}
          disabled={disabled || pool.used <= 0} onClick={() => onChange(pool.id, pool.used - 1)}>+</ActionIcon>
      </Group>)}
      <Text size="sm">{label}</Text>
      <Pips name={label} max={pool.max} used={pool.used} />
    </Fragment>
  )
  return (
    <Box style={{ display: 'grid', gridTemplateColumns: onChange ? 'auto auto 1fr' : 'auto 1fr', columnGap: 'var(--mantine-spacing-md)', rowGap: 'var(--mantine-spacing-xs)', alignItems: 'center' }}>
      {slots.map((pool) => row(pool, pool.name
        ? t('sheet.namedSlotLevel', { name: pool.name, level: pool.slotLevel ?? 0 })
        : t('sheet.slotLevel', { level: pool.slotLevel ?? 0 })))}
      {rest.map((pool) => row(pool, pool.group === 'hit-dice'
        ? t('sheet.hitDie', { dice: (pool.dice ?? '').replace(/^1/, '') })
        : pool.dice ? `${pool.name ?? pool.id} (${pool.dice.replace(/^1/, '')})` : pool.name ?? pool.id))}
    </Box>
  )
}
