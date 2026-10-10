import { useState } from 'react'

import { copyCharacter, createCopyLink, deleteCharacter, moveCharacter } from '@/lib/api'
import type { Folder, Summary } from '@/lib/api'
import { copyText } from '@/lib/clipboard'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import {
  ACTION_ICON_SIZE,
  Alert,
  Button,
  Group,
  IconCopy,
  IconFolder,
  IconSend,
  IconTrash,
  ModalSheet,
  Select,
  SHEET_COMBOBOX,
  Stack,
  Text,
  TextInput,
  type RowAction,
} from '@/ui'

/**
 * What a character's row can be made to do, and the dialogs two of those three
 * need.
 *
 * A hook rather than a component, and that is forced rather than preferred:
 * `DataList` takes a row's actions as *data* and renders no children of its
 * own, so a `ModalSheet` can no longer live inside the row that opens it. It
 * never should have -- a dialog mounted in a table cell is a dialog that
 * disappears when its row does, and the reload that follows a move is exactly
 * the moment the row moves to another folder's table.
 *
 * So the state and the sheets belong to the screen, which is where every other
 * dialog on this page already lives, and this returns both halves: the actions
 * to hand to each folder's list, and the sheets to render once at the bottom.
 *
 * Copy has no dialog because it asks nothing -- it makes a second character and
 * reloads. Only the two that need an answer get one.
 */
export function useCharacterActions(folders: Folder[], onChanged: () => void) {
  const t = useT()
  const [moving, setMoving] = useState<Summary | null>(null)
  const [target, setTarget] = useState('')
  const [deleting, setDeleting] = useState<Summary | null>(null)

  const [sending, setSending] = useState<Summary | null>(null)
  const [link, setLink] = useState('')
  const [copied, setCopied] = useState<boolean | null>(null)

  const move = useAction(moveCharacter)
  const copy = useAction(copyCharacter)
  const mint = useAction(createCopyLink)
  const remove = useAction(deleteCharacter)

  // Every action carries its row's name, which `DataList` appends: a list of
  // these is otherwise a column of buttons all called "Delete", ambiguous to a
  // screen reader and to a test alike.
  const actionsFor = (character: Summary): RowAction[] => [
    {
      key: 'move',
      label: t('characters.move'),
      icon: <IconFolder size={ACTION_ICON_SIZE} />,
      onClick: () => {
        setTarget(character.folder)
        setMoving(character)
      },
    },
    {
      key: 'copy',
      label: t('characters.copy'),
      icon: <IconCopy size={ACTION_ICON_SIZE} />,
      onClick: () => {
        void copy.run(character.id).then((made) => {
          if (made !== null) onChanged()
        })
      },
    },
    {
      key: 'send',
      label: t('characters.sendCopy'),
      icon: <IconSend size={ACTION_ICON_SIZE} />,
      onClick: () => {
        setLink('')
        setCopied(null)
        setSending(character)
        // The token rides in the fragment, which no browser sends to a server.
        void mint.run(character.id).then((made) => {
          if (made !== null) setLink(`${window.location.origin}/characters/receive#${made.token}`)
        })
      },
    },
    {
      key: 'delete',
      label: t('common.delete'),
      color: 'red' as const,
      icon: <IconTrash size={ACTION_ICON_SIZE} />,
      onClick: () => setDeleting(character),
    },
  ]

  const movingLabel = moving === null ? '' : moving.name || t('characters.thisCharacter')
  const sendingLabel = sending === null ? '' : sending.name || t('characters.thisCharacter')
  const deletingLabel = deleting === null ? '' : deleting.name || t('characters.thisCharacter')

  const sheets = (
    <>
      <ModalSheet
        opened={moving !== null}
        onClose={() => setMoving(null)}
        title={t('characters.moveNamed', { name: movingLabel })}
      >
        <Stack gap="md">
          <Select
            label={t('characters.folder')}
            comboboxProps={SHEET_COMBOBOX}
            data={folders.map((f) => ({ value: f.id, label: f.name }))}
            value={target}
            onChange={(value) => setTarget(value ?? target)}
            allowDeselect={false}
          />
          {move.error !== null && <Alert color="red">{move.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setMoving(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              loading={move.pending}
              disabled={moving === null || target === moving.folder}
              onClick={() => {
                if (moving === null) return
                void move.run(moving.id, target).then((ok) => {
                  if (ok !== null) {
                    setMoving(null)
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

      <ModalSheet
        opened={sending !== null}
        onClose={() => setSending(null)}
        title={t('sendCopy.title', { name: sendingLabel })}
      >
        <Stack gap="md">
          <Text size="sm">{t('sendCopy.bargain')}</Text>
          {mint.error !== null && <Alert color="red">{mint.error}</Alert>}
          {/* A field as well as a button: copying needs a secure context, and
              a link that can be selected is still a link that can be sent. */}
          <TextInput
            readOnly
            aria-label={t('sendCopy.link')}
            value={link}
            disabled={link === ''}
            onFocus={(event) => event.currentTarget.select()}
          />
          {copied === false && <Text size="sm" c="dimmed">{t('invite.clipboard.detail')}</Text>}
          <Group justify="flex-end">
            <Button
              variant={copied ? 'light' : 'filled'}
              loading={mint.pending}
              disabled={link === ''}
              onClick={() => void copyText(link).then(setCopied)}
            >
              {copied ? t('invite.copied') : t('invite.copyLink')}
            </Button>
          </Group>
        </Stack>
      </ModalSheet>

      <ModalSheet
        opened={deleting !== null}
        onClose={() => setDeleting(null)}
        title={t('characters.deleteNamedTitle', { name: deletingLabel })}
      >
        <Stack gap="md">
          <Text size="sm">
            {t('characters.deleteWarning')}
          </Text>
          {remove.error !== null && <Alert color="red">{remove.error}</Alert>}
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setDeleting(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              color="red"
              loading={remove.pending}
              onClick={() => {
                if (deleting === null) return
                void remove.run(deleting.id).then((ok) => {
                  if (ok !== null) {
                    setDeleting(null)
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

  return { actionsFor, sheets }
}
