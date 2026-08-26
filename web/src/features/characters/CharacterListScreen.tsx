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
} from '@/lib/api'
import type { Folder, Summary } from '@/lib/api'
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
  IconCopy,
  IconFolder,
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

import { StubButton } from './StubButton'

/** The sentinel the folder filter uses for "don't narrow". */
const ALL = 'all'

/**
 * The characters, filed into folders.
 *
 * A folder is one account's private filing -- not a group of players. Nothing
 * here is shared with anybody, which is why there is no owner column and no
 * permissions anywhere on the screen.
 *
 * This screen holds two resources rather than one, which is the case
 * docs/web.md left open when it said to revisit the no-query-library decision
 * if this list ever rendered six things. It still does not need one: the
 * two reads are independent, every mutation here is followed by an explicit
 * reload of exactly the lists that changed, and there is no cache to go stale
 * in between.
 */
export function CharacterListScreen() {
  const navigate = useNavigate()
  const [selected, setSelected] = useState(ALL)

  const folders = useResource('folders', (signal) => listFolders(signal))
  // The key carries the filter, so changing it aborts the request in flight
  // and starts the right one -- rather than leaving the previous folder's
  // characters on screen under the new heading.
  const characters = useResource(`characters:${selected}`, (signal) =>
    listCharacters(selected === ALL ? undefined : selected, signal),
  )

  const folderList = folders.data?.folders ?? []
  const rows = characters.data?.characters ?? []
  const activeFolder = selected === ALL ? undefined : selected

  const reloadAll = () => {
    folders.reload()
    characters.reload()
  }

  const nameOf = (id: string) => folderList.find((f) => f.id === id)?.name ?? 'Unknown folder'

  // Written only by StubButton, which exists only in a development build. The
  // state is unconditional because it is a hook; what it costs a production
  // bundle is one string nothing ever sets.
  const [stubError, setStubError] = useState<string | null>(null)

  return (
    <Page
      trail={[]}
      state={pageState(characters, {
        title: 'Could not load your characters',
        fallback: 'Unknown error',
        onRetry: characters.reload,
      })}
      /*
       * The folder filter is this section's, and no other section has one.
       *
       * It sits above the table rather than on the heading line because it
       * narrows the rows rather than acting on the section, and it survived
       * the unification for that reason: a control doing real work here and
       * nowhere else is not drift. The `Group` that used to wrap the title
       * alone did not survive -- it held one child and justified nothing.
       */
      filters={
        <Group gap="xs" align="flex-end">
          <Select
            label="Folder"
            data={[
              { value: ALL, label: 'All characters' },
              ...folderList.map((f) => ({ value: f.id, label: f.name })),
            ]}
            value={selected}
            onChange={(value) => setSelected(value ?? ALL)}
            allowDeselect={false}
            w={220}
          />
          <ManageFolders folders={folderList} onChanged={reloadAll} />
        </Group>
      }
    >
      <Stack gap="md">
        {/* The folders resource fails on its own terms, and inline.
            Deliberately not folded into the page's state, which the characters
            resource drives: folders are a filter, and a filter that would not
            load is no reason to refuse to draw the characters it was going to
            narrow. This screen is the only one holding two resources, and this
            is the case that makes it worth the extra alert. */}
        {folders.error !== null && (
          <Alert color="red" title="Could not load your folders">
            <Stack gap="xs" align="flex-start">
              <Text size="sm">{folders.error}</Text>
              <Button variant="light" onClick={folders.reload}>
                Try again
              </Button>
            </Stack>
          </Alert>
        )}

        {import.meta.env.DEV && stubError !== null && (
          <Alert color="red" title="Could not create the stub character">
            <Text size="sm">{stubError}</Text>
          </Alert>
        )}

          <DataList
            items={rows}
            getKey={(character) => character.id}
            columns={[
              {
                key: 'name',
                header: 'Name',
                primary: true,
                render: (character) => (
                  <Anchor component={Link} to={`/characters/${character.id}`}>
                    {character.name || 'Unnamed'}
                  </Anchor>
                ),
              },
              { key: 'level', header: 'Level', render: (character) => character.level || '--' },
              {
                key: 'classes',
                header: 'Classes',
                render: (character) => classLine(character.classes),
              },
              // Only worth a column when the listing spans folders; inside one
              // folder every row would say the same thing.
              ...(selected === ALL
                ? [
                    {
                      key: 'folder',
                      header: 'Folder',
                      render: (character: Summary) => (
                        <Badge variant="light">{nameOf(character.folder)}</Badge>
                      ),
                    },
                  ]
                : []),
              {
                key: 'actions',
                header: 'Actions',
                render: (character) => (
                  <RowActions
                    character={character}
                    folders={folderList}
                    onChanged={reloadAll}
                  />
                ),
              },
            ]}
            empty={
              selected === ALL
                ? 'No characters yet. Make one.'
                : 'Nothing in this folder yet. Make one, or move one here.'
            }
          />

        {/* Under the table, on the left, like every other way of adding a row. */}
        <Group gap="xs">
          <Button
            size={ACTION_SIZE}
            variant="light"
            leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
            onClick={() =>
              void navigate(
                activeFolder ? `/characters/new?folder=${activeFolder}` : '/characters/new',
              )
            }
          >
            New character
          </Button>
          <Button
            size={ACTION_SIZE}
            variant="default"
            onClick={() =>
              void navigate(
                activeFolder ? `/characters/import?folder=${activeFolder}` : '/characters/import',
              )
            }
          >
            Import
          </Button>
          {/* Development only, and absent from a production bundle rather than
              hidden in one: Vite replaces import.meta.env.DEV with a literal, so
              this folds away and StubButton is eliminated with it. The route it
              would call is not registered in production either. */}
          {import.meta.env.DEV && <StubButton folder={activeFolder} onFailed={setStubError} />}
        </Group>
      </Stack>
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
  const [moving, setMoving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [target, setTarget] = useState(character.folder)

  const move = useAction(moveCharacter)
  const copy = useAction(copyCharacter)
  const remove = useAction(deleteCharacter)

  const label = character.name || 'this character'

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
          aria-label={`Move ${label}`}
          onClick={() => {
            setTarget(character.folder)
            setMoving(true)
          }}
        >
          Move
        </Button>
        <Button
          size={ACTION_SIZE}
          variant="subtle"
          leftSection={<IconCopy size={ACTION_ICON_SIZE} />}
          aria-label={`Copy ${label}`}
          onClick={() => {
            void copy.run(character.id).then((made) => {
              if (made !== null) onChanged()
            })
          }}
        >
          Copy
        </Button>
        <Button
          size={ACTION_SIZE}
          variant="subtle"
          color="red"
          leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
          aria-label={`Delete ${label}`}
          onClick={() => setDeleting(true)}
        >
          Delete
        </Button>
      </Group>

      <ModalSheet opened={moving} onClose={() => setMoving(false)} title={`Move ${label}`}>
        <Stack gap="md">
          <Select
            label="Folder"
            data={folders.map((f) => ({ value: f.id, label: f.name }))}
            value={target}
            onChange={(value) => setTarget(value ?? character.folder)}
            allowDeselect={false}
          />
          {move.error !== null && <Alert color="red">{move.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setMoving(false)}>
              Cancel
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
              Move
            </Button>
          </Group>
        </Stack>
      </ModalSheet>

      <ModalSheet opened={deleting} onClose={() => setDeleting(false)} title={`Delete ${label}?`}>
        <Stack gap="md">
          <Text size="sm">
            This deletes the character and their whole log. It cannot be undone.
          </Text>
          {remove.error !== null && <Alert color="red">{remove.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setDeleting(false)}>
              Cancel
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
              Delete
            </Button>
          </Group>
        </Stack>
      </ModalSheet>
    </>
  )
}

/**
 * Create, rename and delete folders.
 *
 * `count` is what makes the delete confirmation honest: the server deletes the
 * characters in a folder along with it, so a dialog that only named the folder
 * would be describing a smaller action than the one about to happen.
 */
function ManageFolders({ folders, onChanged }: { folders: Folder[]; onChanged: () => void }) {
  const [open, setOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [renaming, setRenaming] = useState<Folder | null>(null)
  const [renameTo, setRenameTo] = useState('')
  const [doomed, setDoomed] = useState<Folder | null>(null)

  const create = useAction(createFolder)
  const rename = useAction(renameFolder)
  const remove = useAction(deleteFolder)

  // Counting from the unfiltered listing would need a third request; this is
  // the one number the dialog needs, so it fetches only while a folder is
  // actually up for deletion.
  const doomedCharacters = useResource(doomed === null ? 'idle' : `doomed:${doomed.id}`, (signal) =>
    doomed === null ? Promise.resolve({ characters: [] }) : listCharacters(doomed.id, signal),
  )
  const count = doomedCharacters.data?.characters.length ?? 0

  return (
    <>
      <Button variant="default" onClick={() => setOpen(true)}>
        Manage folders
      </Button>

      <ModalSheet opened={open} onClose={() => setOpen(false)} title="Folders">
        <Stack gap="md">
          <Text c="dimmed" size="sm">
            Where you file your characters. Only you can see them.
          </Text>

          <Stack gap="xs">
            {folders.map((folder) => (
              <Group key={folder.id} justify="space-between" wrap="nowrap">
                <Group gap="xs" wrap="nowrap">
                  <Text size="sm">{folder.name}</Text>
                  {folder.default && (
                    <Badge size="sm" variant="light">
                      Default
                    </Badge>
                  )}
                </Group>
                <Group gap="xs" wrap="nowrap">
                  <Button
                    size="compact-sm"
                    variant="subtle"
                    onClick={() => {
                      setRenaming(folder)
                      setRenameTo(folder.name)
                    }}
                  >
                    Rename
                  </Button>
                  {/* The default folder has no delete control: it is the one
                      an account is guaranteed to have. */}
                  {!folder.default && (
                    <Button
                      size="compact-sm"
                      variant="subtle"
                      color="red"
                      onClick={() => setDoomed(folder)}
                    >
                      Delete
                    </Button>
                  )}
                </Group>
              </Group>
            ))}
          </Stack>

          {create.error !== null && <Alert color="red">{create.error}</Alert>}
          <Group gap="xs" align="flex-end">
            <TextInput
              label="New folder"
              placeholder="Campaign"
              value={newName}
              onChange={(event) => setNewName(event.currentTarget.value)}
              error={create.fields.find((f) => f.field === 'name')?.message}
              style={{ flex: 1 }}
            />
            <Button
              loading={create.pending}
              disabled={newName.trim() === ''}
              onClick={() => {
                void create.run(newName.trim()).then((made) => {
                  if (made !== null) {
                    setNewName('')
                    onChanged()
                  }
                })
              }}
            >
              Add
            </Button>
          </Group>
        </Stack>
      </ModalSheet>

      <ModalSheet
        opened={renaming !== null}
        onClose={() => setRenaming(null)}
        title="Rename folder"
      >
        <Stack gap="md">
          <TextInput
            label="Name"
            value={renameTo}
            onChange={(event) => setRenameTo(event.currentTarget.value)}
            error={rename.fields.find((f) => f.field === 'name')?.message}
          />
          {rename.error !== null && <Alert color="red">{rename.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setRenaming(null)}>
              Cancel
            </Button>
            <Button
              loading={rename.pending}
              disabled={renameTo.trim() === ''}
              onClick={() => {
                if (renaming === null) return
                void rename.run(renaming.id, renameTo.trim()).then((done) => {
                  if (done !== null) {
                    setRenaming(null)
                    onChanged()
                  }
                })
              }}
            >
              Rename
            </Button>
          </Group>
        </Stack>
      </ModalSheet>

      <ModalSheet
        opened={doomed !== null}
        onClose={() => setDoomed(null)}
        title={doomed === null ? '' : `Delete ${doomed.name}?`}
      >
        <Stack gap="md">
          {/* The count is the point of this dialog. Deleting a folder takes
              its characters with it, and characters are held in memory -- so
              there is no undo and not even a backup to go back to. */}
          <Text size="sm">
            {count === 0
              ? 'This folder is empty. Deleting it cannot be undone.'
              : `This deletes the folder and the ${count} character${
                  count === 1 ? '' : 's'
                } in it, with their logs. It cannot be undone.`}
          </Text>
          {remove.error !== null && <Alert color="red">{remove.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setDoomed(null)}>
              Cancel
            </Button>
            <Button
              color="red"
              loading={remove.pending}
              onClick={() => {
                if (doomed === null) return
                void remove.run(doomed.id).then((done) => {
                  if (done !== null) {
                    setDoomed(null)
                    onChanged()
                  }
                })
              }}
            >
              {count === 0 ? 'Delete folder' : `Delete folder and ${count}`}
            </Button>
          </Group>
        </Stack>
      </ModalSheet>
    </>
  )
}
