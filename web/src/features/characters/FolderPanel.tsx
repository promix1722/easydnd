import type { DragEvent, ReactNode } from 'react'

import type { Folder } from '@/lib/api'
import { useT } from '@/lib/i18n'
import {
  ACTION_ICON_SIZE,
  Badge,
  Box,
  Button,
  Group,
  IconArrowDown,
  IconArrowUp,
  IconChevronDown,
  IconChevronRight,
  IconDotsVertical,
  IconGripVertical,
  IconPencil,
  IconPlus,
  IconTrash,
  Menu,
  Paper,
  Text,
  UnstyledButton,
} from '@/ui'

export interface FolderPanelProps {
  folder: Folder
  /** How many characters are filed here, drawn beside the name. */
  count: number
  open: boolean
  onToggle: () => void
  /** The folder's table and its add buttons. Not rendered while collapsed. */
  children: ReactNode

  onRename: () => void
  /** Absent on the default folder, which cannot go. */
  onDelete?: (() => void) | undefined
  /** Absent at the ends of the run, and on the default folder, which cannot move. */
  onMoveUp?: (() => void) | undefined
  onMoveDown?: (() => void) | undefined

  /** Absent on the default folder: it leads the listing and is not draggable. */
  onDragStart?: (() => void) | undefined
  onDragOver?: ((event: DragEvent) => void) | undefined
  onDrop?: (() => void) | undefined
  onDragEnd?: (() => void) | undefined
  /** Draws the line marking the gap directly above this folder. */
  dropTarget?: boolean
}

/**
 * One folder: a bordered card holding its own characters and its own way of
 * adding to them.
 *
 * A folder used to be something you *switched to* -- a `Select` above one
 * table, with a Folder column so you could tell the rows apart once you had
 * switched back to all of them. It is the structure of the page now, which is
 * what retired both: with a folder drawn round its characters there is nothing
 * to filter and nothing for the column to say that the heading above it does
 * not.
 *
 * The card owns no data and fetches nothing. Every button here reports upward,
 * because the order of the folders and the reload after a change are facts
 * about the list rather than about any one folder in it.
 */
