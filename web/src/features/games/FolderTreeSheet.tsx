import { useState } from 'react'

import type { Folder, Summary } from '@/lib/api'
import { listCharacters, listFolders } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { Accordion, Button, Checkbox, Group, Loader, ModalSheet, Stack, Text } from '@/ui'

import { classLine } from '@/domain'

/**
 * Your own characters, on the shelves you filed them on.
 *
 * A tree rather than one long list because a folder is how its owner already
 * thinks about their characters: somebody with three campaigns' worth of them
 * knows which shelf tonight's is on, and a flat list makes them read every
 * name to find it.
 *
 * Collapsed by default. Unlike the group tree this one fetches everything up
 * front -- a character listing carries its own folder, so one request covers
 * every branch, and a request per shelf would be slower for no benefit.
 */
export function FolderTreeSheet({
  opened,
  seated,
  pending,
  onClose,
  onAdd,
}: {
  opened: boolean
  seated: Set<string>
  pending: boolean
  onClose: () => void
  onAdd: (ids: string[]) => void
}) {
  const folders = useResource(opened ? 'folders' : '', (signal) => listFolders(signal))
  const characters = useResource(opened ? 'characters:mine' : '', (signal) =>
    listCharacters(undefined, signal),
  )
  const [picked, setPicked] = useState<string[]>([])
  const [open, setOpen] = useState<string[]>([])

  const loading = folders.loading || characters.loading
  const mine = (characters.data?.characters ?? []).filter((c: Summary) => !seated.has(c.id))
  const shelves = (folders.data?.folders ?? []).filter((folder: Folder) =>
    mine.some((c: Summary) => c.folder === folder.id),
  )

  function toggle(id: string) {
    setPicked((current) =>
      current.includes(id) ? current.filter((each) => each !== id) : [...current, id],
    )
  }

  return (
    <ModalSheet opened={opened} onClose={onClose} title="Add your characters">
      <Stack gap="sm">
        <Text size="sm" c="dimmed">
          Seating one of your own puts it on the group&rsquo;s table too, so everybody here can
          read it.
        </Text>

        {loading && <Loader size="sm" />}
        {!loading && mine.length === 0 && (
          <Text size="sm">
            {characters.data === null
              ? 'You have not made a character yet.'
              : 'All of your characters are already at this game.'}
          </Text>
        )}

        <Accordion multiple value={open} onChange={setOpen} variant="contained">
          {shelves.map((folder: Folder) => {
            const inFolder = mine.filter((c: Summary) => c.folder === folder.id)
            return (
              <Accordion.Item key={folder.id} value={folder.id}>
                <Accordion.Control>
                  <Text size="sm">
                    {folder.name}
                    <Text span size="xs" c="dimmed">
                      {' '}
                      &middot; {inFolder.length}
                    </Text>
                  </Text>
                </Accordion.Control>
                <Accordion.Panel>
                  <Stack gap="xs">
                    {inFolder.map((character: Summary) => (
                      <Checkbox
                        key={character.id}
                        checked={picked.includes(character.id)}
                        onChange={() => toggle(character.id)}
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
                  </Stack>
                </Accordion.Panel>
              </Accordion.Item>
            )
          })}
        </Accordion>

        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={picked.length === 0} loading={pending} onClick={() => onAdd(picked)}>
            Add
          </Button>
        </Group>
      </Stack>
    </ModalSheet>
  )
}
