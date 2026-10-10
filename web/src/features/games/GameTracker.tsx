import { useEffect, useId, useRef, useState, type PointerEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'

import type { Change, EntryPatch, EntryStats, GameDetail, GameEntry, Item, ItemHit } from '@/lib/api'
import {
  addGameMonster, adjustCoins, bySlug, deleteGameEntry, getSharedSheet, getSheet, giveItem, grantItem,
  orderGameEntries, patchGameEntry, restGame, writeChanges,
} from '@/lib/api'
import { CatalogScope } from '@/lib/api/catalogScope'
import { useResource } from '@/lib/useResource'
import { useAction } from '@/lib/useAction'
import { useAuth } from '@/lib/auth'
import { useLocale, useT } from '@/lib/i18n'
import {
  Avatar, characterAvatar, playerAvatar,
  ACTION_ICON_SIZE, ActionIcon, Affix, Alert, Anchor, Badge, Box, Button, Card, Divider,
  Group, IconArrowDown, IconArrowsExchange, IconArrowUp, IconBackpack, IconCoins, IconDice5, IconDotsVertical, IconGripVertical, IconPencil,
  IconShield, IconSwords, IconTrash, IconPlus, IconChevronDown, ItemIcon, Menu, ModalSheet, Notification, NumberInput, Select, SHEET_COMBOBOX, SimpleGrid, Stack, Text, TextInput, useIsDesktop,
} from '@/ui'
import { ABILITY_ORDER, COINS, groupOf, mergeStacks, setCoin, setTotal, signed, titleCase } from '@/domain'
import { abilityAbbr, senseName, speedName } from '../character/labels'
import { AddItems } from '../character/ItemPicker'
import { ResourcePools } from '../character/ResourcePools'
import { FolderTreeSheet } from './FolderTreeSheet'
import { sheetPath } from './sheetPath'

const STAT_COLUMNS = { base: 2, sm: 4, md: 7 } as const

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
  const [dragging, setDragging] = useState<string | null>(null)
  const gesture = useRef<{ id: string; pointer: number; x: number; y: number; moved: boolean } | null>(null)
  const [over, setOver] = useState<string | null>(null)
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

  function endDrag() {
    gesture.current = null
    setDragging(null)
    setOver(null)
  }

  // Match folder ordering while using one gesture for mouse, touch and stylus.
  // Hit-test the real pointer position because pointer capture keeps events
  // arriving on the grip even when the pointer is over another row.
  function beforeAtPoint(from: string, x: number, y: number): string | null {
    const start = entries.findIndex((entry) => entry.id === from)
    if (start < 0) return null
    const element = document.elementFromPoint?.(x, y)
    if (element?.closest('[data-game-roster]')?.getAttribute('data-game-roster') !== game.id) return null
    if (element?.closest('[data-game-drop-end]')) return ''
    const target = element?.closest('[data-game-entry]')?.getAttribute('data-game-entry')
    const index = entries.findIndex((entry) => entry.id === target)
    if (index < 0) return null
    const gap = start < index ? index + 1 : index
    return entries[gap]?.id ?? ''
  }

  function startDrag(event: PointerEvent<HTMLButtonElement>, id: string) {
    if (!draggable || gesture.current || event.isPrimary === false || (event.pointerType === 'mouse' && event.button !== 0)) return
    event.preventDefault()
    event.currentTarget.setPointerCapture?.(event.pointerId)
    gesture.current = { id, pointer: event.pointerId, x: event.clientX, y: event.clientY, moved: false }
  }

  function moveDrag(event: PointerEvent<HTMLButtonElement>) {
    const drag = gesture.current
    if (!drag || event.pointerId !== drag.pointer) return
    if (!draggable || !entries.some((entry) => entry.id === drag.id)) { endDrag(); return }
    drag.moved ||= Math.hypot(event.clientX - drag.x, event.clientY - drag.y) >= 8
    if (!drag.moved) return
    event.preventDefault()
    setDragging(drag.id)
    setOver(beforeAtPoint(drag.id, event.clientX, event.clientY))
  }

  function drop(event: PointerEvent<HTMLButtonElement>) {
    const drag = gesture.current
    if (!drag || event.pointerId !== drag.pointer) return
    const before = beforeAtPoint(drag.id, event.clientX, event.clientY)
    if (draggable && drag.moved && before !== null && before !== drag.id) {
      void act(order.run(game.id, { entry_id: drag.id, before_id: before }))
    }
    endDrag()
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
  }

  function cancelDrag(event: PointerEvent<HTMLButtonElement>) {
    if (event.pointerId === gesture.current?.pointer) endDrag()
  }

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

/**
 * What a DM hands out, at the foot of the page: who gets it, then the sheet's
 * own item search over that character's catalogue. An Add closes the search:
 * a DM hands out one thing and goes back to the table. Nothing on the roster
 * shows an inventory, so the gift is said in the tracker's notice.
 */
function GrantItems({ gameId, players, onGiven }: { gameId: string; players: readonly GameEntry[]; onGiven: (text: string) => void }) {
  const t = useT()
  const [chosen, setChosen] = useState<string | null>(null)
  const to = players.find((entry) => entry.id === chosen) ?? players[0]
  const grant = useAction(grantItem)
  // Remounting the search is what closes it.
  const [round, setRound] = useState(0)
  if (to?.character_id === undefined) return null
  const name = (entry: GameEntry) => entry.name || t('common.unnamed')
  async function give(hit: ItemHit) {
    if (to === undefined) return
    if (await grant.run(gameId, to.id, hit.slug) === null) return
    onGiven(t('game.gave', { item: hit.name, name: name(to) }))
    setRound((previous) => previous + 1)
  }
  // The search is over the receiver's own catalogue: their rule packs decide what exists for them.
  return <CatalogScope.Provider value={`/shared/${encodeURIComponent(to.character_id)}/catalog`}>
    <AddItems key={round} label={t('game.giveItem')} disabled={grant.pending} onAdd={(hit) => void give(hit)}
      detailsTo={(hit) => `/games/${encodeURIComponent(gameId)}/characters/${encodeURIComponent(to.character_id ?? '')}/items/${encodeURIComponent(hit.slug)}`}>
      <Select label={t('game.giveItemTo')} allowDeselect={false} value={to.id} onChange={setChosen}
        data={players.map((entry) => ({ value: entry.id, label: name(entry) }))} />
      {grant.error !== null && <Alert color="red" title={t('group.actionFailed')}>{grant.error}</Alert>}
    </AddItems>
  </CatalogScope.Provider>
}

/**
 * A seated character's coins, and nothing else of theirs: the same dialog for
 * the DM and for its owner. Five rows, one coin under another, edited freely
 * and written once by Save -- a purse is counted, then agreed, and a write per
 * keystroke would be an entry in the character's log for every digit.
 *
 * The two write differently behind the one button. An owner sets their own
 * totals; a DM sends the difference, so coins the player spent in the same
 * moment are not put back.
 */
function CoinsSheet({ gameId, entry, characterId, master, onClose }: {
  gameId: string; entry: GameEntry; characterId: string; master: boolean; onClose: () => void
}) {
  const t = useT()
  const sheet = useResource(`purse:${master}:${characterId}`, (signal) => master ? getSharedSheet(characterId, signal) : getSheet(characterId, signal))
  const [draft, setDraft] = useState<Record<string, number>>({})
  const purse = sheet.data?.equipment.purse ?? {}
  const changed = COINS.filter((coin) => draft[coin] !== undefined && draft[coin] !== (purse[coin] ?? 0))
  const save = useAction(async () => {
    if (!master) return writeChanges(characterId, changed.map((coin) => setCoin(coin, draft[coin] ?? 0)))
    for (const coin of changed) await adjustCoins(gameId, entry.id, coin, (draft[coin] ?? 0) - (purse[coin] ?? 0))
  })
  const error = save.error ?? sheet.error
  return <ModalSheet opened onClose={onClose} size="sm" title={t('game.coinsOf', { name: entry.name || t('common.unnamed') })}
    onSubmit={() => void save.run().then((result) => { if (result !== null) onClose(); else sheet.refresh() })}>
    <Stack gap="sm">
      {error !== null && <Alert color="red" title={t('group.actionFailed')}>{error}</Alert>}
      {COINS.map((coin) => (
        <Group key={coin} justify="space-between" wrap="nowrap">
          <Text size="sm">{t(`equipment.coin.${coin}`)}</Text>
          <NumberInput aria-label={t(`equipment.coin.${coin}`)} w={140} min={0} allowDecimal={false}
            disabled={sheet.data === null || save.pending} value={draft[coin] ?? purse[coin] ?? 0}
            onChange={(value) => setDraft((previous) => ({ ...previous, [coin]: Math.max(0, Math.trunc(Number(value)) || 0) }))} />
        </Group>
      ))}
      <Group justify="flex-end">
        <Button variant="default" onClick={onClose}>{t('common.cancel')}</Button>
        <Button type="submit" disabled={changed.length === 0} loading={save.pending}>{t('common.save')}</Button>
      </Group>
    </Stack>
  </ModalSheet>
}

/**
 * A player's own items over the game, as one of two lists that do one thing
 * each. Without `to` it is what is used up, with Use on every row. With `to`
 * it is everything carried that the catalogue knows, with Transfer on every
 * row -- opened from the *receiver's* card, so who it goes to was already said
 * by where it was pressed. Either way one press is the whole errand: it
 * closes the dialog and the tracker says what happened. `from` is the player's own seated
 * characters to take from, never the receiver itself; a transfer always names
 * its source in a "From" field, a choice when there is more than one.
 * Neither shows coins: those are their own dialog.
 */
function ItemsSheet({ gameId, from, to, onClose, onDone }: {
  gameId: string; from: readonly GameEntry[]; to?: GameEntry; onClose: () => void; onDone: (text: string) => void
}) {
  const t = useT()
  const locale = useLocale()
  const [chosen, setChosen] = useState<string | null>(null)
  const giver = from.find((each) => each.id === chosen) ?? from[0]
  const characterId = giver?.character_id ?? ''
  const sheet = useResource(`items:${locale}:${characterId}`, (signal) => getSheet(characterId, signal))
  const use = useAction((changes: Change[]) => writeChanges(characterId, changes))
  const give = useAction(giveItem)
  const s = sheet.data
  const items = bySlug<Item>([...(s?.catalog?.magicItems ?? []), ...(s?.catalog?.equipment ?? [])])
  const named = (slug: string) => s?.catalogNames?.[`equipment:${slug}`] ?? items.get(slug)?.name ?? titleCase(slug)
  const who = (each: GameEntry | undefined) => each?.name || t('common.unnamed')
  // Only what is carried and not worn, and only what the catalogue knows: a custom item has no slug to move by.
  const rows = (s ? mergeStacks(s.equipment) : []).filter((row) => row.item !== undefined && row.count > row.equipped
    && (to !== undefined || groupOf(items.get(row.item)) === 'consumable'))
  const pending = use.pending || give.pending
  const error = use.error ?? give.error ?? sheet.error
  const label = to === undefined ? t('equipment.use') : t('game.transfer')
  async function act(slug: string, name: string, total: number) {
    if (s === null || giver === undefined) return
    if (to === undefined) {
      if (await use.run(setTotal(s.equipment, slug, total - 1)) !== null) onDone(t('game.usedItem', { item: name }))
    } else if (await give.run(gameId, giver.id, to.id, slug) !== null) onDone(t('game.gave', { item: name, name: who(to) }))
  }
  return <ModalSheet opened onClose={onClose} size="lg"
    title={to === undefined ? t('game.useItemOf', { name: who(giver) }) : t('game.transferItemOf', { name: who(to) })}>
    <Stack gap="sm">
      {error !== null && <Alert color="red" title={t('group.actionFailed')}>{error}</Alert>}
      {to !== undefined && giver !== undefined && <Select label={t('game.transferFrom')} comboboxProps={SHEET_COMBOBOX} allowDeselect={false}
        value={giver.id} onChange={setChosen} data={from.map((each) => ({ value: each.id, label: who(each) }))} />}
      {s && rows.length === 0 && <Text size="sm" c="dimmed">{t('sheet.empty')}</Text>}
      {s && rows.map((row) => {
        const slug = row.item ?? ''
        const name = named(slug)
        const carried = row.count - row.equipped
        return <Card key={row.key} withBorder radius="md" padding="xs">
          <Group gap="sm" wrap="nowrap">
            <ItemIcon icon={items.get(slug)?.icon} />
            <Group gap={8} style={{ flex: 1, minWidth: 0 }}>
              <Anchor component={Link} size="sm" fw={600} style={{ overflowWrap: 'anywhere' }}
                to={`/games/${encodeURIComponent(gameId)}/characters/${encodeURIComponent(characterId)}/items/${encodeURIComponent(slug)}`}>{name}</Anchor>
              {carried > 1 && <Text size="sm" c="dimmed">×{carried}</Text>}
            </Group>
            <Button variant="light" disabled={pending} style={{ flexShrink: 0 }}
              aria-label={t('list.rowAction', { label, name })} onClick={() => void act(slug, name, row.count)}>{label}</Button>
          </Group>
        </Card>
      })}
    </Stack>
  </ModalSheet>
}

function CompactStats({ entry, expanded, detailsId }: { entry: GameEntry; expanded: boolean; detailsId: string }) {
  const t = useT()
  const desktop = useIsDesktop()
  const stats = entry.stats
  if (!stats) return null
  const values = [
    [t('game.hp'), `${entry.hp} / ${stats.max_hp}`],
    [t('game.tempHp'), entry.temp_hp ?? 0],
    [t('game.ac'), stats.armor_class],
    // A class is named only to tell two DCs apart; one caster's number needs no label.
    [t('game.spellDc'), (stats.spellcasting ?? []).map((caster, _, all) => all.length > 1 ? `${titleCase(caster.class)} ${caster.saveDC}` : caster.saveDC).join(' · ') || '—'],
    [t('vitals.speed'), (stats.speeds ?? []).map((speed) => `${speedName(t, speed.kind)} ${t('vitals.feet', { distance: speed.distance })}`).join(' · ') || '—'],
    [t('vitals.vision'), (stats.senses ?? []).map((sense) => `${senseName(t, sense.kind)} ${t('vitals.feet', { distance: sense.distance })}`).join(' · ') || t('vitals.normalVision')],
    [t('vitals.initiative'), entry.initiative ?? '—'],
  ]
  const field = ([label, value]: typeof values[number]) => (
    <Stack key={label} gap={0} style={{ minWidth: 0 }}>
      <Text size="xs" c="dimmed" style={{ overflowWrap: 'anywhere' }}>{label}</Text>
      <Text size="sm" fw={500} style={{ overflowWrap: 'anywhere' }}>{value}</Text>
    </Stack>
  )
  const abilities = <SimpleGrid cols={STAT_COLUMNS} spacing="sm" verticalSpacing="xs">
    {ABILITY_ORDER.map((ability) => (
      <Stack key={ability} gap={0}>
        <Text size="xs" c="dimmed">{abilityAbbr(t, ability)}</Text>
        <Text size="sm" fw={500}>
          {stats.abilities.scores[ability] ?? 10} ({signed(stats.abilities.modifiers[ability] ?? 0)})
        </Text>
      </Stack>
    ))}
  </SimpleGrid>
  return <>
    <Divider />
    <SimpleGrid cols={desktop ? STAT_COLUMNS : 5} spacing={desktop ? 'sm' : 'xs'} verticalSpacing="xs">
      {(desktop ? values : [
        // What is asked mid-turn, and five of them across a phone: current hit points without the maximum, and initiative as a letter.
        [t('game.hp'), entry.hp ?? '—'], values[1]!, values[2]!, [t('game.initiativeShort'), entry.initiative ?? '—'],
        [t('game.spellDc'), (stats.spellcasting ?? []).map((caster) => caster.saveDC).join(' · ') || '—'],
      ] satisfies typeof values).map(field)}
    </SimpleGrid>
    {desktop ? abilities : expanded && <Stack gap="xs" id={detailsId}>
        <SimpleGrid cols={2} spacing="xs">{values.slice(4, 6).map(field)}</SimpleGrid>
        {abilities}
      </Stack>}
  </>
}

/** Tags are game values, edited directly in the row, independent of dialog drafts. */
function EntryTags({ gameId, entry, onChange }: { gameId: string; entry: GameEntry; onChange: () => void }) {
  const t = useT()
  const save = useAction(patchGameEntry)
  const incoming = JSON.stringify(entry.tags ?? [])
  const [observed, setObserved] = useState(incoming)
  const [tags, setTags] = useState(entry.tags ?? [])
  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState('')
  const [invalid, setInvalid] = useState(false)
  if (observed !== incoming) {
    setObserved(incoming)
    setTags(entry.tags ?? [])
  }
  async function update(next: string[]) {
    if (!entry.can_edit || save.pending) return false
    const result = await save.run(gameId, entry.id, { tags: next })
    if (result === null) return false
    setTags(result.entries.find((item) => item.id === entry.id)?.tags ?? next)
    onChange()
    return true
  }
  async function add() {
    const tag = draft.trim()
    if (!tag || tag.length > 100 || tags.length >= 20) { setInvalid(true); return }
    if (tags.includes(tag) || await update([...tags, tag])) {
      setDraft(''); setAdding(false); setInvalid(false)
    }
  }
  if (tags.length === 0 && !entry.can_edit && !adding && save.error === null) return null
  return <Stack gap="xs">
    {(tags.length > 0 || (entry.can_edit && !adding)) && <Group gap="xs" role="group" aria-label={t('game.tags')}>
      {tags.map((tag) => entry.can_edit ? (
        <Button key={tag} size="xs" variant="light" disabled={save.pending}
          styles={{ root: { height: 'auto', minHeight: 28, maxWidth: '100%', paddingBlock: 4 }, label: { whiteSpace: 'normal', overflowWrap: 'anywhere', fontSize: 14, lineHeight: '20px' } }}
          aria-label={t('game.removeTag', { tag })} onClick={() => void update(tags.filter((value) => value !== tag))}>{tag} ×</Button>
      ) : <Badge key={tag} variant="light" size="lg"
        styles={{ root: { textTransform: 'none', height: 'auto', minHeight: 28, maxWidth: '100%', paddingBlock: 4 }, label: { whiteSpace: 'normal', overflowWrap: 'anywhere', fontSize: 14, lineHeight: '20px' } }}>{tag}</Badge>)}
      {entry.can_edit && !adding && <Button variant="subtle" size="xs" h={28} disabled={save.pending}
        leftSection={<IconPlus size={14} />} onClick={() => { save.reset(); setAdding(true) }}>{t('game.addTag')}</Button>}
    </Group>}
    {adding && <Group gap="xs" align="flex-start" wrap="nowrap">
      <TextInput autoFocus size="sm" aria-label={t('game.tags')} placeholder={t('game.tags')} value={draft}
        style={{ flex: 1, minWidth: 0 }} disabled={!entry.can_edit || save.pending}
        error={invalid ? t('game.tagInvalid') : undefined}
        onChange={(event) => { setDraft(event.currentTarget.value); setInvalid(false) }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') { event.preventDefault(); void add() }
          if (event.key === 'Escape') { setAdding(false); setDraft(''); setInvalid(false) }
        }} />
      <ActionIcon size={36} variant="light" aria-label={t('game.addTag')} disabled={!entry.can_edit || save.pending || !draft.trim()}
        onClick={() => void add()}><IconPlus size={ACTION_ICON_SIZE} /></ActionIcon>
      <Button variant="subtle" size="sm" h={36} disabled={save.pending} onClick={() => { setAdding(false); setDraft(''); setInvalid(false) }}>{t('common.cancel')}</Button>
    </Group>}
    {save.error && <Alert color="red">{save.error}</Alert>}
  </Stack>
}

