import { useState } from 'react'

import type { Change } from '@/lib/api'
import { Button, Group, Stack, Textarea } from '@/ui'

import { useT } from '@/lib/i18n'

export interface WrittenFormProps {
  /** What is already written, so changing it starts from what it says. */
  lines?: readonly string[]
  /** The path on the sheet this settles. */
  path: string
  /** What the answer is called, for the field's own label. */
  noun: string
  pending: boolean
  submitLabel: string
  onSubmit: (changes: Change[]) => void
}

/**
 * The questions a player answers in their own words.
 *
 * A personality trait, an ideal, a bond and a flaw. The SRD prints eight of
 * each and the compendium carries them, and they used to *be* the question --
 * a menu of eight, and no way to say anything else. They are suggestions in
 * the book and they are suggestions here; what goes on the sheet is whatever
 * the player writes.
 *
 * One field, and it grows. These are sentences and sometimes several -- "I owe
 * my life to the priest who took me in when my parents died" is already a line
 * and a half at 390px -- so a single-line input would hide the middle of an
 * answer behind a horizontal scroll nobody can see the ends of.
 *
 * Like the six ability scores, this settles a value rather than picking an
 * entry, so it emits the addressed change that settles it. Blank is the same
 * as not answering, and these are optional, so the button simply stays off.
 */
export function WrittenForm({
  lines,
  path,
  noun,
  pending,
  submitLabel,
  onSubmit,
}: WrittenFormProps) {
  const t = useT()
  const [written, setWritten] = useState(() => lines?.join('\n\n') ?? '')

  const kept = written.trim()

  return (
    <Stack gap="md">
      <Textarea
        aria-label={noun}
        placeholder={t('written.placeholder')}
        // Fixed rows rather than autosize: Mantine's autosizing textarea is
        // `react-textarea-autosize`, which measures with a listener jsdom has
        // no element to attach -- so the field could not even be focused in a
        // test. Three rows holds the answers these questions actually get, and
        // a longer one scrolls.
        rows={3}
        value={written}
        onChange={(event) => setWritten(event.currentTarget.value)}
      />
      <Group>
        <Button
          onClick={() => onSubmit([{ path, op: 'set', value: { kind: 'string', string: kept } }])}
          disabled={kept === ''}
          loading={pending}
        >
          {submitLabel}
        </Button>
      </Group>
    </Stack>
  )
}
