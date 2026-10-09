import type { Entry, Item, Option, Prompt } from '@/lib/api'
import { slugOf, titleCase } from '@/domain'
import type { Translate } from '@/lib/i18n'
import { joinProse } from '@/ui'

import { abilityName, choiceOptionName } from './labels'

/**
 * One option as a prompt renders it: a key, a label, and why it may be off.
 *
 * The key is always the server's. A bundle of a shortbow and twenty arrows
 * has no slug of its own -- it is named by what is in it, "shortbow+arrow" --
 * and the rule for composing that lives on the server precisely so that the
 * client cannot get it wrong.
 */
export interface Choosable {
  icon?: Item['icon']
  provenance?: Entry['provenance']
  /** An entry the player wrote, rather than one the rules have. */
  manual?: boolean
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
 * shortbow and twenty arrows has no slug of its own -- the server names it by
 * its contents -- and that rule lives there precisely so that the client
 * cannot get it wrong.
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
      icon: prompt.choice.kind === 'equipment' ? (entry as Item).icon : undefined,
      provenance: entry.provenance,
      ...(entry.manual === true ? { manual: true } : {}),
      label: entry.name,
      ...maybeDetail(entry.desc?.length ? joinProse(entry.desc) : undefined),
      disabled: disabledBy(t, prompt, held, entry.slug) !== undefined,
      ...maybeReason(disabledBy(t, prompt, held, entry.slug)),
    }))
  }

  return (set.options ?? []).map((option) => {
    const reason = disabledBy(t, prompt, held, option.key)
    return {
      key: option.key,
      icon: prompt.choice.kind === 'equipment' ? (entries.get(slugOf(firstRef(option) ?? '')) as Item | undefined)?.icon : undefined,
      provenance: entries.get(slugOf(firstRef(option) ?? ''))?.provenance,
      label: optionLabel(t, option, entries),
      ...maybeDetail(detailOf(t, option, entries)),
      disabled: reason !== undefined,
      ...maybeReason(reason),
    }
  })
}

/** The reference an option is badged by: its own, or its bundle's first. */
function firstRef(option: Option): string | undefined {
  if (option.ref !== undefined) return option.ref
  for (const item of option.items ?? []) {
    const ref = firstRef(item)
    if (ref !== undefined) return ref
  }
  return undefined
}

/**
 * Whether an entry is ammunition, which a weapon's label leaves unsaid: a
 * longbow comes with its arrows the way a crossbow comes with its string.
 * The same test `groupOf` uses to file it under Consumables.
 */
function isAmmunition(entry: Entry | undefined): boolean {
  return (entry as Item | undefined)?.gear?.gearCategory === 'ammunition'
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
  if (prompt.blocked?.includes(key)) return t('option.requiresProficiency')
  if (prompt.heldOnly) return held.has(key) ? undefined : t('option.notProficient')
  return held.has(key) ? t('option.alreadyHave') : undefined
}

/**
 * What one option reads as, `times` of it.
 *
 * `times` is only ever more than one where a prompt lets the same option be
 * picked twice, which is the improvement a level grants: two points into
 * Strength is "Strength +2", not "Strength +1" wearing a multiplier.
 */
