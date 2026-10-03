import { useState } from 'react'

import type { Change } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, Group, NumberInput, Stack } from '@/ui'

import { desiredLevelChange } from './desiredLevel'

import { MAX_LEVEL } from '@/domain'

export interface DesiredLevelFormProps {
  /** The level already declared, or the character's current one to start from. */
  initial: number
  pending: boolean
  submitLabel: string
  onSubmit: (changes: Change[]) => void
}

/**
 * The level the character is being built towards.
 *
 * A number rather than a pick, because the compendium poses no such question:
 * it is the character's own input, like a name, and the answer travels as the
 * change that settles it. What declaring it does is the server's business --
 * every level between here and there opens its own choices.
 */
export function DesiredLevelForm({ initial, pending, submitLabel, onSubmit }: DesiredLevelFormProps) {
  const t = useT()
  const [level, setLevel] = useState<number | string>(Math.min(Math.max(initial, 1), MAX_LEVEL))
  const valid = typeof level === 'number' && level >= 1 && level <= MAX_LEVEL
  const submit = () => {
    if (valid && !pending) onSubmit([desiredLevelChange(level)])
  }

  return (
    <Stack gap="md">
      <NumberInput
        aria-label={t('choice.desiredLevel')}
        min={1}
        max={MAX_LEVEL}
        clampBehavior="strict"
        allowDecimal={false}
        value={level}
        onChange={setLevel}
        onKeyDown={(event) => {
          if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
          event.preventDefault()
          submit()
        }}
      />
      <Group>
        <Button onClick={submit} disabled={!valid} loading={pending}>
          {submitLabel}
        </Button>
      </Group>
    </Stack>
  )
}
