import { useState } from 'react'

import type { ClassLevel } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, Checkbox, Group, Loader, ModalSheet, Stack, Text } from '@/ui'

import { classLine } from '@/domain'

/** The least a row needs to be shown and picked. */
export interface Pickable {
  id: string
  name: string
  classes?: ClassLevel[]
}

/**
 * Pick some characters and seat them.
 *
 * One component for both ways in -- from the group's table, and from your own
 * characters -- because the two differ only in where the list came from and
 * what to say when it is empty. Multi-select rather than a button per row is
 * what makes "everyone at this table" one action without a second control on
 * the screen behind it saying the same thing.
 */
export function PickCharactersSheet({
  opened,
  title,
  description,
  empty,
  characters,
  loading,
  pending,
  onClose,
  onAdd,
}: {
  opened: boolean
  title: string
  description: string
  empty: string
  characters: Pickable[]
  loading: boolean
  pending: boolean
  onClose: () => void
  onAdd: (ids: string[]) => void
}) {
  const t = useT()
  // The caller remounts this with a `key` per source, which is what gives a
  // fresh selection each time it opens -- what was ticked last time is not a
  // draft anybody is coming back to, and resetting it from an effect would
  // start a second render to undo the first.
  const [picked, setPicked] = useState<string[]>([])

  return (
    <ModalSheet opened={opened} onClose={onClose} title={title}>
      <Stack gap="sm">
        <Text size="sm" c="dimmed">
          {description}
        </Text>

        {loading && <Loader size="sm" />}
        {!loading && characters.length === 0 && <Text size="sm">{empty}</Text>}

        {characters.map((character) => (
          <Checkbox
            key={character.id}
            checked={picked.includes(character.id)}
            onChange={() =>
              setPicked((current) =>
                current.includes(character.id)
                  ? current.filter((id) => id !== character.id)
                  : [...current, character.id],
              )
            }
            label={
              <div>
                <Text size="sm">{character.name || 'Unnamed'}</Text>
                <Text size="xs" c="dimmed">
                  {classLine(character.classes)}
                </Text>
              </div>
            }
          />
        ))}

        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            disabled={picked.length === 0}
            loading={pending}
            onClick={() => onAdd(picked)}
          >
            {t('common.add')}
          </Button>
        </Group>
      </Stack>
    </ModalSheet>
  )
}
