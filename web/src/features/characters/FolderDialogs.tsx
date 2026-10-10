import { useState } from 'react'

import { createFolder, deleteFolder, renameFolder, fieldMessage } from '@/lib/api'
import type { Folder } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { Alert, Button, Group, ModalSheet, Stack, Text, TextInput } from '@/ui'

/** The New folder button's dialog. */
export function NewFolder({
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
    // `onSubmit` on the sheet rather than a `<form>` written out here. This
    // dialog and its sibling below were the only two in the app that had one,
    // which is why they were the only two whose Go key worked; `ui/ModalSheet`
    // now wraps every dialog that asks for it, so the eight that were missing
    // it cannot drift back.
    <ModalSheet opened={opened} onClose={onClose} title={t('characters.newFolder')} onSubmit={submit}>
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
    </ModalSheet>
  )
}

/** Renaming one folder, the default one included. */
export function RenameFolder({
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
    <ModalSheet
      opened={folder !== null}
      onClose={onClose}
      title={t('characters.renameFolder')}
      onSubmit={submit}
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
export function DeleteFolder({
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
