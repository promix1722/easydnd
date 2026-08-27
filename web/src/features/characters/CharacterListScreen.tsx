import { useState } from 'react'
import { Link, useNavigate } from 'react-router'

import { classLine } from '@/domain'
import {
  copyCharacter,
  createFolder,
  deleteCharacter,
  deleteFolder,
  listCharacters,
  listFolders,
  moveCharacter,
  renameFolder,
  reorderFolders,
  fieldMessage,
} from '@/lib/api'
import type { Folder, Summary } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  ACTION_SIZE,
  Alert,
  Anchor,
  Box,
  Button,
  DataList,
  Group,
  IconCopy,
  IconFolder,
  IconFolderPlus,
  IconTrash,
  ModalSheet,
  Page,
  pageState,
  Select,
  Stack,
  Text,
  TextInput,
} from '@/ui'

import { FolderAdditions, FolderPanel } from './FolderPanel'
import { StubButton } from './StubButton'

/**
 * The characters, filed into folders.
 *
 * A folder is one account's private filing -- not a group of players. Nothing
 * here is shared with anybody, which is why the screen has no owner column and
 * no permissions anywhere on it.
 *
 * **A folder is the structure of this page rather than a filter on it.** It
 * used to be a `Select` reading "All characters" above one table, with a Folder
 * column so the rows could be told apart once you switched back to all of them,
 * and one pair of add buttons for the whole screen. That made a folder
 * something you switched between: two of them could never be on screen at once,
 * and adding a character meant setting the filter first so the `?folder=` would
 * ride along. Every folder is now drawn as its own card over its own table with
 * its own way of adding to it, which retired the select, the Manage folders
 * dialog and the column together -- not by hiding them, but by leaving them
 * nothing to do.
 *
 * This screen holds two resources rather than one, which is the case
 * docs/web.md left open when it said to revisit the no-query-library decision
 * if this list ever rendered six things. It still does not need one: the two
 * reads are independent, every mutation here is followed by an explicit reload
 * of exactly the lists that changed, and there is no cache to go stale in
 * between.
 */