export function FolderPanel({
  folder,
  count,
  open,
  onToggle,
  children,
  onRename,
  onDelete,
  onMoveUp,
  onMoveDown,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  dropTarget = false,
}: FolderPanelProps) {
  const t = useT()
  const panelId = `folder-${folder.id}`
  const draggable = onDragStart !== undefined

  return (
    <Box>
      {/*
        The gap above this folder, drawn in the brand colour while something
        would land in it.

        Reserved rather than inserted: a 2px row that appears only while
        something is over it shifts every card below it by 2px on each
        crossing, which turns a steady drag into a flicker. It is always there
        and only sometimes coloured. The raw variable is the same one
        `ui/BlockList` reaches for, and for the same reason -- there is no
        Mantine prop for "the filled primary colour" on a box.
      */}
      <Box
        h={2}
        mb={4}
        bg={dropTarget ? 'var(--mantine-primary-color-filled)' : 'transparent'}
        aria-hidden
      />

      <Paper
        withBorder
        radius="md"
        onDragOver={onDragOver}
        onDrop={
          onDrop
            ? (event) => {
                event.preventDefault()
                onDrop()
              }
            : undefined
        }
      >
        {/*
          The header is what drags, with the grip as the affordance -- the
          whole row rather than the grip alone, because a native drag needs a
          `draggable` element and one 16px glyph is a target nobody can hit on
          the first try.

          The default folder gets neither: it leads the listing whatever
          anybody drags, so a grip on it would be an affordance for something
          that cannot happen.
        */}
        <Group
          gap="xs"
          wrap="nowrap"
          p="xs"
          draggable={draggable}
          onDragStart={onDragStart}
          onDragEnd={onDragEnd}
          style={draggable ? { cursor: 'grab' } : undefined}
        >
          {draggable && (
            <IconGripVertical size={ACTION_ICON_SIZE} color="var(--mantine-color-dimmed)" aria-hidden />
          )}

          {/*
            Chevron, name and count are one button, the way an accordion's
            control is: the name is the biggest thing on the row and pressing
            it should do the obvious thing. The grip and the menu sit outside
            it, because a button inside a button is not markup a browser or a
            screen reader can make sense of.
          */}
          <UnstyledButton
            onClick={onToggle}
            aria-expanded={open}
            aria-controls={panelId}
            aria-label={
              open
                ? t('folders.collapse', { name: folder.name })
                : t('folders.expand', { name: folder.name })
            }
            style={{ flex: 1, minWidth: 0 }}
          >
            <Group gap="xs" wrap="nowrap">
              {open ? (
                <IconChevronDown size={ACTION_ICON_SIZE} aria-hidden />
              ) : (
                <IconChevronRight size={ACTION_ICON_SIZE} aria-hidden />
              )}
              <Text fw={650} truncate>
                {folder.name}
              </Text>
              {/* The count is the one thing a collapsed folder still says
                  about its contents, so it is drawn whether it is open or
                  not. */}
              <Text size="xs" c="dimmed">
                {count}
              </Text>
              {folder.default && (
                <Badge size="sm" variant="light">
                  {t('folders.default')}
                </Badge>
              )}
            </Group>
          </UnstyledButton>

          {/*
            Four actions folded into a menu, which is the one case `@/ui`
            blesses one for: four buttons in a row is what `DataList`'s mobile
            card rendering already cannot lay out, and this row is narrower
            than that one. Move up and Move down are in it for a second reason
            -- they are the whole of reordering on a phone, where a native
            drag never fires, and the only path a test can press.
          */}
          <Menu position="bottom-end" withinPortal>
            <Menu.Target>
              <Button
                variant="subtle"
                aria-label={t('list.actions', { name: folder.name })}
                px={6}
              >
                <IconDotsVertical size={ACTION_ICON_SIZE} />
              </Button>
            </Menu.Target>
            <Menu.Dropdown>
              {onMoveUp && (
                <Menu.Item leftSection={<IconArrowUp size={ACTION_ICON_SIZE} />} onClick={onMoveUp}>
                  {t('folders.moveUp')}
                </Menu.Item>
              )}
              {onMoveDown && (
                <Menu.Item
                  leftSection={<IconArrowDown size={ACTION_ICON_SIZE} />}
                  onClick={onMoveDown}
                >
                  {t('folders.moveDown')}
                </Menu.Item>
              )}
              <Menu.Item leftSection={<IconPencil size={ACTION_ICON_SIZE} />} onClick={onRename}>
                {t('common.rename')}
              </Menu.Item>
              {/* The default folder has no delete control: it is the one an
                  account is guaranteed to have. */}
              {onDelete && (
                <Menu.Item
                  color="red"
                  leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
                  onClick={onDelete}
                >
                  {t('common.delete')}
                </Menu.Item>
              )}
            </Menu.Dropdown>
          </Menu>
        </Group>

        {/* Unmounted while collapsed rather than hidden, the way
            `ui/BlockList` does it: a table nobody can see is still a table
            React has to keep in step with its data. */}
        {open && (
          <Box id={panelId} px="xs" pb="xs">
            {children}
          </Box>
        )}
      </Paper>
    </Box>
  )
}

/**
 * The two ways of adding a character, under the folder they will land in.
 *
 * Under the table and on the left, like every other way of adding a row -- but
 * once per folder now rather than once per page, which is what makes the
 * `?folder=` on each of them a fact about where you pressed rather than about
 * what a filter happened to be set to.
 *
 * Both carry the folder's name as their accessible name. Three folders each
 * with a button reading "New character" is exactly the ambiguity the row-action
 * rule already forbids a column of "Delete"s.
 */
export function FolderAdditions({
  folder,
  onNew,
  onImport,
  children,
}: {
  folder: Folder
  onNew: () => void
  onImport: () => void
  /** The development-only stub button, or nothing. */
  children?: ReactNode
}) {
  const t = useT()

  return (
    <Group gap="xs" mt="md">
      <Button
        variant="light"
        leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
        aria-label={t('folders.newCharacterIn', { name: folder.name })}
        onClick={onNew}
      >
        {t('characters.newCharacter')}
      </Button>
      <Button
        variant="default"
        aria-label={t('folders.importInto', { name: folder.name })}
        onClick={onImport}
      >
        {t('characters.import')}
      </Button>
      {children}
    </Group>
  )
}
