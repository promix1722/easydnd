import { useState } from 'react'
import { Link, useNavigate } from 'react-router'

import type { GameSummary, GroupSummary } from '@/lib/api'
import { createGame, deleteGame, listGames, listGroups, renameGame } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  ACTION_SIZE,
  Alert,
  Anchor,
  Button,
  DataList,
  Group,
  IconPencil,
  IconPlus,
  IconTrash,
  ModalSheet,
  Page,
  pageState,
  Select,
  Stack,
  Text,
  TextInput,
} from '@/ui'

import { atLeast } from '../groups/roles'

/**
 * Every game you are at, across every table.
 *
 * A section of its own rather than a corner of a group. A game belongs to a
 * group the way a character belongs to a folder -- the group is a fact about
 * the game, not the way in to it -- and somebody at three tables wants one
 * list rather than having to remember which table Thursday's game is at.
 */
export function GamesScreen() {
  const navigate = useNavigate()
  const games = useResource('games', (signal) => listGames(signal))
  // The groups are for the picker, and for deciding whether to offer one at
  // all: you can only open a game at a table you run.
  const groups = useResource('groups:mine', (signal) => listGroups(signal))

  const [opening, setOpening] = useState(false)
  const [name, setName] = useState('')
  const [group, setGroup] = useState<string | null>(null)
  const [renaming, setRenaming] = useState<GameSummary | null>(null)
  const [deleting, setDeleting] = useState<GameSummary | null>(null)
  const [newName, setNewName] = useState('')
  const open = useAction(createGame)
  const rename = useAction(renameGame)
  const destroy = useAction(deleteGame)

  async function act(work: Promise<unknown | null>) {
    if ((await work) === null) return
    games.reload()
  }

  const state = pageState(games, {
    title: 'Could not load your games',
    fallback: 'Unknown error',
    onRetry: games.reload,
  })

  // This screen used to be the odd one of the three: where Characters and
  // Groups drew a spinner above their table, it replaced the entire page --
  // so a failed list took the heading with it. It no longer does; `Page` puts
  // the state where the table would be and leaves the heading alone.
  if (state.kind !== 'ready' || games.data === null) {
    return <Page trail={[]} state={state} />
  }

  const tables = (groups.data?.groups ?? []).filter((g: GroupSummary) => atLeast(g.role, 'dm'))

  // Whether a row's controls are drawn is the caller's rank at *that* game's
  // table. A DM at one group and a player at another must not get a Delete on
  // the second because they have one on the first.
  function canManage(groupId: string): boolean {
    return tables.some((g: GroupSummary) => g.id === groupId)
  }

  async function create() {
    if (group === null) return
    const made = await open.run(group, name)
    if (made === null) return
    setOpening(false)
    setName('')
    await navigate(`/games/${made.id}`)
  }

  return (
    <Page trail={[]}>
      <Stack gap="md">
        <DataList
          items={games.data.games}
          getKey={(game) => game.id}
          columns={[
            {
              key: 'name',
              header: 'Game',
              primary: true,
              render: (game: GameSummary) => (
                <Anchor component={Link} to={`/games/${game.id}`}>
                  <Text size="sm">{game.name}</Text>
                </Anchor>
              ),
            },
            {
              key: 'group',
              header: 'Group',
              render: (game: GameSummary) => (
                <Anchor component={Link} to={`/groups/${game.group_id}`}>
                  <Text size="sm" c="dimmed">
                    {game.group_name}
                  </Text>
                </Anchor>
              ),
            },
            {
              key: 'actions',
              header: '',
              render: (game: GameSummary) => {
                // Each row edits its own game and nothing else. Whether you may
                // is your rank at *that* table, which the row already carries.
                if (!canManage(game.group_id)) return null
                return (
                  <Group gap="xs" justify="flex-end" wrap="nowrap">
                    <Button
                      size={ACTION_SIZE}
                      variant="subtle"
                      leftSection={<IconPencil size={ACTION_ICON_SIZE} />}
                      onClick={() => {
                        setNewName(game.name)
                        setRenaming(game)
                      }}
                    >
                      Rename
                    </Button>
                    <Button
                      size={ACTION_SIZE}
                      variant="subtle"
                      color="red"
                      leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
                      onClick={() => setDeleting(game)}
                    >
                      Delete
                    </Button>
                  </Group>
                )
              },
            },
          ]}
          empty={
            tables.length > 0
              ? 'No games yet. Open one at a table you run.'
              : 'No games yet. A DM at one of your groups opens them.'
          }
        />

        {/* Under the table, on the left. */}
        {tables.length > 0 && (
          <Group>
            <Button
              size={ACTION_SIZE}
              variant="light"
              leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
              onClick={() => {
                setGroup(tables[0]?.id ?? null)
                setOpening(true)
              }}
            >
              New game
            </Button>
          </Group>
        )}

        <ModalSheet
          opened={renaming !== null}
          onClose={() => setRenaming(null)}
          title="Rename this game"
        >
          <Stack gap="sm">
            <TextInput
              label="Name"
              value={newName}
              error={rename.fields.find((field) => field.field === 'name')?.message}
              onChange={(event) => setNewName(event.currentTarget.value)}
              data-autofocus
            />
            {rename.error !== null && (
              <Alert color="red" title="Could not rename it">
                {rename.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setRenaming(null)}>
                Cancel
              </Button>
              <Button
                loading={rename.pending}
                onClick={() => {
                  const target = renaming
                  setRenaming(null)
                  if (target !== null) void act(rename.run(target.id, newName))
                }}
              >
                Rename
              </Button>
            </Group>
          </Stack>
        </ModalSheet>

        <ModalSheet
          opened={deleting !== null}
          onClose={() => setDeleting(null)}
          title="Delete this game"
        >
          <Stack gap="sm">
            <Text size="sm">
              {deleting?.name} goes. The characters stay on {deleting?.group_name}&rsquo;s table --
              they were never the game&rsquo;s.
            </Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setDeleting(null)}>
                Cancel
              </Button>
              <Button
                color="red"
                loading={destroy.pending}
                onClick={() => {
                  const target = deleting
                  setDeleting(null)
                  if (target !== null) void act(destroy.run(target.id))
                }}
              >
                Delete
              </Button>
            </Group>
          </Stack>
        </ModalSheet>

        <ModalSheet opened={opening} onClose={() => setOpening(false)} title="Open a game">
          <Stack gap="sm">
            <Select
              label="Group"
              data={tables.map((g: GroupSummary) => ({ value: g.id, label: g.name }))}
              value={group}
              onChange={setGroup}
              allowDeselect={false}
            />
            <TextInput
              label="Name"
              placeholder="Thursday night"
              value={name}
              error={open.fields.find((field) => field.field === 'name')?.message}
              onChange={(event) => setName(event.currentTarget.value)}
              data-autofocus
            />
            {/* Inside the dialog that caused it. Raised to the page it sat
                behind the modal, where it read as a failure of the whole
                screen rather than of the form in front of you. */}
            {open.error !== null && (
              <Alert color="red" title="Could not open the game">
                {open.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setOpening(false)}>
                Cancel
              </Button>
              <Button loading={open.pending} onClick={() => void create()}>
                Open
              </Button>
            </Group>
          </Stack>
        </ModalSheet>
      </Stack>
    </Page>
  )
}
