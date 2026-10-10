import { Link } from 'react-router'

import { classLine } from '@/domain'
import type { Summary } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Anchor, Avatar, characterAvatar, DataList, type RowAction } from '@/ui'

/** One folder's characters, as the table inside its card. */
export function CharacterListTable({
  items,
  actions,
}: {
  items: Summary[]
  actions: (character: Summary) => RowAction[]
}) {
  const t = useT()

  return (
    <DataList
      items={items}
      getKey={(character) => character.id}
      leading={(character) => <Avatar image={character.image} fallback={characterAvatar(character.classes)} />}
      actions={actions}
      columns={[
        {
          key: 'name',
          header: '',
          primary: true,
          text: (character) => character.name || t('common.unnamed'),
          to: (character) => `/characters/${character.id}`,
          render: (character) => (
            <Anchor component={Link} to={`/characters/${character.id}`}>
              {character.name || t('common.unnamed')}
            </Anchor>
          ),
        },
        {
          key: 'level',
          header: t('game.level'),
          // The table prints "--" for an unbuilt character, because
          // a blank cell in a column of numbers reads as a fault.
          // The card has no column to keep straight, so it says
          // nothing at all -- which is what `null` asks for.
          render: (character) => character.level || null,
        },
        {
          key: 'classes',
          header: t('characters.classes'),
          render: (character) => classLine(character.classes),
        },
        // No Folder column. The card this table sits in is the
        // folder, so the column would write the same word down
        // every row of it.
      ]}
      empty={t('characters.folderEmpty')}
    />
  )
}
