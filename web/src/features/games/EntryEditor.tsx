import { useState } from 'react'

import type { EntryPatch, EntryStats, GameEntry } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Alert, Button, Group, ModalSheet, NumberInput, SimpleGrid, Stack, TextInput } from '@/ui'
import { ABILITY_ORDER } from '@/domain'
import { abilityAbbr, speedName } from '../character/labels'

/** Drafts belong to this editor; background refreshes never replace typed values. */
export function EntryEditor({ entry, pending, error, onClose, onSave }: {
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
