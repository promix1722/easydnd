import type { CharacterEvent, Dropped, Prompt, PromptsResponse, Sheet } from '@/lib/api'
import { type RulesLock } from '@/lib/api/packs'
import type { Translate } from '@/lib/i18n'
import type { Crumb } from '@/ui'

import { STAGES, stageOf } from '@/domain'
import type { Stage } from '@/domain'

import { keyFor } from './blocks'
import type { SettledRow } from './settled'
import type { SpellSubmission } from './SpellStagePanel'

/** Everything one build screen reads, in one round of requests. */
export interface BuildView {
 rules?: RulesLock | undefined
  prompts: PromptsResponse
  events: CharacterEvent[]
  sheet: Sheet | null
  names: Map<string, string>
}

export const EMPTY_VIEW: BuildView = {
  prompts: { seq: 0, complete: false, prompts: [] },
  events: [],
  sheet: null,
  names: new Map(),
}

/** An answered choice's question, fetched to be put again in place. */
export interface Reposed {
  seq: number
  prompt: Prompt
}

/** A change, priced before it is paid for. */
export interface Preview {
  spellBatch?: { submissions: SpellSubmission[]; next: Stage }
  row: SettledRow
  /** Null for a removal: there is nothing to put back. */
  event: CharacterEvent | null
  dropped: Dropped[]
  names: Map<string, string>
  /** The block to open once this is made: see `done`. */
  open: string | null
}

/**
 * The trail for a build page.
 *
 * Two crumbs either way: the character's name, or "New character" while there
 * is nothing yet to name. It used to end in a third reading "Creation", which
 * said what the screen does -- but the heading of a screen full of questions
 * is not asked what it is, and the word cost a line on a 390px trail to say
 * something the questions underneath it were already saying.
 */
export function buildTrail(t: Translate, isNew: boolean, name: string | null): Crumb[] {
  if (isNew) return [{ label: t('characters.newCharacter') }]
  return [{ label: name }]
}

/**
 * The tab creation asked to land on, out of the route state it rode in on.
 *
 * Unknown or absent is null rather than a guess: a link somebody typed carries
 * no state, and a state naming a tab this build does not have is a client that
 * has moved on. Both mean "open where you would have opened anyway".
 */
export function landingStage(state: unknown): Stage | null {
  const named = (state as { stage?: unknown } | null)?.stage
  return typeof named === 'string' && STAGES.includes(named as Stage) ? (named as Stage) : null
}

/**
 * The prompt a character with no log at all is asked.
 *
 * The server emits the real one -- `character/init` -- as soon as a character
 * exists to have an empty log. Before that there is no character to ask about
 * and no request to make, so Personal poses the same question itself
 * and the answer is a creation rather than an append.
 */
export const NEW_NAME_PROMPT: Prompt = {
  choice: { prompt: 'character/init', choose: 1, kind: 'text', from: { kind: 'explicit' } },
  group: 'identity',
  optional: false,
  event: { type: 'init' },
  heldOnly: false,
}

/**
 * The initial questions, shown on Rules, Personal and Class before creation.
 *
 * Rules can be chosen before creation and saved with the first character
 * write. Level still needs an existing character to be answered.
 */
const NEW_RULES_PROMPT: Prompt = {
  choice: { prompt: 'character/ruleset', choose: 1, kind: 'text', from: { kind: 'explicit' } },
  group: 'identity',
  optional: false,
  event: { type: 'change' },
  heldOnly: false,
}

export const NEW_INITIAL_PROMPTS: Prompt[] = [
  NEW_RULES_PROMPT,
  NEW_NAME_PROMPT,
  {
    choice: {
      prompt: 'character/desired-level',
      choose: 1,
      kind: 'level',
      from: { kind: 'explicit' },
    },
    group: 'identity',
    optional: false,
    event: { type: 'change' },
    heldOnly: false,
  },
]

export const NEW_NAME_KEY = keyFor({ prompt: NEW_NAME_PROMPT, replaces: null })
export const PACKS_KEY = 'rule-packs'
export const NEW_RULES_KEY = keyFor({ prompt: NEW_RULES_PROMPT, replaces: null })

/** The header's subject: what the character is called, or '' when it is not. */
export function title(view: BuildView): string {
  return view.sheet?.identity.name ?? ''
}

/**
 * The first category with something required still open.
 *
 * Optional prompts do not count. A finished character always has some -- a
 * custom spell is on offer to everybody -- so counting them would open Edit
 * on wherever that offer lives. With nothing required it opens the first tab.
 */
export function firstUnfinished(prompts: readonly Prompt[]): Stage {
  const required = new Set(
    prompts.filter((p) => !p.optional).flatMap((p) => [stageOf(p.group, p.choice.kind, p.choice.prompt, p.purpose)].filter(isStage)),
  )
  return STAGES.find((s) => required.has(s)) ?? 'rules'
}

/**
 * The next category with something still open, wrapping round.
 *
 * Follow display order, including optional questions such as Personality.
 * Only wrap to an earlier tab for required work that is still outstanding.
 *
 * The offer of a custom spell does not count: the server makes it to every
 * character for ever, so following it walked a fighter through Cantrips and
 * Spells on the way to anything else.
 */
export function stageAfter(stage: Stage, prompts: readonly Prompt[]): Stage | null {
  const all = new Set(prompts.filter((p) => p.purpose !== 'custom').flatMap((p) => [stageOf(p.group, p.choice.kind, p.choice.prompt, p.purpose)].filter(isStage)))
  const required = new Set(prompts.filter((p) => !p.optional)
    .flatMap((p) => [stageOf(p.group, p.choice.kind, p.choice.prompt, p.purpose)].filter(isStage)))
  // Someone who entered a name before choosing rules should see Rules next.
  if (stage === 'personal' && required.has('rules')) return 'rules'
  const from = STAGES.indexOf(stage)
  return STAGES.slice(from + 1).find((s) => all.has(s))
    ?? STAGES.slice(0, from).find((s) => required.has(s))
    ?? null
}

function isStage(stage: Stage | null): stage is Stage {
  return stage !== null
}
