/** Builder tabs are presentation; class/race/background remain the rule owners. */

export type Stage = 'personal' | 'rules' | 'class' | 'race' | 'background' | 'abilities' | 'personality' | 'cantrips' | 'spells' | 'equipment'

/** Where advancement stops in the 2014 rules; the server enforces the same. */
export const MAX_LEVEL = 20

/**
 * Display order, which is the order a character is actually built in.
 *
 * Class comes first after the name because it is the choice every other choice
 * hangs off -- a class opens more prompts than a race does, and a player who
 * has picked one can see the shape of the rest. The scores follow immediately,
 * because they are what the class was picked *for*: a barbarian wants the 15
 * in Strength, and deciding that while the class is still the last thing you
 * looked at is the difference between building a character and filling in a
 * form. Race and background come after, and change nothing about the numbers
 * that has not already been decided -- a racial bonus is applied by the rules,
 * not typed in here.
 *
 * Personality asks about the character. Spells and equipment follow the
 * choices that grant them, with equipment always last.
 * A trait, an ideal, a bond, a flaw and an alignment are who the character is
 * rather than what they can do, and they used to sit under background because
 * that is which entry suggests them -- which put five questions nobody has to
 * answer in front of the one required question on that tab.
 */
export const STAGES = [
  'rules',
  'personal',
  'class',
  'abilities',
  'race',
  'background',
  'personality',
  'cantrips',
  'spells',
  'equipment',
] as const satisfies readonly Stage[]

const STAGE_OF_GROUP: Record<string, Stage> = {
  identity: 'personal',
  personal: 'personal',
  rules: 'rules',
  class: 'class',
  race: 'race',
  background: 'background',
  abilities: 'abilities',
  personality: 'personality',
  cantrips: 'cantrips',
  spells: 'spells',
  equipment: 'equipment',
}

/**
 * Spell/equipment kinds get their own tabs; other choices retain their group.
 * Older saved events can omit the kind, so their prompt address is a fallback.
 *
 * Null is a real answer rather than a defensive one. The server attributes an
 * event to the prompt it satisfied, and some events satisfy none -- an
 * imported log, a DM's adjustment, a note. Those have no tab, and the event
 * log at `/characters/:id/log` remains the unabridged record of them.
 */
export function stageOf(group: string | undefined, choiceKind?: string, prompt?: string, purpose?: string): Stage | null {
  if (prompt === 'character/ruleset') return 'rules'
  if (prompt === 'character/desired-level') return 'class'
  if (choiceKind === 'spell' || (choiceKind === undefined && prompt?.split('/').includes('spell'))) {
    const cantrip = purpose === 'cantrip' || prompt?.split('/').includes('cantrip') || prompt?.startsWith('high-elf-cantrip/')
    return cantrip ? 'cantrips' : 'spells'
  }
  if (choiceKind === 'equipment' || (choiceKind === undefined && prompt?.split('/').includes('starting-equipment'))) return 'equipment'
  return group === undefined ? null : (STAGE_OF_GROUP[group] ?? null)
}
