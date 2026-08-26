/**
 * The five categories a character is built in, in the order they are offered.
 *
 * These are the server's own prompt groups, not a client-side taxonomy: every
 * prompt arrives carrying one, and every stored entry carries the group of the
 * prompt it answered. So this file holds no rule about what belongs where --
 * only the order the tabs sit in, and the one place two server groups collapse
 * into one tab.
 *
 * Framework-free, like everything else in domain/: no React, no transport.
 */

export type Stage = 'identity' | 'class' | 'race' | 'background' | 'abilities' | 'personality'

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
 * Personality is last, and is the only tab that asks nothing about the rules.
 * A trait, an ideal, a bond, a flaw and an alignment are who the character is
 * rather than what they can do, and they used to sit under background because
 * that is which entry suggests them -- which put five questions nobody has to
 * answer in front of the one required question on that tab.
 */
export const STAGES = [
  'identity',
  'class',
  'abilities',
  'race',
  'background',
  'personality',
] as const satisfies readonly Stage[]

/**
 * A stage's label, which is deliberately the stage's own word.
 *
 * The word lives here and appears in exactly one place in the document: the
 * tab. A block is headed by the choice's own name -- "A race" -- or by what
 * was decided -- "Race chosen" -- and empty copy names no category at all.
 * That rule is what keeps "race" an unambiguous thing to look for on the page,
 * and it is why this table exists rather than the labels being inlined.
 */
export const STAGE_LABELS: Record<Stage, string> = {
  identity: 'identity',
  class: 'class',
  race: 'race',
  background: 'background',
  abilities: 'abilities',
  personality: 'personality',
}

const STAGE_OF_GROUP: Record<string, Stage> = {
  identity: 'identity',
  class: 'class',
  race: 'race',
  background: 'background',
  abilities: 'abilities',
  personality: 'personality',
  // Advancement is the class story continued: which class the next level goes
  // into, the archetype it unlocks, the improvement it grants. A tab of its
  // own would split "I am a rogue" from "I am taking a third rogue level".
  advance: 'class',
}

/**
 * The tab a server group belongs to, or null.
 *
 * Null is a real answer rather than a defensive one. The server attributes an
 * event to the prompt it satisfied, and some events satisfy none -- an
 * imported log, a DM's adjustment, a note. Those have no tab, and the event
 * log at `/characters/:id/log` remains the unabridged record of them.
 */
export function stageOf(group: string | undefined): Stage | null {
  return group === undefined ? null : (STAGE_OF_GROUP[group] ?? null)
}

/**
 * Whether this client will offer to answer a prompt in this group.
 *
 * `advance` is not offered, and the reason is that it does not work: taking a
 * level posts a `level` event the server records as a no-op, so the question
 * would appear answerable, be answered, and change nothing. A question that
 * silently does nothing is worse than a question that is not asked -- and the
 * same judgement took the sheet's "Level up" button off the page.
 *
 * Advancement is filtered out of what is *offered*, never out of what is
 * *recorded*: `stageOf` still puts an `advance` entry on the class tab,
 * because levels an imported character already has are facts about it. They
 * are shown, and they are not editable, for the same reason.
 *
 * This goes when the level machinery works, and nothing else has to change.
 */
export function answerable(group: string | undefined): boolean {
  return group !== 'advance'
}