export function optionLabel(
  t: Translate,
  option: Option,
  entries: Map<string, Entry>,
  times = 1,
): string {
  switch (option.kind) {
    case 'ref': {
      const slug = option.ref === undefined ? '' : slugOf(option.ref)
      const entry = entries.get(slug)
      const name = entry?.name ?? titleCase(slug)
      const count = (option.count ?? 1) * times
      return count > 1 ? `${name} ×${count}` : name
    }
    case 'nested':
      if (option.choice?.from.category !== undefined) {
        // One of a category is the category: "1 × Arcane Foci" is a count
        // that says nothing.
        const name = entries.get(option.choice.from.category)?.name ?? titleCase(option.choice.from.category)
        return option.choice.choose > 1 ? t('option.category', { count: option.choice.choose, name }) : name
      }
      // Named by what the branch chooses, not by how many: "Choose 2..." and
      // "Choose 1..." are the same button twice where one raises two ability
      // scores and the other takes a feat.
      return option.choice
        ? choiceOptionName(
            t,
            option.choice.kind,
            option.choice.choose,
            option.choice.from.collection,
          )
        : t('option.choose')
    case 'bundle': {
      // Joined with the catalogue's own word for "and", and without the
      // ammunition: a shortbow and twenty arrows reads "Shortbow", the
      // arrows being what a bow comes with. The detail under the picked
      // option still lists them.
      const items = option.items ?? []
      const named = items.filter((item) => !(item.ref !== undefined && isAmmunition(entries.get(slugOf(item.ref)))))
      return (named.length === 0 ? items : named)
        .map((item) => optionLabel(t, item, entries))
        .join(t('option.bundleJoin'))
    }
    case 'ability-bonus':
      return `${abilityName(t, option.ability ?? '')} +${(option.bonus ?? 1) * times}`
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
/**
 * An item's numbers on one line: a stat block, not six paragraphs. `name`
 * turns a damage type or weapon property slug into its word; the builder has
 * the entries, the sheet has the catalogue's names.
 *
 * No price: starting equipment is granted, not bought, and a "Cost: 12 gp"
 * under a free pack reads as a bill. The price belongs where things are bought.
 */
export function itemFacts(t: Translate, item: Item, name: (slug: string) => string): string | undefined {
  const facts: string[] = []
  if (item.armor !== undefined) {
    const armor = item.armor
    facts.push(t('equipment.ac', { value: armor.baseAC }))
    if (armor.addsDexBonus) facts.push(armor.maxDexBonus === undefined ? t('equipment.dex') : t('equipment.dexCap', { count: armor.maxDexBonus }))
    if (armor.strengthMinimum) facts.push(t('equipment.strength', { value: armor.strengthMinimum }))
    if (armor.stealthDisadvantage) facts.push(t('equipment.stealth'))
  }
  if (item.weapon !== undefined) {
    const weapon = item.weapon
    if (weapon.damage) facts.push(t('equipment.damage', { dice: weapon.damage.dice, type: weapon.damage.type === undefined ? '' : name(weapon.damage.type) }))
    if (weapon.twoHandedDamage) facts.push(t('equipment.twoHands', { dice: weapon.twoHandedDamage.dice }))
    if (weapon.normalRange) facts.push(t('equipment.range', { normal: weapon.normalRange, long: weapon.longRange ?? weapon.normalRange }))
    if (weapon.throwNormalRange) facts.push(t('equipment.thrownRange', { normal: weapon.throwNormalRange, long: weapon.throwLongRange ?? weapon.throwNormalRange }))
    if (weapon.properties?.length) facts.push(weapon.properties.map(name).join(', '))
  }
  if (item.weight !== undefined) facts.push(t('equipment.weight', { value: item.weight }))
  return facts.length === 0 ? undefined : facts.join(' · ')
}

function detailOf(t: Translate, option: Option, entries: Map<string, Entry>): string | undefined {
  // Each component under its own name in bold: a crossbow and twenty bolts are
  // two things, and a run of unheaded "Weight:" lines does not say whose.
  if (option.kind === 'bundle') return (option.items ?? []).map((item) => {
    const detail = detailOf(t, item, entries)
    const name = `**${optionLabel(t, item, entries)}**`
    return detail === undefined ? name : `${name}\n\n${detail}`
  }).join('\n\n')
  if (option.kind !== 'ref' || option.ref === undefined) return undefined
  const entry = entries.get(slugOf(option.ref))
  if (entry === undefined) return undefined
  const lines = [...(entry.desc ?? [])]
  if (option.ref.includes('item:')) {
    const item = entry as Item
    const facts = itemFacts(t, item, (slug) => entries.get(slug)?.name ?? slug)
    if (facts !== undefined) lines.push(facts)
    // A list, and a count only where it says something: fourteen "×1"s in one
    // sentence was a paragraph nobody could find the rope in.
    if (item.gear?.contents?.length) lines.push(t('equipment.contents'), ...item.gear.contents.map((item) => {
      const name = entries.get(item.item)?.name ?? titleCase(item.item)
      return item.count > 1 ? `- ${name} ×${item.count}` : `- ${name}`
    }))
  }
  return lines.length === 0 ? undefined : joinProse(lines)
}

/**
 * What an equipment question fills, as its title.
 *
 * A class kit is asked slot by slot -- "Body", "Main hand", "Off hand",
 * "Backup weapon", "Pack" -- and the server says which on the choice, so the
 * card is titled by that: a player fills a sheet by its slots, not by the
 * book's "(a) chain mail or (b) leather armor, a longbow and 20 arrows".
 *
 * A kit question with no slot -- a background's, a pack written before slots
 * -- is named by its options instead ("Component pouch or one of: Arcane
 * Foci"), which is how the rulebook writes them. Undefined for anything
 * else, and for a list too long to be a title.
 */
export function equipmentTitle(t: Translate, prompt: Prompt, names: ReadonlyMap<string, string>): string | undefined {
  const { kind, from, slot } = prompt.choice
  if (kind !== 'equipment') return undefined
  const slotTitle = slot === undefined ? undefined : slotTitles(t)[slot]
  if (slotTitle !== undefined) return slotTitle
  const category = (slug: string) =>
    t('choice.oneOf', { name: names.get(`equipment-category:${slug}`) ?? titleCase(slug.slice(slug.lastIndexOf('/') + 1)) })
  const capital = (title: string) => title.charAt(0).toUpperCase() + title.slice(1)
  // A question that is only a category has no alternative: it is granted on
  // top of everything else, and "also" is what tells it from the card above
  // that offers the same category as one of two options.
  if (from.category !== undefined) return capital(t('choice.also', { what: category(from.category) }))
  const options = from.options ?? []
  if (options.length === 0 || options.length > 3) return undefined
  const entries = new Map([...names].map(([ref, name]) => {
    const slug = slugOf(ref)
    return [slug, { slug, name }]
  }))
  const title = options
    .map((option) => option.choice?.from.category !== undefined ? category(option.choice.from.category) : optionLabel(t, option, entries))
    .join(t('choice.or'))
  return capital(title)
}

/** The kit slots a card can be titled by; a word not here falls back to the options. */
function slotTitles(t: Translate): Record<string, string | undefined> {
  return {
    body: t('equipment.slot.body'), 'main-hand': t('equipment.slot.main-hand'), 'off-hand': t('equipment.slot.off-hand'),
    backup: t('equipment.slot.backup'), pack: t('equipment.slot.pack'), focus: t('equipment.slot.focus'), instrument: t('equipment.slot.instrument'),
  }
}
