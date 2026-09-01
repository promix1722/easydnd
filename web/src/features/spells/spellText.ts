import type { RuleValue, SpellComponents } from '@/lib/api'
import type { Translate } from '@/lib/i18n'

/**
 * Rule strings, rendered back into words.
 *
 * The compendium stores casting time, range and duration structured --
 * `{kind: "distance", distance: 90}` -- precisely so that the client can say
 * "90 feet" in the reader's language instead of shipping English prose. This
 * module is that rendering.
 *
 * It lives here and not in `src/domain/` for the same reason
 * `features/character/labels.ts` moved out: `domain/` holds no prose and may
 * not import `@/lib`, and these functions want the typed `Translate` so every
 * key is compile-checked. They take `t` as an argument rather than calling
 * the hook, so they stay pure and testable without a provider.
 *
 * Every switch falls back to the raw kind string: a kind this client has not
 * heard of yet renders as itself rather than as an error.
 */

function unitText(t: Translate, unit: string | undefined, amount: number): string {
  switch (unit) {
    case 'round':
      return t('spell.unit.round', { count: amount })
    case 'minute':
      return t('spell.unit.minute', { count: amount })
    case 'hour':
      return t('spell.unit.hour', { count: amount })
    case 'day':
      return t('spell.unit.day', { count: amount })
    default:
      return `${amount} ${unit ?? ''}`.trim()
  }
}

export function castingTimeText(t: Translate, value: RuleValue | undefined): string {
  if (!value) return ''
  switch (value.kind) {
    case 'action':
      return t('spell.time.action')
    case 'bonus-action':
      return t('spell.time.bonusAction')
    case 'reaction':
      return t('spell.time.reaction')
    case 'over-time':
      return unitText(t, value.unit, value.amount ?? 0)
    default:
      return value.kind
  }
}

export function rangeText(t: Translate, value: RuleValue | undefined): string {
  if (!value) return ''
  switch (value.kind) {
    case 'self':
      return t('spell.range.self')
    case 'touch':
      return t('spell.range.touch')
    case 'distance':
      return t('spell.range.feet', { count: value.distance ?? 0 })
    case 'sight':
      return t('spell.range.sight')
    case 'unlimited':
      return t('spell.range.unlimited')
    case 'special':
      return t('spell.range.special')
    default:
      return value.kind
  }
}

export function durationText(t: Translate, value: RuleValue | undefined): string {
  if (!value) return ''
  switch (value.kind) {
    case 'instantaneous':
      return t('spell.duration.instantaneous')
    case 'timed': {
      const span = unitText(t, value.unit, value.amount ?? 0)
      return value.upTo ? t('spell.duration.upTo', { value: span }) : span
    }
    case 'until-dispelled':
      return t('spell.duration.untilDispelled')
    case 'special':
      return t('spell.duration.special')
    default:
      return value.kind
  }
}

/** "V, S, M" -- the compact form both list rows and the detail's facts use. */
export function componentsAbbrev(t: Translate, components: SpellComponents | undefined): string {
  if (!components) return ''
  const parts: string[] = []
  if (components.verbal) parts.push(t('spell.components.v'))
  if (components.somatic) parts.push(t('spell.components.s'))
  if (components.material) parts.push(t('spell.components.m'))
  return parts.join(', ')
}

/** "Cantrip" or "Level 3". */
export function levelText(t: Translate, level: number): string {
  return level === 0 ? t('spell.level.cantrip') : t('spell.level.numbered', { level })
}
