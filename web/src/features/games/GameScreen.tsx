import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'

import type { TableCharacter } from '@/lib/api'
import {
  addToGame,
  deleteGame,
  getGame,
  listTable,
  removeFromGame,
  renameGame,
} from '@/lib/api'
import { useAction } from '@/lib/useAction'
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
  MAX_TABLE_WIDTH,
  IconPencil,
  IconPlus,
  IconTrash,
  Loader,
  ModalSheet,
  Stack,
  Text,
  TextInput,
  Title,
} from '@/ui'

import { atLeast, ROLE_LABELS } from '../groups/roles'
import { FolderTreeSheet } from './FolderTreeSheet'
import { PickCharactersSheet } from './PickCharactersSheet'

import { classLine } from '@/domain'

/** One game: its name, and who is at it. */
export function GameScreen() {
  const { id: gameId = '' } = useParams()
  const navigate = useNavigate()
  const { data, error, loading, reload } = useResource(`game:${gameId}`, (signal) =>
    getGame(gameId, signal),
  )

  const [renaming, setRenaming] = useState(false)
  const [name, setName] = useState('')
  const [picking, setPicking] = useState<'group' | 'mine' | null>(null)
  const add = useAction(addToGame)

  // The group's table is one flat list: a game is played at exactly one group,
  // so there is nothing to branch on. Your own characters are a tree, and load
  // themselves -- see FolderTreeSheet.
  const table = useResource(
    picking === 'group' && data !== null ? `table:${data.group_id}` : '',
    (signal) => listTable(data?.group_id ?? '', signal),
  )
  const drop = useAction(removeFromGame)
  const rename = useAction(renameGame)
  const destroy = useAction(deleteGame)

  if (loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Loading the game...
        </Text>
      </Group>
    )
  }
  if (error !== null || data === null) {
    return (
      <Alert color="red" title="Could not load this game">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{error ?? 'That game is not there.'}</Text>
          <Button variant="light" onClick={reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const game = data
  const canManage = atLeast(game.role, 'dm')
  const failure = add.error ?? drop.error ?? rename.error ?? destroy.error

  async function act(work: Promise<unknown | null>) {
    if ((await work) === null) return
    reload()
  }

  // Already-seated characters are not offered again: adding one twice is a
  // no-op the server absorbs, but a list that offers it says otherwise.
  const seated = new Set(game.characters.map((c) => c.id))
  const seatable = (table.data?.characters ?? []).filter((c) => !seated.has(c.id))

  async function close() {
    if ((await destroy.run(gameId)) === null) return
    await navigate('/games', { replace: true })
  }

  return (
    <Stack gap="md">
      {/* Capped to the table's width, so the controls land on its right edge
          rather than the window's. */}
      <Group justify="space-between" align="flex-start" maw={MAX_TABLE_WIDTH}>
        <div>
          {/* Your rank, shown the way a group shows it -- a game is reached
              from its own section, so the page has to say what you are at the
              table it is played at rather than leaving you to remember. */}
          <Group gap="xs" align="center">
            <Title order={2}>{game.name}</Title>
            <Badge variant="light">{ROLE_LABELS[game.role]}</Badge>
          </Group>
          <Anchor component={Link} to={`/groups/${game.group_id}`}>
            <Text size="sm" c="dimmed">
              Back to the group
            </Text>
          </Anchor>
        </div>
        {canManage && (
          <Group gap="xs">
            <Button
              size={ACTION_SIZE}
              variant="subtle"
              leftSection={<IconPencil size={ACTION_ICON_SIZE} />}
              onClick={() => {
                setName(game.name)
                setRenaming(true)
              }}
            >
              Rename
            </Button>
            <Button
              size={ACTION_SIZE}
              color="red"
              variant="subtle"
              leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
              loading={destroy.pending}
              onClick={() => void close()}
            >
              Delete
            </Button>
          </Group>
        )}
      </Group>

      {failure !== null && (
        <Alert color="red" title="That did not work">
          {failure}
        </Alert>
      )}

      <DataList
        items={game.characters}
        getKey={(character) => character.id}
        columns={[
          {
            key: 'name',
            header: 'Character',
            primary: true,
            render: (character: TableCharacter) => (
              <Anchor component={Link} to={`/groups/${game.group_id}/characters/${character.id}`}>
                <Text size="sm">{character.name || 'Unnamed'}</Text>
              </Anchor>
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
              if (!canManage) return null
              return (
                <Button
                  size={ACTION_SIZE}
                  variant="subtle"
                  color="red"
                  onClick={() => void act(drop.run(gameId, character.id))}
                >
                  {/* Out of this game, not off the table: giving up a seat is
                      not taking the character back. */}
                  Remove
                </Button>
              )
            },
          },
        ]}
        empty="Nobody is at this game yet."
      />

      {/* Under the table, on the left. Two ways in, because they are two
          different questions: one picks from what a group already shares, the
          other from your own characters, which land on this game's table by
          being seated. */}
      {canManage && (
        <Group>
          <Button
            size={ACTION_SIZE}
            variant="light"
            leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
            onClick={() => setPicking('group')}
          >
            Add character from group
          </Button>
          <Button
            size={ACTION_SIZE}
            variant="light"
            leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
            onClick={() => setPicking('mine')}
          >
            Add my characters
          </Button>
        </Group>
      )}

      <PickCharactersSheet
        key={picking === 'group' ? 'group' : 'group-closed'}
        opened={picking === 'group'}
        title="Add from the group"
        description="Everything this group has shared. Tick as many as you want."
        empty="Nobody has shared a character with this group yet."
        characters={seatable}
        loading={table.loading}
        pending={add.pending}
        onClose={() => setPicking(null)}
        onAdd={(ids) => {
          setPicking(null)
          void act(add.run(gameId, ids))
        }}
      />

      <FolderTreeSheet
        key={picking === 'mine' ? 'mine' : 'mine-closed'}
        opened={picking === 'mine'}
        seated={seated}
        pending={add.pending}
        onClose={() => setPicking(null)}
        onAdd={(ids) => {
          setPicking(null)
          void act(add.run(gameId, ids))
        }}
      />

      <ModalSheet opened={renaming} onClose={() => setRenaming(false)} title="Rename this game">
        <Stack gap="sm">
          <TextInput
            label="Name"
            value={name}
            error={rename.fields.find((field) => field.field === 'name')?.message}
            onChange={(event) => setName(event.currentTarget.value)}
          />
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setRenaming(false)}>
              Cancel
            </Button>
            <Button
              loading={rename.pending}
              onClick={() => {
                setRenaming(false)
                void act(rename.run(gameId, name))
              }}
            >
              Rename
            </Button>
          </Group>
        </Stack>
      </ModalSheet>
    </Stack>
  )
}
