import { useEffect, useId, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'

import type { GameDetail, GameEntry } from '@/lib/api'
import { addGameMonster, deleteGameEntry, orderGameEntries, patchGameEntry, restGame } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import {
  Avatar, characterAvatar, playerAvatar,
  ACTION_ICON_SIZE, ActionIcon, Affix, Alert, Anchor, Badge, Box, Button, Card,
  Group, IconArrowDown, IconArrowsExchange, IconArrowUp, IconBackpack, IconCoins, IconDice5, IconDotsVertical, IconGripVertical, IconPencil,
  IconShield, IconSwords, IconTrash, IconPlus, IconChevronDown, Menu, ModalSheet, Notification, Stack, Text, useIsDesktop,
} from '@/ui'
import { CoinsSheet } from './CoinsSheet'
import { CompactStats } from './CompactStats'
import { ConsumablesSheet } from './ConsumablesSheet'
import { EntryEditor } from './EntryEditor'
import { EntryTags } from './EntryTags'
import { FolderTreeSheet } from './FolderTreeSheet'
import { GrantItems } from './GrantItems'
import { InitiativeSheet } from './InitiativeSheet'
import { ItemsSheet } from './ItemsSheet'
import { sheetPath } from './sheetPath'
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
  const master = game.role === 'owner' || game.role === 'dm'
  const [editing, setEditing] = useState<string | null>(null)
  const [rolling, setRolling] = useState<string | null>(null)
  const [consuming, setConsuming] = useState<string | null>(null)
  const [carrying, setCarrying] = useState<{ id: string; mode: 'use' | 'give' } | null>(null)
  const [paying, setPaying] = useState<string | null>(null)
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
  const patch = useAction(patchGameEntry)
  const remove = useAction(deleteGameEntry)
  const order = useAction(orderGameEntries)
  const monster = useAction(addGameMonster)
  const rest = useAction(restGame)
  const [resting, setResting] = useState<'short' | 'long' | null>(null)
  const entries = game.entries
  const selected = entries.find((entry) => entry.id === editing)
  const roller = entries.find((entry) => entry.id === rolling)
  const consumer = entries.find((entry) => entry.id === consuming)
  const carrier = entries.find((entry) => entry.id === carrying?.id)
  const payee = entries.find((entry) => entry.id === paying)
  const players = entries.filter((entry) => entry.kind === 'player' && entry.character_id !== undefined)
  const owned = (entry: GameEntry) => game.characters.find((each) => each.id === entry.character_id)?.owner_id === me
  const myPlayers = players.filter(owned)
  const error = patch.error ?? remove.error ?? order.error ?? monster.error ?? rest.error
  const pending = patch.pending || remove.pending || order.pending || monster.pending || rest.pending

  async function act(work: Promise<unknown | null>) {
    if (await work !== null) onChange()
  }

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
          const mine = entry.kind === 'player' && owned(entry)
          const actions = [
            // A player's card edits what a fight changes, and says so; an NPC's also edits who it is.
            ...(entry.can_edit ? [{ label: entry.kind === 'player' ? t('game.hp') : t('common.edit'), icon: IconPencil,
              run: () => { patch.reset(); setEditing(entry.id) } }] : []),
            // Its own entry and its own dialog: it is set once, when the fight starts, and hit points all through it.
            ...(entry.can_edit ? [{ label: t('vitals.initiative'), icon: IconSwords,
              run: () => { patch.reset(); setRolling(entry.id) } }] : []),
            // The server sends pools to the character's owner and to a DM only; a locked owner reads them and cannot spend.
            ...((entry.resources ?? []).length > 0 ? [{ label: t('sheet.consumables'), icon: IconDice5,
              run: () => setConsuming(entry.id) }] : []),
            // Coins, using and handing over are three entries and three dialogs: each does one thing.
            // A DM pays or charges anybody; an owner counts their own coins.
            ...(entry.kind === 'player' && (master || mine) ? [{ label: t('game.coins'), icon: IconCoins, run: () => setPaying(entry.id) }] : []),
            ...(mine ? [{ label: t('game.useItem'), icon: IconBackpack, run: () => setCarrying({ id: entry.id, mode: 'use' }) }] : []),
            // Handing over starts at whoever receives: on their card, to anybody with a character of their own to give from.
            ...(entry.kind === 'player' && myPlayers.some((each) => each.id !== entry.id)
              ? [{ label: t('game.transferItem'), icon: IconArrowsExchange, run: () => setCarrying({ id: entry.id, mode: 'give' }) }] : []),
            // An item no catalogue holds is written on a page of its own; the search under the roster gives the rest.
            ...(master && entry.kind === 'player' && entry.character_id ? [{ label: t('customItem.add'), icon: IconPlus,
              run: () => void navigate(`/games/${game.id}/characters/${entry.character_id}/custom-item?entry=${encodeURIComponent(entry.id)}`) }] : []),
            ...(master ? [
              ...(entry.kind === 'player' ? [{ label: entry.locked ? t('game.unlock') : t('game.lock'), icon: IconShield,
                run: () => void act(patch.run(game.id, entry.id, { locked: !entry.locked })) }] : []),
              ...(index > 0 ? [{ label: t('game.moveUp'), icon: IconArrowUp,
                run: () => void act(order.run(game.id, { entry_id: entry.id, direction: -1 })) }] : []),
              ...(index < entries.length - 1 ? [{ label: t('game.moveDown'), icon: IconArrowDown,
                run: () => void act(order.run(game.id, { entry_id: entry.id, direction: 1 })) }] : []),
              { label: t('common.remove'), icon: IconTrash,
                run: () => void act(remove.run(game.id, entry.id)), color: 'red' },
            ] : []),
          ]
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
                    {actions.length > 0 && (
                      <Menu position="bottom-end">
                        <Menu.Target>
                          <ActionIcon variant="subtle" color="gray" aria-label={t('list.actions', { name })}>
                            <IconDotsVertical size={ACTION_ICON_SIZE} />
                          </ActionIcon>
                        </Menu.Target>
                        <Menu.Dropdown>
                          {actions.map((action) => <Menu.Item key={action.label}
                            {...('color' in action ? { color: action.color } : {})}
                            leftSection={<action.icon size={ACTION_ICON_SIZE} />}
                            disabled={pending} onClick={action.run}>{action.label}</Menu.Item>)}
                        </Menu.Dropdown>
                      </Menu>
                    )}
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
      {carrier !== undefined && carrying?.mode === 'use' && <ItemsSheet gameId={game.id} from={[carrier]} onClose={() => setCarrying(null)}
        onDone={(text) => { setCarrying(null); setNotice({ title: t('game.used'), text }) }} />}
      {carrier !== undefined && carrying?.mode === 'give' && <ItemsSheet gameId={game.id} to={carrier}
        from={myPlayers.filter((entry) => entry.id !== carrier.id)} onClose={() => setCarrying(null)}
        onDone={(text) => { setCarrying(null); setNotice({ title: t('game.transferred'), text }) }} />}
      {notice !== null && <Affix position={{ bottom: 16, right: 16 }}>
        <Notification withBorder role="status" color="green" title={notice.title} onClose={() => setNotice(null)}>{notice.text}</Notification>
      </Affix>}
      {payee?.character_id !== undefined && <CoinsSheet gameId={game.id} entry={payee} characterId={payee.character_id} master={master}
        onClose={() => setPaying(null)} />}
      {consumer && <ConsumablesSheet gameId={game.id} entry={consumer} onClose={() => setConsuming(null)} onChange={onChange} />}
      {selected && <EntryEditor key={selected.id} entry={selected} pending={patch.pending} error={patch.error}
        onClose={() => setEditing(null)} onSave={async (changes) => {
          if (await patch.run(game.id, selected.id, changes) === null) return
          setEditing(null)
          onChange()
        }} />}
      {roller && <InitiativeSheet key={roller.id} entry={roller} pending={patch.pending} error={patch.error}
        onClose={() => setRolling(null)} onSave={async (initiative) => {
          if (await patch.run(game.id, roller.id, { initiative }) === null) return
          setRolling(null)
          onChange()
        }} />}
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
