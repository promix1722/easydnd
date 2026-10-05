import { useId, useRef, useState, type PointerEvent } from 'react'
import { Link } from 'react-router'

import type { EntryPatch, EntryStats, GameDetail, GameEntry } from '@/lib/api'
import { addGameMonster, deleteGameEntry, orderGameEntries, patchGameEntry, restGame } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import {
  ACTION_ICON_SIZE, ActionIcon, Alert, Anchor, Badge, Box, Button, Card, Divider,
  Group, IconArrowDown, IconArrowUp, IconDice5, IconDotsVertical, IconGripVertical, IconPencil,
  IconShield, IconTrash, IconPlus, IconChevronDown, Menu, ModalSheet, NumberInput, SimpleGrid, Stack, Text, TextInput, useIsDesktop,
} from '@/ui'
import { ABILITY_ORDER, signed, titleCase } from '@/domain'
import { abilityAbbr, senseName, speedName } from '../character/labels'
import { ResourcePools } from '../character/ResourcePools'
import { FolderTreeSheet } from './FolderTreeSheet'

const STAT_COLUMNS = { base: 2, sm: 4, md: 7 } as const

/** One ordered list shared by the table; all private fields are filtered upstream. */
export function GameTracker({ game, onChange, onAddFromGroup }: {
  game: GameDetail
  onChange: () => void
  onAddFromGroup: () => void
}) {
  const t = useT()
  const desktop = useIsDesktop()
  const detailsPrefix = useId()
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const master = game.role === 'owner' || game.role === 'dm'
  const [dragging, setDragging] = useState<string | null>(null)
  const gesture = useRef<{ id: string; pointer: number; x: number; y: number; moved: boolean } | null>(null)
  const [over, setOver] = useState<string | null>(null)
  const [editing, setEditing] = useState<string | null>(null)
  const [consuming, setConsuming] = useState<string | null>(null)
  const [pickingMonster, setPickingMonster] = useState(false)
  const patch = useAction(patchGameEntry)
  const remove = useAction(deleteGameEntry)
  const order = useAction(orderGameEntries)
  const monster = useAction(addGameMonster)
  const rest = useAction(restGame)
  const [resting, setResting] = useState(false)
  const entries = game.entries
  const selected = entries.find((entry) => entry.id === editing)
  const consumer = entries.find((entry) => entry.id === consuming)
  const error = patch.error ?? remove.error ?? order.error ?? monster.error ?? rest.error
  const pending = patch.pending || remove.pending || order.pending || monster.pending || rest.pending

  async function act(work: Promise<unknown | null>) {
    if (await work !== null) onChange()
  }

  const draggable = master && !pending && entries.length > 1

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
        <Group justify="space-between" align="flex-start">
          <Group gap="xs">
            <Button variant="light" leftSection={<IconPlus size={ACTION_ICON_SIZE} />} disabled={pending}
              onClick={onAddFromGroup}>{t('game.addFromGroup')}</Button>
            <Button variant="light" disabled={pending} onClick={() => setPickingMonster(true)}>{t('game.addMonsterFromMine')}</Button>
            <Button variant="light" disabled={pending} onClick={() => void act(monster.run(game.id))}>{t('game.addMonsterStub')}</Button>
          </Group>
          <Group gap="xs">
            <Button variant="subtle" disabled={pending} onClick={() => setResting(true)}>{t('game.longRest')}</Button>
            <Button variant="subtle" leftSection={<IconArrowDown size={ACTION_ICON_SIZE} />}
              disabled={pending || entries.length < 2}
              onClick={() => void act(order.run(game.id, { by_initiative: true }))}>
              {t('game.orderInitiative')}
            </Button>
          </Group>
        </Group>
      )}
      {entries.length === 0 && <Text c="dimmed">{t('game.empty')}</Text>}
      <Stack gap="xs" data-game-roster={game.id}>
        {entries.map((entry, index) => {
          const name = entry.name || t('common.unnamed')
          const actions = [
            ...(entry.can_edit ? [{ label: t('common.edit'), icon: IconPencil,
              run: () => { patch.reset(); setEditing(entry.id) } }] : []),
            // Offered to everyone who can see the entry: without edit rights the dialog is read-only.
            ...((entry.resources ?? []).length > 0 ? [{ label: t('sheet.consumables'), icon: IconDice5,
              run: () => setConsuming(entry.id) }] : []),
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
                    {!desktop && entry.stats && entry.initiative != null && <Text component="span" size="sm" fw={500}
                      aria-label={t('vitals.initiative')} title={t('vitals.initiative')}
                      style={{ flexShrink: 0, fontVariantNumeric: 'tabular-nums' }}>({entry.initiative})</Text>}
                    <Stack gap={0} style={{ flex: 1, minWidth: 0 }}>
                      <Text size="xs" c="dimmed" visibleFrom="md">{t('common.name')}</Text>
                      <Group gap="xs">
                        {entry.character_id ? (
                          <Anchor component={Link} draggable={false} size="sm" fw={500}
                            style={{ overflowWrap: 'anywhere' }}
                            to={`/groups/${game.group_id}/characters/${entry.character_id}`}>{name}</Anchor>
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
        {master && entries.length > 1 && (
          <Box h={16} aria-hidden data-game-drop-end>
            <Box h={2} bg={dragging !== null && over === '' ? 'var(--mantine-primary-color-filled)' : 'transparent'} />
          </Box>
        )}
      </Stack>
      {consumer && <ConsumablesSheet gameId={game.id} entry={consumer} onClose={() => setConsuming(null)} onChange={onChange} />}
      {selected && <EntryEditor key={selected.id} entry={selected} pending={patch.pending} error={patch.error}
        onClose={() => setEditing(null)} onSave={async (changes) => {
          if (await patch.run(game.id, selected.id, changes) === null) return
          setEditing(null)
          onChange()
        }} />}
      <ModalSheet opened={resting} onClose={() => setResting(false)} title={t('game.longRest')}
        onSubmit={() => { setResting(false); void act(rest.run(game.id)) }}>
        <Stack gap="sm">
          <Text size="sm">{t('game.longRestHint')}</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setResting(false)}>{t('common.cancel')}</Button>
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
    <SimpleGrid cols={desktop ? STAT_COLUMNS : 4} spacing={desktop ? 'sm' : 'xs'} verticalSpacing="xs">
      {(desktop ? values : [values[0]!, values[1]!, values[2]!, [t('game.spellDc'), (stats.spellcasting ?? []).map((caster) => caster.saveDC).join(' · ') || '—']]).map(field)}
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

/** Drafts belong to this editor; background refreshes never replace typed values. */
function EntryEditor({ entry, pending, error, onClose, onSave }: {
  entry: GameEntry; pending: boolean; error: string | null; onClose: () => void; onSave: (patch: EntryPatch) => Promise<void>
}) {
  const t = useT()
  const [original] = useState(entry)
  const [hp, setHP] = useState<number | string>(entry.hp ?? 0)
  const [tempHP, setTempHP] = useState<number | string>(entry.temp_hp ?? 0)
  const [damage, setDamage] = useState<number | string>('')
  const [initiative, setInitiative] = useState<number | string>(entry.initiative ?? '')
  const [stats, setStats] = useState<EntryStats>(structuredClone(entry.stats!))
  const baseValid = typeof hp === 'number' && Number.isInteger(hp) && hp >= 0 && typeof tempHP === 'number' && Number.isInteger(tempHP) && tempHP >= 0
  const amount = damage === '' ? 0 : Number(damage)
  const damageValid = Number.isSafeInteger(amount) && amount >= 0
  const nextTempHP = baseValid && damageValid ? Math.max(0, Number(tempHP) - amount) : tempHP
  const nextHP = baseValid && damageValid ? Math.max(0, Number(hp) - Math.max(0, amount - Number(tempHP))) : hp
  const valid = baseValid && damageValid && (initiative === '' || (typeof initiative === 'number' && Number.isInteger(initiative)))
  const monster = entry.kind === 'monster'
  async function submit() {
    if (!valid || !entry.can_edit || pending) return
    const patch: EntryPatch = {}
    if (nextHP !== original.hp) patch.hp = Number(nextHP)
    if (nextTempHP !== original.temp_hp) patch.temp_hp = Number(nextTempHP)
    if ((initiative === '' ? null : initiative) !== (original.initiative ?? null)) patch.initiative = initiative === '' ? null : Number(initiative)
    if (monster && JSON.stringify(stats) !== JSON.stringify(original.stats)) {
      const changed: Partial<EntryStats> = {}
      for (const key of ['name', 'max_hp', 'armor_class', 'spellcasting', 'speeds', 'senses', 'abilities'] as const) {
        if (JSON.stringify(stats[key]) !== JSON.stringify(original.stats?.[key])) Object.assign(changed, { [key]: stats[key] })
      }
      patch.stats = changed
    }
    await onSave(patch)
  }
  return <ModalSheet opened onClose={onClose} onSubmit={() => void submit()}>
    <Stack gap="sm">
      {error && <Alert color="red">{error}</Alert>}
      {!entry.can_edit && <Alert color="yellow">{t('game.editLocked')}</Alert>}
      {monster && <TextInput label={t('common.name')} value={stats.name} maxLength={64} disabled={!entry.can_edit}
        onChange={(event) => setStats({ ...stats, name: event.currentTarget.value })} />}
      <SimpleGrid cols={{ base: 1, sm: 3 }}>
        <NumberInput label={t('vitals.hitPoints')} min={0} allowDecimal={false} value={nextHP} onChange={(value) => { setHP(value); setTempHP(nextTempHP); setDamage('') }} disabled={!entry.can_edit} />
        <NumberInput label={t('vitals.tempHp')} min={0} allowDecimal={false} value={nextTempHP} onChange={(value) => { setTempHP(value); setHP(nextHP); setDamage('') }} disabled={!entry.can_edit} />
        <NumberInput label={t('vitals.initiative')} allowDecimal={false} value={initiative} onChange={setInitiative} disabled={!entry.can_edit} />
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