/**
 * One number, on its own: where an entry acts in the round. Emptied, it is
 * cleared -- the entry has not rolled yet.
 */
function InitiativeSheet({ entry, pending, error, onClose, onSave }: {
  entry: GameEntry; pending: boolean; error: string | null; onClose: () => void; onSave: (initiative: number | null) => Promise<void>
}) {
  const t = useT()
  const [initiative, setInitiative] = useState<number | string>(entry.initiative ?? '')
  const valid = initiative === '' || (typeof initiative === 'number' && Number.isInteger(initiative))
  return <ModalSheet opened onClose={onClose} size="xs" title={entry.name || t('common.unnamed')}
    onSubmit={() => { if (valid && entry.can_edit && !pending) void onSave(initiative === '' ? null : Number(initiative)) }}>
    <Stack gap="sm">
      {error && <Alert color="red">{error}</Alert>}
      {!entry.can_edit && <Alert color="yellow">{t('game.editLocked')}</Alert>}
      <NumberInput label={t('vitals.initiative')} allowDecimal={false} value={initiative} onChange={setInitiative} disabled={!entry.can_edit} data-autofocus />
      <Group justify="flex-end">
        <Button type="submit" loading={pending} disabled={!valid || !entry.can_edit}>{t('game.apply')}</Button>
      </Group>
    </Stack>
  </ModalSheet>
}

