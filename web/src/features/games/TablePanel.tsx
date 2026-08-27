import { useState } from 'react'
import { Link } from 'react-router'

import type { GroupRole, Summary, TableCharacter } from '@/lib/api'
import { listCharacters, listTable, shareCharacter, unshareCharacter } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useAuth } from '@/lib/auth'
import { useResource } from '@/lib/useResource'
import { useT } from '@/lib/i18n'
import {
  ACTION_ICON_SIZE,
  Alert,
  Anchor,
  Badge,
  Button,
  DataList,
  Group,
  IconPlus,
  Loader,
  ModalSheet,
  Stack,
  Text,
} from '@/ui'

import { atLeast } from '../groups/roles'

import { classLine } from '@/domain'

/**
 * The characters a group's members have put on its table.
 *
 * Sharing is a read and nothing more: every member can open any sheet here,
 * and only the owner of a character can ever change it. The controls below say
 * so by what they do not offer -- there is no edit anywhere on this panel,
 * because there is no route behind one.
 */
export function TablePanel({ groupId, role }: { groupId: string; role: GroupRole }) {
  const t = useT()
  const { user } = useAuth()
  const me = user?.id ?? ''
  const canManage = atLeast(role, 'dm')

  const table = useResource(`table:${groupId}`, (signal) => listTable(groupId, signal))
  const [sharing, setSharing] = useState(false)
  const share = useAction(shareCharacter)
  const unshare = useAction(unshareCharacter)

  if (table.loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          {t('table.loading')}
        </Text>
      </Group>
    )
  }
  if (table.error !== null || table.data === null) {
    return (
      <Alert color="red" title={t('table.loadFailed')}>
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{table.error ?? t('error.unknown')}</Text>
          <Button variant="light" onClick={table.reload}>
            {t('page.retry')}
          </Button>
        </Stack>
      </Alert>
    )
  }

  const characters = table.data.characters
  const failure = share.error ?? unshare.error

  async function act(work: Promise<unknown | null>) {
    if ((await work) === null) return
    table.refresh()
  }

  return (
    <Stack gap="md">
      {failure !== null && (
        <Alert color="red" title={t('group.actionFailed')}>
          {failure}
        </Alert>
      )}

      <DataList
        items={characters}
        getKey={(character) => character.id}
        // "Yours" is a mark on the name rather than a column: no table here
        // ever gave it a header, and it used to be hand-built inside the name
        // cell -- which is what put a `div` inside a `<Text>`'s paragraph.
        badges={(character: TableCharacter) =>
          character.owner_id === me ? (
            <Badge size="xs" variant="light">
              {t('table.yours')}
            </Badge>
          ) : null
        }
        actions={(character: TableCharacter) =>
          // Your own always; anybody's if you run the table, because a guest's
          // session ends and their character would otherwise sit here with
          // nobody able to take it down.
          character.owner_id !== me && !canManage
            ? []
            : [
                {
                  key: 'take-off',
                  label: t('table.takeOff'),
                  color: 'red',
                  onClick: () => void act(unshare.run(groupId, character.id)),
                },
              ]
        }
        columns={[
          {
            key: 'name',
            header: t('game.character'),
            primary: true,
            text: (character: TableCharacter) => character.name || t('common.unnamed'),
            // The sheet, and only the sheet. The event log is the record of its
            // owner's decisions and is not the table's business.
            to: (character: TableCharacter) => `/groups/${groupId}/characters/${character.id}`,
            render: (character: TableCharacter) => (
              <Anchor component={Link} to={`/groups/${groupId}/characters/${character.id}`}>
                <Text size="sm">{character.name || t('common.unnamed')}</Text>
              </Anchor>
            ),
          },
          {
            key: 'classes',
            header: t('game.class'),
            // `classLine` answers "--" for a character with no class yet, which
            // is right for a table cell and is exactly what the card drops.
            render: (character: TableCharacter) => classLine(character.classes),
          },
          {
            key: 'level',
            header: t('game.level'),
            // `|| null` rather than the bare number: a character still being
            // built is level 0, and "Level 0" is not a fact worth a line on a
            // phone. The table keeps drawing the zero, where a blank cell in a
            // column of numbers would read as a fault.
            render: (character: TableCharacter) => character.level || null,
          },
        ]}
        empty={t('table.empty')}
      />

      {/* Under the table, on the left, like every other way of adding a row. */}
      <Group>
        <Button
          variant="light"
          leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
          onClick={() => setSharing(true)}
        >
          {t('table.addCharacter')}
        </Button>
      </Group>

      <ShareSheet
        opened={sharing}
        already={characters.map((character) => character.id)}
        pending={share.pending}
        onClose={() => setSharing(false)}
        onPick={(id) => void act(share.run(groupId, id)).then(() => setSharing(false))}
      />
    </Stack>
  )
}

/** Pick one of your own characters to put on the table. */
function ShareSheet({
  opened,
  already,
  pending,
  onClose,
  onPick,
}: {
  opened: boolean
  already: string[]
  pending: boolean
  onClose: () => void
  onPick: (id: string) => void
}) {
  const t = useT()
  const mine = useResource(opened ? 'characters:mine' : '', (signal) =>
    listCharacters(undefined, signal),
  )
  const characters = mine.data?.characters ?? []

  return (
    <ModalSheet opened={opened} onClose={onClose} title={t('table.putOnTable')}>
      <Stack gap="sm">
        <Text size="sm" c="dimmed">
          {t('table.shareWarning')}
        </Text>
        {mine.loading && <Loader size="sm" />}
        {characters.length === 0 && !mine.loading && (
          <Text size="sm">{t('table.noCharacters')}</Text>
        )}
        {characters.map((character: Summary) => {
          const shared = already.includes(character.id)
          return (
            <Group key={character.id} justify="space-between">
              <div>
                <Text size="sm">{character.name || t('common.unnamed')}</Text>
                <Text size="xs" c="dimmed">
                  {classLine(character.classes)}
                </Text>
              </div>
              <Button
                variant="light"
                disabled={shared}
                loading={pending}
                onClick={() => onPick(character.id)}
              >
                {shared ? t('table.alreadyThere') : t('common.add')}
              </Button>
            </Group>
          )
        })}
      </Stack>
    </ModalSheet>
  )
}
