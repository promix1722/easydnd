import { useState } from 'react'

import type { Change } from '@/lib/api'
import { Button, Group, Stack, Text, TextInput } from '@/ui'

export interface WrittenFormProps {
  /** How many lines the question asks for: acolyte suggests two traits. */
  choose: number
  /** What is already written, so changing it starts from what it says. */
  lines?: readonly string[]
  /** The path on the sheet these lines settle. */
  path: string
  /** What one of them is called, for the field's own label. */
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
 * Like the six ability scores, this settles a value rather than picking an
 * entry, so it emits the addressed changes that settle it: the first line sets
 * the list and the rest add to it, which is exactly how the list is stored.
 *
 * Blank lines are dropped rather than stored, so a question asking for two
 * traits can be answered with one. Answering with none is the same as not
 * answering, and the button says so.
 */
export function WrittenForm({
  choose,
  lines,
  path,
  noun,
  pending,
  submitLabel,
  onSubmit,
}: WrittenFormProps) {
  const [written, setWritten] = useState<string[]>(() =>
    Array.from({ length: choose }, (_, at) => lines?.[at] ?? ''),
  )

  const kept = written.map((line) => line.trim()).filter((line) => line !== '')
  const blank = kept.length === 0

  const submit = () => {
    onSubmit(
      kept.map((line, at) => ({
        path,
        op: at === 0 ? 'set' : 'add',
        value: { kind: 'string', string: line },
      })),
    )
  }

  return (
    <Stack gap="md">
      {written.map((line, at) => (
        <TextInput
          // The index is the identity here: these are N slots for one kind of
          // answer, not a list anything is inserted into or removed from.
          key={at}
          aria-label={choose === 1 ? noun : `${noun} ${at + 1}`}
          placeholder="In their own words..."
          value={line}
          onChange={(event) => {
            const next = event.currentTarget.value
            setWritten((current) => current.map((was, i) => (i === at ? next : was)))
          }}
        />
      ))}
      <Group>
        <Button onClick={submit} disabled={blank} loading={pending}>
          {submitLabel}
        </Button>
      </Group>
      {choose > 1 && (
        <Text size="xs" c="dimmed">
          Two is what the background suggests. One is an answer too.
        </Text>
      )}
    </Stack>
  )
}