/** Drafts belong to this editor; background refreshes never replace typed values. */
function EntryEditor({ entry, pending, error, onClose, onSave }: {
  entry: GameEntry; pending: boolean; error: string | null; onClose: () => void; onSave: (patch: EntryPatch) => Promise<void>
}) {
  const t = useT()
  const [original] = useState(entry)
  const [hp, setHP] = useState<number | string>(entry.hp ?? 0)
  const [tempHP, setTempHP] = useState<number | string>(entry.temp_hp ?? 0)
  const [damage, setDamage] = useState<number | string>('')
  const [stats, setStats] = useState<EntryStats>(structuredClone(entry.stats!))
  const baseValid = typeof hp === 'number' && Number.isInteger(hp) && hp >= 0 && typeof tempHP === 'number' && Number.isInteger(tempHP) && tempHP >= 0
  const amount = damage === '' ? 0 : Number(damage)
  const damageValid = Number.isSafeInteger(amount) && amount >= 0
  const nextTempHP = baseValid && damageValid ? Math.max(0, Number(tempHP) - amount) : tempHP
  const nextHP = baseValid && damageValid ? Math.max(0, Number(hp) - Math.max(0, amount - Number(tempHP))) : hp
  const valid = baseValid && damageValid
  const monster = entry.kind === 'monster'
  async function submit() {
    if (!valid || !entry.can_edit || pending) return
    const patch: EntryPatch = {}
    if (nextHP !== original.hp) patch.hp = Number(nextHP)
    if (nextTempHP !== original.temp_hp) patch.temp_hp = Number(nextTempHP)
    if (monster && JSON.stringify(stats) !== JSON.stringify(original.stats)) {
      const changed: Partial<EntryStats> = {}
      for (const key of ['name', 'max_hp', 'armor_class', 'spellcasting', 'speeds', 'senses', 'abilities'] as const) {
        if (JSON.stringify(stats[key]) !== JSON.stringify(original.stats?.[key])) Object.assign(changed, { [key]: stats[key] })
      }
      patch.stats = changed
    }
    await onSave(patch)
  }
  return <ModalSheet opened onClose={onClose} title={entry.name || t('common.unnamed')} onSubmit={() => void submit()}>
    <Stack gap="sm">
      {error && <Alert color="red">{error}</Alert>}
      {!entry.can_edit && <Alert color="yellow">{t('game.editLocked')}</Alert>}
      {monster && <TextInput label={t('common.name')} value={stats.name} maxLength={64} disabled={!entry.can_edit}
        onChange={(event) => setStats({ ...stats, name: event.currentTarget.value })} />}
      <SimpleGrid cols={{ base: 1, sm: 2 }}>
        <NumberInput label={t('vitals.hitPoints')} min={0} allowDecimal={false} value={nextHP} onChange={(value) => { setHP(value); setTempHP(nextTempHP); setDamage('') }} disabled={!entry.can_edit} />
        <NumberInput label={t('vitals.tempHp')} min={0} allowDecimal={false} value={nextTempHP} onChange={(value) => { setTempHP(value); setHP(nextHP); setDamage('') }} disabled={!entry.can_edit} />
      </SimpleGrid>
      <NumberInput label={t('game.damage')} description={t('game.damageHint')} min={0} allowNegative={false}
        allowDecimal={false} value={damage} onChange={setDamage} disabled={!entry.can_edit || !baseValid} />
      {monster && <MonsterStatsEditor stats={stats} onChange={setStats} />}
      <Group justify="flex-end">
        <Button type="submit" loading={pending} disabled={!valid || !entry.can_edit}>{t('game.apply')}</Button>
      </Group>
    </Stack>
  </ModalSheet>
}

