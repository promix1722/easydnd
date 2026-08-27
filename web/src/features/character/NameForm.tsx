import { Button, Group, Stack, TextInput } from '@/ui'

import { useT } from '@/lib/i18n'

export interface NameFormProps {
  value: string
  onValueChange: (value: string) => void
  pending: boolean
  /** The server's complaint about the name, where it made one. */
  error?: string
  submitLabel: string
  onSubmit: (name: string) => void
}

/**
 * The one text field this application collects.
 *
 * A name is a selection like any other and is written as its own entry, which
 * is what makes it changeable: replacing the init entry with another init
 * entry renames a character and moves nothing after it. That is also why this
 * form does not know whether it is creating or renaming -- only its caller
 * knows whether there is an entry to replace.
 *
 * It is controlled rather than holding the draft itself, because on a
 * character that does not exist yet the draft *is* the character: clicking any
 * tab creates one from it, and a name locked inside this component would be a
 * name the screen around it could not act on.
 *
 * It carries no caption of any kind, like every other answering surface. The
 * block it opens inside is headed "A name", and this used to add a heading --
 * "What are they called?" -- and a field label under that, so the first screen
 * anybody sees said the same thing three times. The label survives as an
 * `aria-label`, because a field still has to be named to whoever cannot see
 * where it sits.
 */
export function NameForm({
  value,
  onValueChange,
  pending,
  error,
  submitLabel,
  onSubmit,
}: NameFormProps) {
  const t = useT()
  const blank = value.trim() === ''

  return (
    <Stack gap="md">
      <TextInput
        aria-label={t('common.name')}
        placeholder={t('common.name')}
        value={value}
        error={error}
        onChange={(event) => onValueChange(event.currentTarget.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !blank) onSubmit(value.trim())
        }}
      />
      <Group>
        <Button onClick={() => onSubmit(value.trim())} disabled={blank} loading={pending}>
          {submitLabel}
        </Button>
      </Group>
    </Stack>
  )
}
