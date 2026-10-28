import type { GameDetail, GameEntry } from '@/lib/api'
import { getGame } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useT } from '@/lib/i18n'
import { useId } from 'react'

import { Alert, Card, Panel, Stack, Text, Title } from '@/ui'
import { CompactStats } from './CompactStats'
import { ConsumablesEditor } from './ConsumablesSheet'
import { EntryTags } from './EntryTags'
import { poolsOf } from './pools'
import { useEntryActions } from './useEntryActions'

/**
 * What one game is keeping for one character, on that character's own sheet.
 *
 * Hit points, initiative and spent uses are the game's and not the
 * character's, so the sheet cannot say them; this is the game saying them
 * there, in the card the game's roster draws for the entry -- the same
 * numbers and the same tags, so a player reads one layout in both places.
 * HP, Initiative and Transfer are that roster row's menu entries, laid out here
 * as buttons, so there is one set of dialogs and one set of rules; the slots
 * are spent in their own box. The rest of the menu is left out: coins and
 * using an item are the sheet's own tabs, and a DM's work is the game page's.
 *
 * It follows the game at the game page's own pace, because a DM changes these
 * numbers while the player is looking at them.
 */
export function GameSeat({ gameId, characterId, onChanged }: {
  gameId: string
  characterId: string
  /** Called after any change made here: coins and items are written to the character. */
  onChanged: () => void
}) {
  const game = useResource(`game:${gameId}`, (signal) => getGame(gameId, signal),
    { pollInterval: 3000, retainOnRefreshError: true })
  // Nothing while it loads: the tab is not the one the sheet opens on.
  if (game.data === null) return game.error === null ? null : <Text size="sm" c="red">{game.error}</Text>
  const index = game.data.entries.findIndex((entry) => entry.kind === 'player' && entry.character_id === characterId)
  const entry = game.data.entries[index]
  // Unseated since the sheet asked: the tab outlives the seat until the next load.
  if (entry === undefined) return null
  return <Seat game={game.data} entry={entry} index={index} onChange={() => { game.refresh(); onChanged() }} />
}

/** What the roster row's menu offers here; see the component's comment for the rest. */
const SEAT_ACTIONS = ['hp', 'initiative', 'transfer'] as const

function Seat({ game, entry, index, onChange }: { game: GameDetail; entry: GameEntry; index: number; onChange: () => void }) {
  const t = useT()
  const detailsId = useId()
  const row = useEntryActions(game, onChange)
  const pools = poolsOf(entry)
  return (
    <Stack gap="lg">
      {row.error !== null && <Alert color="red" title={t('group.actionFailed')}>{row.error}</Alert>}
      <Card withBorder radius="md" padding="xs">
        <Stack gap="xs">
          {row.buttons(entry, index, SEAT_ACTIONS)}
          {/* Unfolded at every width: a phone's roster folds a row to fit many, and here there is one. */}
          <CompactStats entry={entry} expanded detailsId={detailsId} />
          {entry.stats && <EntryTags gameId={game.id} entry={entry} onChange={onChange} />}
        </Stack>
      </Card>
      {pools.length > 0 && (
        <Panel>
          <Stack gap="sm">
            <Title order={4}>{t('sheet.consumables')}</Title>
            <ConsumablesEditor gameId={game.id} entry={entry} onChange={onChange} />
          </Stack>
        </Panel>
      )}
      {row.dialogs}
    </Stack>
  )
}