function MonsterStatsEditor({ stats, onChange }: { stats: EntryStats; onChange: (stats: EntryStats) => void }) {
  const t = useT()
  const integer = (value: string | number) => Number.isFinite(Number(value)) ? Math.trunc(Number(value)) : 0
  const spellDC = Math.max(0, ...(stats.spellcasting ?? []).map((caster) => caster.saveDC))
  return <Stack gap="sm">
    <SimpleGrid cols={2}>
      <NumberInput label={t('game.maxHp')} min={0} allowDecimal={false} value={stats.max_hp} onChange={(value) => onChange({ ...stats, max_hp: integer(value) })} />
      <NumberInput label={t('vitals.armorClass')} min={0} allowDecimal={false} value={stats.armor_class} onChange={(value) => onChange({ ...stats, armor_class: integer(value) })} />
      {ABILITY_ORDER.map((ability) => <NumberInput key={ability} label={abilityAbbr(t, ability)} min={1} max={30} allowDecimal={false}
        value={stats.abilities.scores[ability] ?? 10} onChange={(value) => onChange({ ...stats, abilities: { ...stats.abilities, scores: { ...stats.abilities.scores, [ability]: integer(value) } } })} />)}
      <NumberInput label={speedName(t, 'walking')} min={0} allowDecimal={false}
        value={(stats.speeds ?? []).find((speed) => speed.kind === 'walking')?.distance ?? 0}
        onChange={(value) => onChange({ ...stats, speeds: [
          ...(stats.speeds ?? []).filter((speed) => speed.kind !== 'walking'),
          { kind: 'walking', distance: integer(value) },
        ] })} />
      <NumberInput label={t('vitals.spellSaveDc')} allowDecimal={false} min={0} value={spellDC || ''}
        onChange={(value) => {
          const saveDC = integer(value)
          const casting = stats.spellcasting?.length
            ? stats.spellcasting.map((caster) => ({ ...caster, saveDC }))
            : [{ class: '', ability: '', saveDC, attackBonus: 0 }]
          onChange({ ...stats, spellcasting: saveDC > 0 ? casting : [] })
        }} />
    </SimpleGrid>
  </Stack>
}

