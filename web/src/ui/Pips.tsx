import { Group, Text } from '@mantine/core'

import { useT } from '@/lib/i18n'

/** Above this a row of marks stops being countable at a glance. */
const MAX_PIPS = 20
/** Packs spell "no limit" as this capacity; see the barbarian's level 20 rage. */
const UNLIMITED = 9999

export interface PipsProps {
  /** What is being counted; the row's accessible name. */
  name: string
  max: number
  used: number
}

function Pip({ filled }: { filled: boolean }) {
  // The same ring and disc as ProficiencyMark, so the sheet has one mark language.
  return (
    <svg viewBox="0 0 16 16" aria-hidden style={{ width: 16, height: 16, display: 'block' }}>
      <circle cx="8" cy="8" r="5.5" fill="none" stroke="currentColor" strokeWidth="1.5" opacity={filled ? 1 : 0.55} />
      {filled && <circle cx="8" cy="8" r="3" fill="currentColor" />}
    </svg>
  )
}

/**
 * One mark per use of a consumable: a disc for a use still available, a ring
 * for one spent. A display only -- whatever spends a use sits beside it, since
 * a target the size of a mark is too easy to hit by accident at the table.
 * Shape carries the state, as in ProficiencyMark, so it survives monochrome.
 */
export function Pips({ name, max, used }: PipsProps) {
  const t = useT()
  if (max >= UNLIMITED) return <Text size="sm">{t('pips.unlimited')}</Text>
  const available = Math.max(0, max - used)
  return (
    <Group gap={2} wrap="nowrap" role="img" aria-label={t('pips.count', { name, available, max })}>
      {max > MAX_PIPS
        ? <Text size="sm" fw={500} style={{ fontVariantNumeric: 'tabular-nums' }}>{available} / {max}</Text>
        : Array.from({ length: max }, (_, index) => <Pip key={index} filled={index < available} />)}
    </Group>
  )
}
