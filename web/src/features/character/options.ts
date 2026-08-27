import type { Entry, Option, Prompt } from '@/lib/api'
import { slugOf, titleCase } from '@/domain'
import type { Translate } from '@/lib/i18n'

import { abilityName } from './labels'

/**
 * One option as a prompt renders it: a key, a label, and why it may be off.
 *
 * The key is always the server's. A bundle of a shortbow and twenty arrows
 * has no slug of its own, and the rule for naming one lives on the server
 * precisely so that the client cannot get it wrong.
 */
export interface Choosable {
  key: string
  label: string
  detail?: string
  disabled: boolean
  reason?: string
}

/**
 * Whether a prompt offers anything to pick between.
 *
 * Two questions arrive with the kind `ability-scores`: the improvement a level
 * grants, which offers "raise two scores" or "take a feat", and the six a
 * character starts with, which offers nothing because it is six numbers rather
 * than a choice of N. This is what tells them apart, and it asks the option set
 * rather than the prompt's slug so that the answer comes from the server's own
 * statement of what may be picked.
 */
export function offersOptions(prompt: Prompt): boolean {
  const set = prompt.choice.from
  if ((set.options ?? []).length > 0) return true
  return set.collection !== undefined || set.category !== undefined
}

/**
 * Turns a prompt's options into the buttons the card draws.
 *
 * An option's key is always the server's, never computed here. A bundle of a
 * shortbow and twenty arrows has no slug of its own, and the rule for naming
 * one lives on the server precisely so that the client cannot get it wrong.
 */
export function choosableOptions(
  t: Translate,
  prompt: Prompt,
  entries: Map<string, Entry>,
): Choosable[] {
  const held = new Set(prompt.held ?? [])
  const set = prompt.choice.from

  // A set drawn from a collection lists nothing inline: the whole collection
  // is the option list, and an entry's own slug is its key.
  if (set.kind !== 'explicit') {
    return [...entries.values()].map((entry) => ({
      key: entry.slug,
      label: entry.name,
      disabled: disabledBy(t, prompt, held, entry.slug) !== undefined,
      ...maybeReason(disabledBy(t, prompt, held, entry.slug)),
    }))
  }

  return (set.options ?? []).map((option) => {
    const reason = disabledBy(t, prompt, held, option.key)
    return {
      key: option.key,
      label: labelOf(t, option, entries),
      ...maybeDetail(detailOf(option, entries)),
      disabled: reason !== undefined,
      ...maybeReason(reason),
    }
  })
}

function maybeReason(reason: string | undefined): { reason?: string } {
  return reason === undefined ? {} : { reason }
}

function maybeDetail(detail: string | undefined): { detail?: string } {
  return detail === undefined ? {} : { detail }
}

/**
 * Why an option cannot be picked, or undefined.
 *
 * `heldOnly` inverts the test. Expertise doubles a proficiency the character
 * already has, so there holding a skill is what makes it pickable; everywhere
 * else holding it means picking it would be wasted.
 */
function disabledBy(
  t: Translate,
  prompt: Prompt,
  held: Set<string>,
  key: string,
): string | undefined {
  if (prompt.heldOnly) return held.has(key) ? undefined : t('option.notProficient')
  return held.has(key) ? t('option.alreadyHave') : undefined
}

function labelOf(t: Translate, option: Option, entries: Map<string, Entry>): string {
  switch (option.kind) {
    case 'ref': {
      const slug = option.ref === undefined ? '' : slugOf(option.ref)
      const entry = entries.get(slug)
      const name = entry?.name ?? titleCase(slug)
      return option.count !== undefined && option.count > 1 ? `${name} ×${option.count}` : name
    }
    case 'nested':
      return option.choice
        ? t('option.chooseN', { count: option.choice.choose })
        : t('option.choose')
    case 'bundle':
      // Joined with the catalogue's own word for "and": a bundle of a shortbow
      // and twenty arrows is one option, and the conjunction between them is
      // prose like any other.
      return (option.items ?? [])
        .map((item) => labelOf(t, item, entries))
        .join(t('option.bundleJoin'))
    case 'ability-bonus':
      return `${abilityName(t, option.ability ?? '')} +${option.bonus ?? 1}`
    case 'text':
      return option.text ?? option.key
    case 'money':
      return option.cost ? `${option.cost.amount} ${option.cost.unit}` : t('option.coin')
    case 'damage':
      return option.text ?? option.damage?.type ?? t('option.damage')
    case 'size':
      return titleCase(option.size ?? '')
    case 'action':
      return option.text ?? option.key
    case 'score-minimum':
      return `${abilityName(t, option.ability ?? '')} ${option.minimum ?? 0}+`
    default:
      return option.key
  }
}

/**
 * What an option says about itself, whole.
 *
 * It used to be cut to 120 characters, because it was drawn on the same line
 * as the name and something had to give. It is drawn underneath now -- and
 * only under the option that was picked -- so there is room for the sentence
 * the compendium actually wrote, and a description that stops mid-word is
 * worse than one that takes three lines.
 */
function detailOf(option: Option, entries: Map<string, Entry>): string | undefined {
  if (option.kind === 'ref') {
    const slug = option.ref === undefined ? '' : slugOf(option.ref)
    return entries.get(slug)?.desc?.[0]
  }
  return undefined
}