/**
 * One entry's consumables, in a dialog of their own. A press is drawn at once
 * from a local count and saved behind it, with its own action: sharing the
 * roster's would grey every control on the page for the length of a request.
 */
function ConsumablesSheet({ gameId, entry, onClose, onChange }: {
  gameId: string; entry: GameEntry; onClose: () => void; onChange: () => void
}) {
  const t = useT()
  const pools = (entry.resources ?? []).map(({ slot_level, ...pool }) => ({ ...pool, ...(slot_level ? { slotLevel: slot_level } : {}) }))
  const save = useAction(patchGameEntry)
  const [local, setLocal] = useState<Record<string, number>>({})
  // A local count has done its job once the server reports the same one; kept
  // longer it would hide a long rest called while the dialog is open.
  const settled = pools.filter((pool) => local[pool.id] === pool.used)
  if (settled.length > 0) setLocal(Object.fromEntries(Object.entries(local).filter(([id]) => !settled.some((pool) => pool.id === id))))
  const forget = (id: string) => setLocal((previous) => Object.fromEntries(Object.entries(previous).filter(([key]) => key !== id)))
  async function spend(id: string, used: number) {
    setLocal((previous) => ({ ...previous, [id]: used }))
    if (await save.run(gameId, entry.id, { used: { [id]: used } }) === null) forget(id)
    else onChange()
  }
  return <ModalSheet opened onClose={onClose} title={t('sheet.consumables')}>
    <Stack gap="sm">
      {save.error !== null && <Alert color="red" title={t('group.actionFailed')}>{save.error}</Alert>}
      <ResourcePools pools={pools.map((pool) => ({ ...pool, used: local[pool.id] ?? pool.used }))}
        {...(entry.can_edit ? { onChange: (id: string, used: number) => void spend(id, used) } : {})} />
    </Stack>
  </ModalSheet>
}
