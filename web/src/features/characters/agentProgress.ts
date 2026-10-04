import type { Translate } from '@/lib/i18n'
import { titleCase } from '@/domain'
const fields = {
  'identity.name': 'agent.field.name',
  race: 'agent.field.race',
  subrace: 'agent.field.subrace',
  class: 'agent.field.class',
  subclass: 'agent.field.subclass',
  background: 'agent.field.background',
  spell: 'agent.field.spell',
  cantrip: 'agent.field.cantrip',
  item: 'agent.field.item',
  equipment: 'agent.field.item',
  choice: 'agent.field.choice',
  definitions: 'agent.field.definitions',
  feature: 'agent.field.feature',
  feat: 'agent.field.feat',
  trait: 'agent.field.trait',
  'base.languages': 'agent.field.languages',
} as const
export function progressText(t: Translate, data: unknown): string | null {
  if (
    !data ||
    typeof data !== 'object' ||
    !('field' in data) ||
    !('value' in data) ||
    !('operation' in data)
  )
    return null
  const field = String(data.field)
  const key =
    fields[field as keyof typeof fields] ??
    (field.startsWith('finalAbilities.') || field.startsWith('abilities.')
      ? 'agent.field.scores'
      : field.includes('hitPoints')
        ? 'agent.field.hp'
        : field.startsWith('equipment.')
          ? 'agent.field.inventory'
          : 'agent.field.details')
  const ability =
    field.startsWith('finalAbilities.') || field.startsWith('abilities.')
      ? field.split('.')[1]?.toUpperCase()
      : undefined
  const label = ability ? `${t(key)} (${ability})` : t(key)
  const detail =
    field.startsWith('equipment.') ||
    field.startsWith('skills.') ||
    field.startsWith('savingThrows.')
      ? ` (${field.split('.').slice(1).map(titleCase).join(' · ')})`
      : ''
  const raw = String(data.value)
  const value =
    (raw.length > 100 ? raw.slice(0, 97) + '…' : raw) +
    ('level' in data ? `, ${t('agent.progress.level', { level: data.level })}` : '')
  const operation =
    data.operation === 'custom'
      ? 'agent.progress.custom'
      : data.operation === 'updated'
        ? 'agent.progress.updated'
        : 'agent.progress.imported'
  return t(operation, { field: label + detail, value })
}
