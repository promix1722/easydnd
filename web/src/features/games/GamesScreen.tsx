import { useState } from 'react'
import { Link, useNavigate } from 'react-router'

import type { GameSummary, GroupSummary } from '@/lib/api'
import { fieldMessage, createGame, deleteGame, listGames, listGroups, renameGame } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
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
  SHEET_COMBOBOX,
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
  const t = useT()
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
    games.refresh()
  }

  const state = pageState(games, {
    title: t('games.loadFailed'),
    fallback: t('error.unknown'),
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
          actions={(game: GameSummary) =>
            // Each row edits its own game and nothing else. Whether you may is
            // your rank at *that* table, which the row already carries.
            canManage(game.group_id)
              ? [
                  {
                    key: 'rename',
                    label: t('common.rename'),
                    icon: <IconPencil size={ACTION_ICON_SIZE} />,
                    onClick: () => {
                      setNewName(game.name)
                      setRenaming(game)
                    },
                  },
                  {
                    key: 'delete',
                    label: t('common.delete'),
                    color: 'red' as const,
                    icon: <IconTrash size={ACTION_ICON_SIZE} />,
                    onClick: () => setDeleting(game),
                  },
                ]
              : []
          }
          columns={[
            {
              key: 'name',
              header: t('games.game'),
              primary: true,
              text: (game: GameSummary) => game.name,
              to: (game: GameSummary) => `/games/${game.id}`,
              render: (game: GameSummary) => (
                <Anchor component={Link} to={`/games/${game.id}`}>
                  <Text size="sm">{game.name}</Text>
                </Anchor>
              ),
            },
            {
              key: 'group',
              header: t('games.group'),
              // The one meta value in the app that is a link rather than a
              // fact: somebody who plays at three tables tells their Thursdays
              // apart by it, and it is the way back to the group.
              render: (game: GameSummary) => (
                <Anchor component={Link} to={`/groups/${game.group_id}`} size="sm">
                  {game.group_name}
                </Anchor>
              ),
            },
          ]}
          empty={tables.length > 0 ? t('games.emptyForDm') : t('games.empty')}
        />

        {/* Under the table, on the left. */}
        {tables.length > 0 && (
          <Group>
            <Button
              variant="light"
              leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
              onClick={() => {
                setGroup(tables[0]?.id ?? null)
                setOpening(true)
              }}
            >
              {t('games.new')}
            </Button>
          </Group>
        )}

        <ModalSheet
          opened={renaming !== null}
          onClose={() => setRenaming(null)}
          title={t('games.renameTitle')}
          onSubmit={() => {
            const target = renaming
            setRenaming(null)
            if (target !== null) void act(rename.run(target.id, newName))
          }}
        >
          <Stack gap="sm">
            <TextInput
              label={t('common.name')}
              value={newName}
              error={fieldMessage(t, rename.fields, 'name')}
              onChange={(event) => setNewName(event.currentTarget.value)}
              data-autofocus
            />
            {rename.error !== null && (
              <Alert color="red" title={t('games.renameFailed')}>
                {rename.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setRenaming(null)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" loading={rename.pending}>
                {t('common.rename')}
              </Button>
            </Group>
          </Stack>
        </ModalSheet>

        <ModalSheet
          opened={deleting !== null}
          onClose={() => setDeleting(null)}
          title={t('games.deleteTitle')}
        >
          <Stack gap="sm">
            <Text size="sm">
              {t('games.deleteWarning', {
                name: deleting?.name ?? '',
                group: deleting?.group_name ?? '',
              })}
            </Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setDeleting(null)}>
                {t('common.cancel')}
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
                {t('common.delete')}
              </Button>
            </Group>
          </Stack>
        </ModalSheet>

        <ModalSheet
          opened={opening}
          onClose={() => setOpening(false)}
          title={t('games.openTitle')}
          onSubmit={() => void create()}
        >
          <Stack gap="sm">
            <Select
              label={t('games.group')}
              comboboxProps={SHEET_COMBOBOX}
              data={tables.map((g: GroupSummary) => ({ value: g.id, label: g.name }))}
              value={group}
              onChange={setGroup}
              allowDeselect={false}
            />
            <TextInput
              label={t('common.name')}
              placeholder={t('games.namePlaceholder')}
              value={name}
              error={fieldMessage(t, open.fields, 'name')}
              onChange={(event) => setName(event.currentTarget.value)}
              data-autofocus
            />
            {/* Inside the dialog that caused it. Raised to the page it sat
                behind the modal, where it read as a failure of the whole
                screen rather than of the form in front of you. */}
            {open.error !== null && (
              <Alert color="red" title={t('games.openFailed')}>
                {open.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setOpening(false)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" loading={open.pending}>
                {t('games.open')}
              </Button>
            </Group>
          </Stack>
        </ModalSheet>
      </Stack>
    </Page>
  )
}
