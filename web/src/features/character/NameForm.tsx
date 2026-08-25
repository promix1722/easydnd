import { Button, Group, Stack, Text, TextInput } from '@/ui'

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
 */
export function NameForm({
  value,
  onValueChange,
  pending,
  error,
  submitLabel,
  onSubmit,
}: NameFormProps) {
  const blank = value.trim() === ''

  return (
    <Stack gap="md">
      {/*
        The one surface that keeps a heading of its own. The block above it is
        headed "A name", which is what the choice is called, not what it asks
        -- and this is the first line anybody reads in this application.
      */}
      <Text fw={600}>What are they called?</Text>
      <TextInput
        label="Name"
        placeholder="Who are they?"
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
