import { useState } from 'react'

import { getEvents } from '@/lib/api'
import { deleteCustomOption, upsertCustomOption } from '@/lib/api/characters'
import type { CustomOption } from '@/lib/api/characters'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { Button, Group, ModalSheet, Paper, Stack, Text, Textarea, TextInput } from '@/ui'

/** Which form is open: a new item's, or the one replacing the item with this id. */
const NEW = ':new'

/**
 * The Custom tab of the sheet and of the build screen: whatever the player
 * wants written on the character that the rules have no field for, as any
 * number of titled texts.
 *
 * Each is a custom entry of kind `note` -- a name and a description the server
 * stores and gives no meaning to -- so a note an import once left is here too.
 * The text is drawn as it was typed and never as Markdown: the player wrote
 * it, and a stray asterisk is not formatting.
 *
 * Read-only without `characterId`, which is how a sheet shared with a table is
 * drawn. With it, an item is edited where it stands and a new one is written
 * where the Add button was; nothing here opens a dialog but the question
 * before a delete. A write reads the log's head first, as the sheet's other
 * edits do, so neither screen has to hand a revision down.
 */
export function CustomNotes({ options, characterId, disabled = false, onChanged }: {
  options: readonly CustomOption[] | undefined
  characterId?: string
  disabled?: boolean
  onChanged?: () => void
}) {
  const t = useT()
  const notes = (options ?? []).filter((option) => option.kind === 'note')
  const [editing, setEditing] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<CustomOption | null>(null)

  const write = useAction(async (change: { save: CustomOption } | { remove: string }) => {
    if (characterId === undefined) return
    const log = await getEvents(characterId)
    const revision = log.revision ?? log.seq
    if ('save' in change) await upsertCustomOption(characterId, revision, change.save)
    else await deleteCustomOption(characterId, revision, change.remove)
    onChanged?.()
  })
  const editable = characterId !== undefined
  const busy = disabled || write.pending

  async function save(note: CustomOption) {
    if (await write.run({ save: note }) !== null) setEditing(null)
  }
  async function remove(note: CustomOption) {
    if (note.id !== undefined) await write.run({ remove: note.id })
    setDeleting(null)
  }

  return (
    <Stack gap="sm">
      {notes.length === 0 && editing !== NEW && <Text size="sm" c="dimmed">{t(editable ? 'customNotes.empty' : 'sheet.empty')}</Text>}
      {notes.map((note) => editing === note.id && editable ? (
        <NoteForm key={note.id} note={note} pending={busy} onSave={(next) => void save(next)} onCancel={() => setEditing(null)} />
      ) : (
        <Paper key={note.id ?? note.name} component="article" aria-label={note.name} withBorder radius="sm" p="sm">
          <Stack gap="xs">
            <Group justify="space-between" wrap="nowrap" align="flex-start">
              <Text size="sm" fw={600} style={{ minWidth: 0, overflowWrap: 'anywhere' }}>{note.name}</Text>
              {editable && <Group gap="xs" wrap="nowrap">
                <Button variant="default" disabled={busy} aria-label={t('list.rowAction', { label: t('common.edit'), name: note.name })}
                  onClick={() => setEditing(note.id ?? null)}>{t('common.edit')}</Button>
                <Button variant="default" color="red" disabled={busy} aria-label={t('list.rowAction', { label: t('common.delete'), name: note.name })}
                  onClick={() => setDeleting(note)}>{t('common.delete')}</Button>
              </Group>}
            </Group>
            {note.description !== '' && <Text size="sm" style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{note.description}</Text>}
          </Stack>
        </Paper>
      ))}
      {write.error !== null && <Text size="sm" c="red">{write.error}</Text>}
      {editable && (editing === NEW
        ? <NoteForm pending={busy} onSave={(next) => void save(next)} onCancel={() => setEditing(null)} />
        : <Group><Button variant="light" disabled={busy} onClick={() => setEditing(NEW)}>{t('common.add')}</Button></Group>)}

      {/* No onSubmit: a confirmation is not a form. */}
      <ModalSheet opened={deleting !== null} onClose={() => setDeleting(null)} title={t('customNotes.deleteTitle', { name: deleting?.name ?? '' })}>
        <Stack gap="md">
          <Text size="sm">{t('customNotes.deleteWarning')}</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleting(null)}>{t('common.cancel')}</Button>
            <Button color="red" loading={write.pending} onClick={() => deleting !== null && void remove(deleting)}>{t('common.delete')}</Button>
          </Group>
        </Stack>
      </ModalSheet>
    </Stack>
  )
}

/** A title and a text, in place of the item they replace or the button that asked for a new one. */
function NoteForm({ note, pending, onSave, onCancel }: {
  note?: CustomOption
  pending: boolean
  onSave: (note: CustomOption) => void
  onCancel: () => void
}) {
  const t = useT()
  const [name, setName] = useState(note?.name ?? '')
  const [description, setDescription] = useState(note?.description ?? '')
  return (
    <Paper component="form" withBorder radius="sm" p="sm" aria-label={note?.name ?? t('customNotes.new')}
      onSubmit={(event) => {
        event.preventDefault()
        onSave({ ...note, kind: 'note', name: name.trim(), description, source: note?.source ?? '', selected: true })
      }}>
      <Stack gap="sm">
        <TextInput label={t('customNotes.title')} value={name} maxLength={300} required autoFocus
          onChange={(event) => setName(event.currentTarget.value)} />
        {/* Fixed rows rather than autosize, for the reason WrittenForm gives. */}
        <Textarea label={t('customNotes.text')} value={description} maxLength={16000} rows={6}
          onChange={(event) => setDescription(event.currentTarget.value)} />
        <Group>
          <Button type="submit" loading={pending} disabled={name.trim() === ''}>{t('sheet.save')}</Button>
          <Button variant="default" onClick={onCancel}>{t('common.cancel')}</Button>
        </Group>
      </Stack>
    </Paper>
  )
}
