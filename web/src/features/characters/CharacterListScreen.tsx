import { useState } from 'react'
import { useNavigate } from 'react-router'

import { listCharacters, listFolders } from '@/lib/api'
import type { Folder } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  Alert,
  Box,
  Button,
  Group,
  IconFolderPlus,
  Page,
  pageState,
  Stack,
  Text,
} from '@/ui'

import { CharacterListTable } from './CharacterListTable'
import { DeleteFolder, NewFolder, RenameFolder } from './FolderDialogs'
import { FolderAdditions, FolderPanel } from './FolderPanel'
import { useCharacterActions } from './useCharacterActions'
import { useFolderOrder } from './useFolderOrder'

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
    folders.refresh()
    characters.refresh()
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

  // A row's three actions and the two dialogs two of them open. They live here
  // rather than in the row because `DataList` renders no children -- see the
  // hook.
  const { actionsFor, sheets } = useCharacterActions(folderList, reloadAll)

  const { reorder, movable, moveTo, dragging, setDragging, over, setOver, lineOver, endDrag, drop } =
    useFolderOrder(folderList, folders)

  return (
    <Page trail={[]} state={foldersState.kind !== 'ready' ? foldersState : charactersState}>
      <Stack gap="md">
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
                <CharacterListTable items={inFolder} actions={actionsFor} />

                <FolderAdditions
                  folder={folder}
                  onNew={() => void navigate(`/characters/new?folder=${folder.id}`)}
                >
                  <Button variant="default" onClick={() => void navigate(`/ai-wizard?folder=${encodeURIComponent(folder.id)}`)}>{t('section.aiWizard')}</Button>
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
            variant="default"
            leftSection={<IconFolderPlus size={ACTION_ICON_SIZE} />}
            onClick={() => setAdding(true)}
          >
            {t('characters.newFolder')}
          </Button>
        </Group>
      </Stack>

      {sheets}

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
