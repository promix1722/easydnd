import { useEffect, useId, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'

import type { GameDetail } from '@/lib/api'
import { addGameMonster, restGame } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import {
  Avatar, characterAvatar, playerAvatar,
  ACTION_ICON_SIZE, ActionIcon, Affix, Alert, Anchor, Badge, Box, Button, Card,
  Group, IconDotsVertical, IconGripVertical, IconChevronDown, ModalSheet, Notification, Stack, Text, useIsDesktop,
} from '@/ui'
import { CompactStats } from './CompactStats'
import { EntryTags } from './EntryTags'
import { FolderTreeSheet } from './FolderTreeSheet'
import { GrantItems } from './GrantItems'
import { sheetPath } from './sheetPath'
import { useEntryActions } from './useEntryActions'
import { useRosterDrag } from './useRosterDrag'

/** One ordered list shared by the table; all private fields are filtered upstream. */
export function GameTracker({ game, onChange, onAddFromGroup }: {
  game: GameDetail
  onChange: () => void
  onAddFromGroup: () => void
}) {
  const t = useT()
  const desktop = useIsDesktop()
  const me = useAuth().user?.id ?? ''
  const detailsPrefix = useId()
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  // What changed hands, said in a corner and gone by itself: nothing on the roster shows an inventory.
  // The custom item page comes back with what it gave, to be said the same way.
  const navigate = useNavigate()
  const arrived = (useLocation().state as { given?: { item: string; name: string } } | null)?.given
  const [notice, setNotice] = useState<{ title: string; text: string } | null>(
    () => arrived === undefined ? null : { title: t('game.added'), text: t('game.gave', arrived) })
  // Said once: the navigation's state is dropped, so a reload does not say it again.
  useEffect(() => { if (arrived !== undefined) void navigate('.', { replace: true, state: null }) }, [arrived, navigate])
  useEffect(() => {
    if (notice === null) return
    const timer = setTimeout(() => setNotice(null), 4000)
    return () => clearTimeout(timer)
  }, [notice])
  const [pickingMonster, setPickingMonster] = useState(false)
  // The row menu and its dialogs are the sheet's too: see useEntryActions.
  const row = useEntryActions(game, onChange, (title, text) => setNotice({ title, text }))
  const { master, players, order, act } = row
  const monster = useAction(addGameMonster)
  const rest = useAction(restGame)
  const [resting, setResting] = useState<'short' | 'long' | null>(null)
  const entries = game.entries
  const error = row.error ?? monster.error ?? rest.error
  const pending = row.pending || monster.pending || rest.pending

  // A phone has no grip: a row there is too narrow to give a column to one, and
  // a drag under a thumb fights the page's scroll. Its order is the menu's Move up and Move down.
  const draggable = desktop && master && !pending && entries.length > 1

  const { dragging, over, startDrag, moveDrag, drop, cancelDrag } = useRosterDrag({
    gameId: game.id, entries, draggable,
    onMove: (entryId, beforeId) => void act(order.run(game.id, { entry_id: entryId, before_id: beforeId })),
  })

  return (
    <Stack gap="sm">
      {error !== null && <Alert color="red" title={t('group.actionFailed')}>{error}</Alert>}
      {master && (
        // One row of one kind of button: what a DM does to the table as a whole.
        <Group gap="xs">
          <Button variant="light" disabled={pending} onClick={onAddFromGroup}>{t('game.addFromGroup')}</Button>
          <Button variant="light" disabled={pending} onClick={() => void act(monster.run(game.id))}>{t('game.addMonsterStub')}</Button>
          <Button variant="light" disabled={pending} onClick={() => setPickingMonster(true)}>{t('game.addMonsterFromMine')}</Button>
          <Button variant="light" disabled={pending} onClick={() => setResting('long')}>{t('game.longRest')}</Button>
          <Button variant="light" disabled={pending} onClick={() => setResting('short')}>{t('game.shortRest')}</Button>
          <Button variant="light" disabled={pending || entries.length < 2}
            onClick={() => void act(order.run(game.id, { by_initiative: true }))}>{t('game.orderInitiative')}</Button>
        </Group>
      )}
      {entries.length === 0 && <Text c="dimmed">{t('game.empty')}</Text>}
      <Stack gap="xs" data-game-roster={game.id}>
        {entries.map((entry, index) => {
          const name = entry.name || t('common.unnamed')
          const classes = entry.class ? [{ class: entry.class }] : game.characters.find((character) => character.id === entry.character_id)?.classes
          const fallback = characterAvatar(classes) ?? (entry.kind === 'monster' ? playerAvatar(entry.id) : undefined)
          // A closed character is a name and no link to a player who does not own it.
          const sheet = entry.character_id
            ? sheetPath(game.group_id, game.characters.find((each) => each.id === entry.character_id) ?? { id: entry.character_id }, me, master)
            : undefined
          return (
            <Box key={entry.id}>
              <Box h={2} mb={4} aria-hidden
                bg={dragging !== null && over === entry.id ? 'var(--mantine-primary-color-filled)' : 'transparent'} />
              <Card withBorder radius="md" padding="xs" component="article" aria-label={name} data-game-entry={entry.id}
                style={{ opacity: dragging === entry.id ? 0.55 : 1 }}>
                <Stack gap="xs">
                  <Group gap="xs" wrap="nowrap" align="center">
                    {draggable && <ActionIcon variant="subtle" color="gray" size="md"
                      aria-label={t('game.dragEntry', { name })} title={t('game.dragEntry', { name })}
                      onPointerDown={(event) => startDrag(event, entry.id)} onPointerMove={moveDrag}
                      onPointerUp={drop} onPointerCancel={cancelDrag} onLostPointerCapture={cancelDrag}
                      style={{ touchAction: 'none', userSelect: 'none', cursor: dragging === entry.id ? 'grabbing' : 'grab', flexShrink: 0 }}>
                      <IconGripVertical size={ACTION_ICON_SIZE} aria-hidden />
                    </ActionIcon>}
                    <Stack gap={0} style={{ flex: 1, minWidth: 0 }}>
                      <Group gap="xs">
                        <Avatar image={entry.image} fallback={fallback} />
                        {sheet !== undefined ? (
                          <Anchor component={Link} draggable={false} size="sm" fw={500}
                            style={{ overflowWrap: 'anywhere' }}
                            to={sheet}>{name}</Anchor>
                        ) : <Text size="sm" fw={500} style={{ overflowWrap: 'anywhere' }}>{name}</Text>}
                        {entry.kind === 'monster' && <Badge size="xs" variant="light">{t('game.monster')}</Badge>}
                        {entry.locked && <Badge size="xs" color="gray" variant="light">{t('game.locked')}</Badge>}
                      </Group>
                    </Stack>
                    {!desktop && entry.stats && <ActionIcon variant="subtle" color="gray"
                      aria-label={t(expanded.has(entry.id) ? 'game.hideDetails' : 'game.showDetails')}
                      aria-expanded={expanded.has(entry.id)} aria-controls={`${detailsPrefix}-${entry.id}`}
                      onClick={() => setExpanded((previous) => {
                        const next = new Set(previous)
                        if (next.has(entry.id)) next.delete(entry.id); else next.add(entry.id)
                        return next
                      })}>
                      <IconChevronDown size={ACTION_ICON_SIZE} style={{ transform: expanded.has(entry.id) ? 'rotate(180deg)' : undefined }} />
                    </ActionIcon>}
                    {row.menu(entry, index, (
                      <ActionIcon variant="subtle" color="gray" aria-label={t('list.actions', { name })}>
                        <IconDotsVertical size={ACTION_ICON_SIZE} />
                      </ActionIcon>
                    ), pending)}
                  </Group>
                  <CompactStats entry={entry} expanded={expanded.has(entry.id)} detailsId={`${detailsPrefix}-${entry.id}`} />
                  {entry.stats && <EntryTags gameId={game.id} entry={entry} onChange={onChange} />}
                </Stack>
              </Card>
            </Box>
          )
        })}
        {desktop && master && entries.length > 1 && (
          <Box h={16} aria-hidden data-game-drop-end>
            <Box h={2} bg={dragging !== null && over === '' ? 'var(--mantine-primary-color-filled)' : 'transparent'} />
          </Box>
        )}
      </Stack>
      {master && players.length > 0 && <GrantItems gameId={game.id} players={players}
        onGiven={(text) => setNotice({ title: t('game.added'), text })} />}
      {notice !== null && <Affix position={{ bottom: 16, right: 16 }}>
        <Notification withBorder role="status" color="green" title={notice.title} onClose={() => setNotice(null)}>{notice.text}</Notification>
      </Affix>}
      {row.dialogs}
      <ModalSheet opened={resting !== null} onClose={() => setResting(null)} title={resting === 'short' ? t('game.shortRest') : t('game.longRest')}
        onSubmit={() => { if (resting !== null) void act(rest.run(game.id, resting)); setResting(null) }}>
        <Stack gap="sm">
          <Text size="sm">{resting === 'short' ? t('game.shortRestHint') : t('game.longRestHint')}</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setResting(null)}>{t('common.cancel')}</Button>
            <Button type="submit">{t('game.apply')}</Button>
          </Group>
        </Stack>
      </ModalSheet>
      {master && pickingMonster && <FolderTreeSheet opened seated={new Set()} pending={monster.pending}
        title={t('game.addMonsterFromMine')} description={t('game.privateMonsterHint')}
        onClose={() => setPickingMonster(false)} onAdd={(ids) => {
          void (async () => {
            for (const id of ids) {
              if (await monster.run(game.id, id) === null) { onChange(); return }
            }
            setPickingMonster(false)
            onChange()
          })()
        }} />}
    </Stack>
  )
}