export function CharacterListScreen() {
  const t = useT()
  const navigate = useNavigate()

  const folders = useResource('folders', (signal) => listFolders(signal))
  // Every character, once, grouped below by the `folder` each one carries --
  // rather than a request per folder. The same call the game screen's folder
  // tree makes, and for the same reason: a character listing already says
  // which shelf it is on, so one request covers every shelf and a request per
  // shelf would be slower for no benefit.
  const characters = useResource('characters', (signal) => listCharacters(undefined, signal))

  const folderList = folders.data?.folders ?? []
  const rows = characters.data?.characters ?? []

  const reloadAll = () => {
    folders.reload()
    characters.reload()
  }

  /*
   * Both resources drive the page's state now, and the folders one wins.
   *
   * The screen used to draw the characters whatever the folders did, with a
   * second inline alert for the folder request -- on the grounds that a filter
   * that would not load is no reason to refuse to draw the rows it was going to
   * narrow. That was right while a folder was a filter. It is not right now: a
   * character carries a folder *id*, so without the folder listing there are no
   * names to head the cards with and no cards to put the rows in. There is no
   * half-drawn version of this page worth showing.
   */
  const foldersState = pageState(folders, {
    title: t('characters.foldersLoadFailed'),
    fallback: t('error.unknown'),
    onRetry: folders.reload,
  })
  const charactersState = pageState(characters, {
    title: t('characters.loadFailed'),
    fallback: t('error.unknown'),
    onRetry: characters.reload,
  })

  // Collapsed rather than open is what is tracked, so a folder made while the
  // page is up arrives expanded like every other one instead of having to be
  // added to a list of the visible.
  const [collapsed, setCollapsed] = useState<readonly string[]>([])
  const toggle = (id: string) =>
    setCollapsed((current) =>
      current.includes(id) ? current.filter((each) => each !== id) : [...current, id],
    )

  const [renaming, setRenaming] = useState<Folder | null>(null)
  const [doomed, setDoomed] = useState<Folder | null>(null)
  const [adding, setAdding] = useState(false)

  const reorder = useAction(reorderFolders)

  // The folders that can move, in the order they are drawn. The default is not
  // among them: it leads the listing, and the server refuses an order naming
  // it.
  const movable = folderList.filter((folder) => !folder.default)

  /** Sends a whole new order, then reloads. There is no optimistic list. */
  const commit = (ids: readonly string[]) => {
    void reorder.run(ids).then((done) => {
      if (done !== null) folders.reload()
    })
  }

  /** Moves the folder at `from` so that it sits at `to`. */
  const moveTo = (from: number, to: number) => {
    if (from === to) return
    const ids = movable.map((folder) => folder.id)
    const [moved] = ids.splice(from, 1)
    if (moved === undefined) return
    ids.splice(to, 0, moved)
    commit(ids)
  }

  /*
   * Dragging, hand-rolled over the native events -- the same shape as the
   * ability-score assignment, which is this app's other drag and was written
   * this way for the same two reasons. There is no drag library below `@/ui`
   * and adding one for two screens is not a trade worth making, and a native
   * drag fires on neither a touchscreen nor jsdom, so it can never be the only
   * way to do something. Move up and Move down in each folder's menu are the
   * real path; this is the shortcut for a mouse.
   */
  const [dragging, setDragging] = useState<number | null>(null)
  /**
   * Where the line is drawn: a gap between folders, 0 to `movable.length`, not
   * a folder's own index.
   *
   * A gap rather than a folder is the whole of the off-by-one this kind of list
   * is famous for. Dragging the first folder onto the second does not put it
   * *where* the second is -- it puts it after it -- so a line drawn above
   * whatever the pointer is over promises the opposite of what the drop does.
   * `lineOver` turns "the pointer is on this folder" into "the folder would
   * land in this gap", and `drop` turns the gap back into an index.
   */
  const [over, setOver] = useState<number | null>(null)

  /** The gap a drop on the folder at `at` would land in. */
  const lineOver = (at: number) => (dragging !== null && dragging < at ? at + 1 : at)

  const endDrag = () => {
    setDragging(null)
    setOver(null)
  }

  const drop = (at: number) => {
    if (dragging !== null) {
      const line = lineOver(at)
      // Closing the gap the folder came out of shifts every later index down
      // by one, which is what this subtraction is. It reduces to `at` in both
      // directions -- and that is the point of writing it out rather than
      // writing `at`: the line and the landing are one rule now, instead of
      // two that were supposed to agree and did not.
      moveTo(dragging, line > dragging ? line - 1 : line)
    }
    endDrag()
  }

  // Written only by StubButton, which exists only in a development build. The
  // state is unconditional because it is a hook; what it costs a production
  // bundle is one string nothing ever sets.
  const [stubError, setStubError] = useState<string | null>(null)

  return (
    <Page trail={[]} state={foldersState.kind !== 'ready' ? foldersState : charactersState}>
      <Stack gap="md">
        {import.meta.env.DEV && stubError !== null && (
          <Alert color="red" title={t('characters.stubFailed')}>
            <Text size="sm">{stubError}</Text>
          </Alert>
        )}

        {reorder.error !== null && (
          <Alert color="red" title={t('characters.reorderFailed')}>
            <Text size="sm">{reorder.error}</Text>
          </Alert>
        )}

        <Stack gap={0}>
          {folderList.map((folder) => {
            const inFolder = rows.filter((character) => character.folder === folder.id)
            // Its index among the folders that can move, which is what the
            // drag and the menu both work in. -1 for the default.
            const at = movable.findIndex((each) => each.id === folder.id)

            return (
              <FolderPanel
                key={folder.id}
                folder={folder}
                count={inFolder.length}
                open={!collapsed.includes(folder.id)}
                onToggle={() => toggle(folder.id)}
                onRename={() => setRenaming(folder)}
                onDelete={folder.default ? undefined : () => setDoomed(folder)}
                onMoveUp={at > 0 ? () => moveTo(at, at - 1) : undefined}
                onMoveDown={
                  at >= 0 && at < movable.length - 1 ? () => moveTo(at, at + 1) : undefined
                }
                {...(folder.default
                  ? {}
                  : {
                      onDragStart: () => setDragging(at),
                      onDragOver: (event) => {
                        event.preventDefault()
                        setOver(lineOver(at))
                      },
                      onDrop: () => drop(at),
                      onDragEnd: endDrag,
                      dropTarget: dragging !== null && over === at,
                    })}
              >
                <DataList
                  items={inFolder}
                  getKey={(character) => character.id}
                  columns={[
                    {
                      key: 'name',
                      header: t('common.name'),
                      primary: true,
                      render: (character) => (
                        <Anchor component={Link} to={`/characters/${character.id}`}>
                          {character.name || 'Unnamed'}
                        </Anchor>
                      ),
                    },
                    {
                      key: 'level',
                      header: t('game.level'),
                      render: (character) => character.level || '--',
                    },
                    {
                      key: 'classes',
                      header: t('characters.classes'),
                      render: (character) => classLine(character.classes),
                    },
                    // No Folder column. The card this table sits in is the
                    // folder, so the column would write the same word down
                    // every row of it.
                    {
                      key: 'actions',
                      header: t('characters.actions'),
                      render: (character) => (
                        <RowActions
                          character={character}
                          folders={folderList}
                          onChanged={reloadAll}
                        />
                      ),
                    },
                  ]}
                  empty={t('characters.folderEmpty')}
                />

                <FolderAdditions
                  folder={folder}
                  onNew={() => void navigate(`/characters/new?folder=${folder.id}`)}
                  onImport={() => void navigate(`/characters/import?folder=${folder.id}`)}
                >
                  {/* Development only, and absent from a production bundle
                      rather than hidden in one: Vite replaces
                      import.meta.env.DEV with a literal, so this folds away
                      and StubButton is eliminated with it. The route it would
                      call is not registered in production either. */}
                  {import.meta.env.DEV && (
                    <StubButton folder={folder} onFailed={setStubError} />
                  )}
                </FolderAdditions>
              </FolderPanel>
            )
          })}

          {/* The last gap has no folder under it to carry a line, so it gets
              its own. Always drawn and only sometimes coloured, like the ones
              above it, so that a drag past the end shifts nothing. */}
          <Box
            h={2}
            mt={4}
            bg={
              dragging !== null && over === movable.length
                ? 'var(--mantine-primary-color-filled)'
                : 'transparent'
            }
            aria-hidden
          />
        </Stack>

        {/* Under all of them, on the left -- the same rule the add buttons
            inside each folder follow, applied one level up. This one adds to
            the list of folders, so it sits below the list of folders. */}
        <Group gap="xs">
          <Button
            size={ACTION_SIZE}
            variant="default"
            leftSection={<IconFolderPlus size={ACTION_ICON_SIZE} />}
            onClick={() => setAdding(true)}
          >
            {t('characters.newFolder')}
          </Button>
        </Group>
      </Stack>

      <NewFolder opened={adding} onClose={() => setAdding(false)} onMade={reloadAll} />
      <RenameFolder folder={renaming} onClose={() => setRenaming(null)} onDone={reloadAll} />
      <DeleteFolder
        folder={doomed}
        count={doomed === null ? 0 : rows.filter((c) => c.folder === doomed.id).length}
        onClose={() => setDoomed(null)}
        onDone={reloadAll}
      />
    </Page>
  )
}

