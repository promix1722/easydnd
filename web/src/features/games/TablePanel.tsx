import { useState } from 'react'
import { Link } from 'react-router'

import type { GroupRole, Summary, TableCharacter } from '@/lib/api'
import { listCharacters, listTable, shareCharacter, unshareCharacter } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useAuth } from '@/lib/auth'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  ACTION_SIZE,
  Alert,
  Anchor,
  Badge,
  Button,
  DataList,
  Group,
  IconPlus,
  Loader,
  ModalSheet,
  Stack,
  Text,
} from '@/ui'

import { atLeast } from '../groups/roles'

import { classLine } from '@/domain'

/**
 * The characters a group's members have put on its table.
 *
 * Sharing is a read and nothing more: every member can open any sheet here,
 * and only the owner of a character can ever change it. The controls below say
 * so by what they do not offer -- there is no edit anywhere on this panel,
 * because there is no route behind one.
 */
export function TablePanel({ groupId, role }: { groupId: string; role: GroupRole }) {
  const { user } = useAuth()
  const me = user?.id ?? ''
  const canManage = atLeast(role, 'dm')

  const table = useResource(`table:${groupId}`, (signal) => listTable(groupId, signal))
  const [sharing, setSharing] = useState(false)
  const share = useAction(shareCharacter)
  const unshare = useAction(unshareCharacter)

  if (table.loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Loading the table...
        </Text>
      </Group>
    )
  }
  if (table.error !== null || table.data === null) {
    return (
      <Alert color="red" title="Could not load this table">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{table.error ?? 'Unknown error'}</Text>
          <Button variant="light" onClick={table.reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const characters = table.data.characters
  const failure = share.error ?? unshare.error

  async function act(work: Promise<unknown | null>) {
    if ((await work) === null) return
    table.reload()
  }

  return (
    <Stack gap="md">
      {failure !== null && (
        <Alert color="red" title="That did not work">
          {failure}
        </Alert>
      )}

      <DataList
        items={characters}
        getKey={(character) => character.id}
        columns={[
          {
            key: 'name',
            header: 'Character',
            primary: true,
            render: (character: TableCharacter) => (
              <Group gap="xs">
                {/* The sheet, and only the sheet. The event log is the record
                    of its owner's decisions and is not the table's business. */}
                <Anchor component={Link} to={`/groups/${groupId}/characters/${character.id}`}>
                  <Text size="sm">{character.name || 'Unnamed'}</Text>
                </Anchor>
                {character.owner_id === me && (
                  <Badge size="xs" variant="light">
                    Yours
                  </Badge>
                )}
              </Group>
            ),
          },
          {
            key: 'classes',
            header: 'Class',
            render: (character: TableCharacter) => classLine(character.classes),
          },
          {
            key: 'level',
            header: 'Level',
            render: (character: TableCharacter) => character.level,
          },
          {
            key: 'actions',
            header: '',
            render: (character: TableCharacter) => {
              // Your own always; anybody's if you run the table, because a
              // guest's session ends and their character would otherwise sit
              // here with nobody able to take it down.
              if (character.owner_id !== me && !canManage) return null
              return (
                <Button
                  size={ACTION_SIZE}
                  variant="subtle"
                  color="red"
                  onClick={() => void act(unshare.run(groupId, character.id))}
                >
                  Take off
                </Button>
              )
            },
          },
        ]}
        empty="Nothing on the table yet."
      />

      {/* Under the table, on the left, like every other way of adding a row. */}
      <Group>
        <Button
          size={ACTION_SIZE}
          variant="light"
          leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
          onClick={() => setSharing(true)}
        >
          Add a character
        </Button>
      </Group>

      <ShareSheet
        opened={sharing}
        already={characters.map((character) => character.id)}
        pending={share.pending}
        onClose={() => setSharing(false)}
        onPick={(id) => void act(share.run(groupId, id)).then(() => setSharing(false))}
      />
    </Stack>
  )
}

/** Pick one of your own characters to put on the table. */
function ShareSheet({
  opened,
  already,
  pending,
  onClose,
  onPick,
}: {
  opened: boolean
  already: string[]
  pending: boolean
  onClose: () => void
  onPick: (id: string) => void
}) {
  const mine = useResource(opened ? 'characters:mine' : '', (signal) =>
    listCharacters(undefined, signal),
  )
  const characters = mine.data?.characters ?? []

  return (
    <ModalSheet opened={opened} onClose={onClose} title="Put a character on the table">
      <Stack gap="sm">
        <Text size="sm" c="dimmed">
          Everyone at this table will be able to read the sheet. Only you can change it.
        </Text>
        {mine.loading && <Loader size="sm" />}
        {characters.length === 0 && !mine.loading && (
          <Text size="sm">You have not made a character yet.</Text>
        )}
        {characters.map((character: Summary) => {
          const shared = already.includes(character.id)
          return (
            <Group key={character.id} justify="space-between">
              <div>
                <Text size="sm">{character.name || 'Unnamed'}</Text>
                <Text size="xs" c="dimmed">
                  {classLine(character.classes)}
                </Text>
              </div>
              <Button
                size="compact-sm"
                variant="light"
                disabled={shared}
                loading={pending}
                onClick={() => onPick(character.id)}
              >
                {shared ? 'Already there' : 'Add'}
              </Button>
            </Group>
          )
        })}
      </Stack>
    </ModalSheet>
  )
}
