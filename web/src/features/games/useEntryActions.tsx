import { useState } from 'react'
import type { ReactElement } from 'react'
import { useNavigate } from 'react-router'

import type { GameDetail, GameEntry } from '@/lib/api'
import { deleteGameEntry, orderGameEntries, patchGameEntry } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import {
  ACTION_ICON_SIZE, Button, Group, IconArrowDown, IconArrowsExchange, IconArrowUp, IconBackpack, IconCoins, IconDice5, IconPencil,
  IconShield, IconSwords, IconTrash, IconPlus, Menu,
} from '@/ui'
import { CoinsSheet } from './CoinsSheet'
import { ConsumablesSheet } from './ConsumablesSheet'
import { EntryEditor } from './EntryEditor'
import { InitiativeSheet } from './InitiativeSheet'
import { ItemsSheet } from './ItemsSheet'

/**
 * What can be done to one roster entry, and the dialogs that do it.
 *
 * A hook because two screens offer it -- the game's roster, on every row, and
 * a character's own sheet, for the one entry that is theirs -- and the list has
 * to be one list: who may do what is decided here once, from the game the
 * server sent, so the two cannot come to disagree.
 */
export function useEntryActions(game: GameDetail, onChange: () => void, onNotice?: (title: string, text: string) => void) {
  const t = useT()
  const me = useAuth().user?.id ?? ''
  const navigate = useNavigate()
  const master = game.role === 'owner' || game.role === 'dm'
  const [editing, setEditing] = useState<string | null>(null)
  const [rolling, setRolling] = useState<string | null>(null)
  const [consuming, setConsuming] = useState<string | null>(null)
  const [carrying, setCarrying] = useState<{ id: string; mode: 'use' | 'give' } | null>(null)
  const [paying, setPaying] = useState<string | null>(null)
  const patch = useAction(patchGameEntry)
  const remove = useAction(deleteGameEntry)
  const order = useAction(orderGameEntries)
  const entries = game.entries
  const selected = entries.find((entry) => entry.id === editing)
  const roller = entries.find((entry) => entry.id === rolling)
  const consumer = entries.find((entry) => entry.id === consuming)
  const carrier = entries.find((entry) => entry.id === carrying?.id)
  const payee = entries.find((entry) => entry.id === paying)
  const players = entries.filter((entry) => entry.kind === 'player' && entry.character_id !== undefined)
  const owned = (entry: GameEntry) => game.characters.find((each) => each.id === entry.character_id)?.owner_id === me
  const myPlayers = players.filter(owned)
  const error = patch.error ?? remove.error ?? order.error
  const pending = patch.pending || remove.pending || order.pending

  async function act(work: Promise<unknown | null>) {
    if (await work !== null) onChange()
  }

  function actionsFor(entry: GameEntry, index: number) {
    const mine = entry.kind === 'player' && owned(entry)
    return [
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
      // Using up is the owner's, and a DM's on anybody's: a potion drunk at the table is struck off by whoever is keeping count.
      ...(mine || (master && entry.kind === 'player') ? [{ label: t('game.useItem'), icon: IconBackpack, run: () => setCarrying({ id: entry.id, mode: 'use' }) }] : []),
      // Handing over is offered to anybody with a character of their own to give from, on every card
      // there is somebody to hand to: their own opens as the giver, anybody else's as the receiver.
      ...(entry.kind === 'player' && myPlayers.length > 0 && players.length > 1
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
  }

  /** The entry's menu under `target`, or nothing for an entry the viewer can do nothing to. */
  function menu(entry: GameEntry, index: number, target: ReactElement, disabled = pending) {
    const actions = actionsFor(entry, index)
    if (actions.length === 0) return null
    return (
      <Menu position="bottom-end">
        <Menu.Target>{target}</Menu.Target>
        <Menu.Dropdown>
          {actions.map((action) => <Menu.Item key={action.label}
            {...('color' in action ? { color: action.color } : {})}
            leftSection={<action.icon size={ACTION_ICON_SIZE} />}
            disabled={disabled} onClick={action.run}>{action.label}</Menu.Item>)}
        </Menu.Dropdown>
      </Menu>
    )
  }

  /** The same entries laid out as buttons: where there is one entry on the page, nothing needs folding away. */
  function buttons(entry: GameEntry, index: number) {
    const actions = actionsFor(entry, index)
    if (actions.length === 0) return null
    return (
      <Group gap="xs">
        {actions.map((action) => <Button key={action.label} variant="light" size="xs"
          {...('color' in action ? { color: action.color } : {})}
          leftSection={<action.icon size={ACTION_ICON_SIZE} />}
          disabled={pending} onClick={action.run}>{action.label}</Button>)}
      </Group>
    )
  }

  // Coins and items are written to the character, not the game: the caller is
  // told all the same, because a sheet on screen has to be read again.
  const dialogs = <>
    {carrier !== undefined && carrying?.mode === 'use' && <ItemsSheet gameId={game.id} from={[carrier]} master={!owned(carrier)} onClose={() => setCarrying(null)}
      onDone={(text) => { setCarrying(null); onNotice?.(t('game.used'), text); onChange() }} />}
    {carrier !== undefined && carrying?.mode === 'give' && <ItemsSheet gameId={game.id} at={carrier}
      from={myPlayers} others={players} onClose={() => setCarrying(null)}
      onDone={(text) => { setCarrying(null); onNotice?.(t('game.transferred'), text); onChange() }} />}
    {payee?.character_id !== undefined && <CoinsSheet gameId={game.id} entry={payee} characterId={payee.character_id} master={master}
      onClose={() => { setPaying(null); onChange() }} />}
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
  </>

  return { menu, buttons, dialogs, error, pending, order, act, master, players }
}