/** Move, copy and delete for one row. */
function RowActions({
  character,
  folders,
  onChanged,
}: {
  character: Summary
  folders: Folder[]
  onChanged: () => void
}) {
  const t = useT()
  const [moving, setMoving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [target, setTarget] = useState(character.folder)

  const move = useAction(moveCharacter)
  const copy = useAction(copyCharacter)
  const remove = useAction(deleteCharacter)

  const label = character.name || t('characters.thisCharacter')

  return (
    <>
      {/* Spelled out rather than folded into a menu, so a row's actions read
          the same way every other table's do -- and so that what you can do to
          a character is visible without opening anything.
          Each carries the row's name as its accessible name: a table of these
          is otherwise a column of buttons all called "Delete", which is
          ambiguous to a screen reader and to a test alike. */}
      <Group gap="xs" wrap="nowrap">
        <Button
          size={ACTION_SIZE}
          variant="subtle"
          leftSection={<IconFolder size={ACTION_ICON_SIZE} />}
          aria-label={t('characters.moveNamed', { name: label })}
          onClick={() => {
            setTarget(character.folder)
            setMoving(true)
          }}
        >
          {t('characters.move')}
        </Button>
        <Button
          size={ACTION_SIZE}
          variant="subtle"
          leftSection={<IconCopy size={ACTION_ICON_SIZE} />}
          aria-label={t('characters.copyNamed', { name: label })}
          onClick={() => {
            void copy.run(character.id).then((made) => {
              if (made !== null) onChanged()
            })
          }}
        >
          {t('characters.copy')}
        </Button>
        <Button
          size={ACTION_SIZE}
          variant="subtle"
          color="red"
          leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
          aria-label={t('characters.deleteNamed', { name: label })}
          onClick={() => setDeleting(true)}
        >
          {t('common.delete')}
        </Button>
      </Group>

      <ModalSheet opened={moving} onClose={() => setMoving(false)} title={t('characters.moveNamed', { name: label })}>
        <Stack gap="md">
          <Select
            label={t('characters.folder')}
            data={folders.map((f) => ({ value: f.id, label: f.name }))}
            value={target}
            onChange={(value) => setTarget(value ?? character.folder)}
            allowDeselect={false}
          />
          {move.error !== null && <Alert color="red">{move.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setMoving(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              loading={move.pending}
              disabled={target === character.folder}
              onClick={() => {
                void move.run(character.id, target).then((ok) => {
                  if (ok !== null) {
                    setMoving(false)
                    onChanged()
                  }
                })
              }}
            >
              {t('characters.move')}
            </Button>
          </Group>
        </Stack>
      </ModalSheet>

      <ModalSheet opened={deleting} onClose={() => setDeleting(false)} title={t('characters.deleteNamedTitle', { name: label })}>
        <Stack gap="md">
          <Text size="sm">
            {t('characters.deleteWarning')}
          </Text>
          {remove.error !== null && <Alert color="red">{remove.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setDeleting(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              color="red"
              loading={remove.pending}
              onClick={() => {
                void remove.run(character.id).then((ok) => {
                  if (ok !== null) {
                    setDeleting(false)
                    onChanged()
                  }
                })
              }}
            >
              {t('common.delete')}
            </Button>
          </Group>
        </Stack>
      </ModalSheet>
    </>
  )
}

/** The New folder button's dialog. */
function NewFolder({
  opened,
  onClose,
  onMade,
}: {
  opened: boolean
  onClose: () => void
  onMade: () => void
}) {
  const t = useT()
  const [name, setName] = useState('')
  const create = useAction(createFolder)

  const submit = () => {
    if (name.trim() === '' || create.pending) return
    void create.run(name.trim()).then((made) => {
      if (made !== null) {
        setName('')
        onClose()
        onMade()
      }
    })
  }

  return (
    <ModalSheet opened={opened} onClose={onClose} title={t('characters.newFolder')}>
      {/* A real form, so the phone's keyboard offers a Go key that works. The
          alternative -- an Enter handler on the input, which
          `features/character/NameForm` still uses -- is what a desktop browser
          needs and what a soft keyboard's action key is not obliged to send.
          One `onSubmit` also means the button and the key press cannot drift
          apart, which is the bug that gets shipped when they are two handlers. */}
      <form
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
      >
        <Stack gap="md">
          <Text c="dimmed" size="sm">
            {t('characters.folderExplained')}
          </Text>
          <TextInput
            label={t('common.name')}
            placeholder={t('characters.folderPlaceholder')}
            value={name}
            onChange={(event) => setName(event.currentTarget.value)}
            error={fieldMessage(t, create.fields, 'name')}
          />
          {create.error !== null && <Alert color="red">{create.error}</Alert>}
          <Group justify="flex-end">
            {/* Mantine's Button is type="button" unless told otherwise, so
                Cancel cannot submit by sitting inside a form. */}
            <Button variant="subtle" onClick={onClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" loading={create.pending} disabled={name.trim() === ''}>
              {t('common.add')}
            </Button>
          </Group>
        </Stack>
      </form>
    </ModalSheet>
  )
}

/** Renaming one folder, the default one included. */
function RenameFolder({
  folder,
  onClose,
  onDone,
}: {
  folder: Folder | null
  onClose: () => void
  onDone: () => void
}) {
  const t = useT()
  const [name, setName] = useState('')
  const rename = useAction(renameFolder)

  // Seeded from the folder rather than held in step with it: the dialog is
  // mounted once and opened many times, so the name it starts with is decided
  // when it opens.
  const [seeded, setSeeded] = useState<string | null>(null)
  if (folder !== null && seeded !== folder.id) {
    setSeeded(folder.id)
    setName(folder.name)
  }

  const submit = () => {
    if (folder === null || name.trim() === '' || rename.pending) return
    void rename.run(folder.id, name.trim()).then((done) => {
      if (done !== null) {
        onClose()
        onDone()
      }
    })
  }

  return (
    <ModalSheet opened={folder !== null} onClose={onClose} title={t('characters.renameFolder')}>
      {/* A form for the same reason the New folder dialog is one: the key a
          soft keyboard offers has to do something. */}
      <form
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
      >
        <Stack gap="md">
          <TextInput
            label={t('common.name')}
            value={name}
            onChange={(event) => setName(event.currentTarget.value)}
            error={fieldMessage(t, rename.fields, 'name')}
          />
          {rename.error !== null && <Alert color="red">{rename.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={onClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" loading={rename.pending} disabled={name.trim() === ''}>
              {t('common.rename')}
            </Button>
          </Group>
        </Stack>
      </form>
    </ModalSheet>
  )
}

/**
 * Deleting a folder, and saying what that costs.
 *
 * `count` is what makes the confirmation honest: the server deletes the
 * characters in a folder along with it, so a dialog that only named the folder
 * would be describing a smaller action than the one about to happen. It used to
 * cost a third request; the page now holds every character already, so it is a
 * filter over what is on screen.
 */
function DeleteFolder({
  folder,
  count,
  onClose,
  onDone,
}: {
  folder: Folder | null
  count: number
  onClose: () => void
  onDone: () => void
}) {
  const t = useT()
  const remove = useAction(deleteFolder)

  return (
    <ModalSheet
      opened={folder !== null}
      onClose={onClose}
      title={folder === null ? '' : t('characters.deleteFolderTitle', { name: folder.name })}
    >
      <Stack gap="md">
        {/* The count is the point of this dialog. Deleting a folder takes its
            characters with it, and characters are held in memory -- so there is
            no undo and not even a backup to go back to. */}
        <Text size="sm">
          {count === 0
            ? t('characters.deleteFolderEmpty')
            : t('characters.deleteFolderWarning', { count })}
        </Text>
        {remove.error !== null && <Alert color="red">{remove.error}</Alert>}
        <Group justify="flex-end">
          <Button variant="subtle" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            color="red"
            loading={remove.pending}
            onClick={() => {
              if (folder === null) return
              void remove.run(folder.id).then((done) => {
                if (done !== null) {
                  onClose()
                  onDone()
                }
              })
            }}
          >
            {count === 0 ? t('characters.deleteFolder') : t('characters.deleteFolderAnd', { count })}
          </Button>
        </Group>
      </Stack>
    </ModalSheet>
  )
}
