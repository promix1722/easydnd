import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import {
  addToGame,
  deleteGame,
  getGame,
  listTable,
  renameGame,
  fieldMessage,
} from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  Alert,
  Badge,
  Button,
  Group,
  IconPencil,
  IconTrash,
  ModalSheet,
  Page,
  Panel,
  Stack,
  TextInput,
  pageState,
} from '@/ui'

import { atLeast, roleLabel } from '../groups/roles'
import { PickCharactersSheet } from './PickCharactersSheet'

import { GameTracker } from './GameTracker'

/** One game: its name, and who is at it. */
export function GameScreen() {
  const t = useT()
  const { id: gameId = '' } = useParams()
  const navigate = useNavigate()
  const { data, error, loading, reload, refresh, refreshError } = useResource(`game:${gameId}`, (signal) =>
    getGame(gameId, signal),
    { pollInterval: 3000, retainOnRefreshError: true },
  )

  const [renaming, setRenaming] = useState(false)
  const [name, setName] = useState('')
  const [picking, setPicking] = useState(false)
  const add = useAction(addToGame)

  // The group's table is one flat list: a game is played at exactly one group,
  // so there is nothing to branch on.
  const table = useResource(
    picking && data !== null ? `table:${data.group_id}` : '',
    (signal) => listTable(data?.group_id ?? '', signal),
  )
  const rename = useAction(renameGame)
  const destroy = useAction(deleteGame)

  const state = pageState(
    { data, error, loading },
    { title: t('game.loadFailed'), fallback: t('game.missing'), onRetry: reload },
  )

  if (state.kind !== 'ready' || data === null) {
    return <Page trail={[{ label: data?.name ?? null }]} state={state} />
  }

  const game = data
  const canManage = atLeast(game.role, 'dm')
  const failure = add.error ?? rename.error ?? destroy.error

  async function act(work: Promise<unknown | null>) {
    if ((await work) === null) return
    refresh()
  }

  // Already-seated characters are not offered again: adding one twice is a
  // no-op the server absorbs, but a list that offers it says otherwise.
  const seated = new Set(game.characters.map((c) => c.id))
  const seatable = (table.data?.characters ?? []).filter((c) => !seated.has(c.id))

  async function close() {
    if ((await destroy.run(gameId)) === null) return
    await navigate('/games', { replace: true })
  }

  /*
   * Two things about this header.
   *
   * The **badge** is your rank, shown the way a group shows it: a game is
   * reached from its own section, so the page has to say what you are at the
   * table it is played at rather than leaving you to remember.
   *
   * The **group is not on this page at all** -- not as a crumb, and no longer
   * as a link beneath the title.
   *
   * A game is played at a table but is not reached through one: games are
   * their own section, which is the whole argument in
   * docs/web/games.md#games-are-a-section-not-a-corner-of-a-group. A trail reading
   * `Groups / Wednesday Night / Thursday night` would say the opposite and
   * would disagree with the navbar, which lights Games. The "Back to the
   * group" link that used to sit under the title was the same claim in a
   * quieter voice -- it named a direction rather than a destination, and the
   * only thing it offered was a way out of a page you had just arrived at.
   * Groups are one press away in the navigation either way.
   */
  return (
    <Page
      trail={[{ label: game.name }]}
      badge={<Badge variant="light">{roleLabel(t, game.role)}</Badge>}
      actions={
        canManage ? (
          <>
            <Button
              variant="subtle"
              leftSection={<IconPencil size={ACTION_ICON_SIZE} />}
              onClick={() => {
                setName(game.name)
                setRenaming(true)
              }}
            >
              {t('common.rename')}
            </Button>
            <Button
              color="red"
              variant="subtle"
              leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
              loading={destroy.pending}
              onClick={() => void close()}
            >
              {t('common.delete')}
            </Button>
          </>
        ) : undefined
      }
    >
      <Panel>
        <Stack gap="md">
          {refreshError !== null && (
            <Alert color="yellow" title={t('game.refreshFailed')}>
              {refreshError}
              <Button variant="subtle" onClick={refresh}>{t('page.retry')}</Button>
            </Alert>
          )}
          {failure !== null && (
            <Alert color="red" title={t('group.actionFailed')}>
              {failure}
            </Alert>
          )}

          <GameTracker game={game} onChange={refresh}
            onAddFromGroup={() => setPicking(true)} />

          <PickCharactersSheet
            key={picking ? 'group' : 'group-closed'}
            opened={picking}
            title={t('game.addFromGroupTitle')}
            description={t('game.pickShared')}
            empty={t('game.nothingShared')}
            characters={seatable}
            loading={table.loading}
            pending={add.pending}
            onClose={() => setPicking(false)}
            onAdd={(ids) => {
              setPicking(false)
              void act(add.run(gameId, ids))
            }}
          />

          <ModalSheet
            opened={renaming}
            onClose={() => setRenaming(false)}
            title={t('games.renameTitle')}
            onSubmit={() => {
              setRenaming(false)
              void act(rename.run(gameId, name))
            }}
          >
            <Stack gap="sm">
              <TextInput
                label={t('common.name')}
                value={name}
                error={fieldMessage(t, rename.fields, 'name')}
                onChange={(event) => setName(event.currentTarget.value)}
              />
              <Group justify="flex-end">
                <Button variant="default" onClick={() => setRenaming(false)}>
                  {t('common.cancel')}
                </Button>
                <Button type="submit" loading={rename.pending}>
                  {t('common.rename')}
                </Button>
              </Group>
            </Stack>
          </ModalSheet>
        </Stack>
      </Panel>
    </Page>
  )
}
