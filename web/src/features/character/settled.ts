import type { Change, CharacterEvent } from '@/lib/api'

import { writtenLabel } from './promptNames'
import { refName } from './refNames'

import { ABILITY_ORDER, pickLabel, promptLabel, stageOf } from '@/domain'
import type { Stage } from '@/domain'
import type { Translate } from '@/lib/i18n'

import { abilityName, describeChange, eventLabel, formatValue } from './labels'

/**
 * One thing the player has already decided, and the entry that records it.
 *
 * `seq` is the whole point. The log is one entry per selection now, so every
 * settled row is exactly one entry -- which is what makes it a thing that can
 * be pointed at and replaced. A row assembled from two entries, or two rows
 * sharing one, would have no answer to "change this".
 */
export interface SettledRow {
  seq: number
  stage: Stage
  /** What was decided: "Race chosen", "Name", "Ability scores". */
  label: string
  /** What it was decided to be. */
  value: string
  /** Set where the decision belongs to a class level. */
  level?: number
  /** The entry itself, so a replacement can be built from what it was. */
  event: CharacterEvent
}

/** What `settledByStage` reads: a log, and the compendium's names for it. */
export interface SettledView {
  events: readonly CharacterEvent[]
  names: ReadonlyMap<string, string>
}

/**
 * Every stored selection, grouped by the tab it was made on.
 *
 * The grouping is the server's: each entry carries the group of the prompt it
 * answered, so nothing here infers a category from an event's type. That
 * matters because the same type lands in two places -- a `level` event is the
 * class story, and a `change` event might be the six ability scores or a DM's
 * ruling -- and only the server knows which prompt it settled.
 *
 * An entry the server could not attribute is in no tab at all. It is not lost:
 * `/characters/:id/log` is the unabridged record, and this screen is a
 * constructor rather than a history.
 */
export function settledByStage(t: Translate, view: SettledView): Map<Stage, SettledRow[]> {
  const byStage = new Map<Stage, SettledRow[]>()
  for (const event of view.events) {
    const stage = stageOf(event.source)
    if (stage === null) continue
    const row = rowFor(t, event, stage, view.names)
    if (row === null) continue
    const rows = byStage.get(stage) ?? []
    rows.push(row)
    byStage.set(stage, rows)
  }
  return byStage
}

/**
 * One entry as a row, or null when it says nothing worth a row.
 *
 * Answers are read before the reference, and that order is the whole of it.
 * A follow-up entry carries the ref of the thing that posed it -- a half-elf's
 * ability bonuses arrive as a `race` event naming `race:half-elf` -- so a row
 * that led with the reference would print "Race chosen: Half-Elf" twice and
 * never print what the player actually picked. The reference is the anchor;
 * the answers are the selection.
 */
function rowFor(
  t: Translate,
  event: CharacterEvent,
  stage: Stage,
  names: ReadonlyMap<string, string>,
): SettledRow | null {
  const seq = event.seq ?? 0
  const level = event.level !== undefined && event.level > 0 ? { level: event.level } : {}

  const answers = event.choices ?? []
  const first = answers.find((answer) => answer.picks.length > 0)
  if (first !== undefined) {
    return {
      seq,
      stage,
      label: promptLabel(first.prompt),
      value: answers.flatMap((answer) => answer.picks).map(pickLabel).join(', '),
      ...level,
      event,
    }
  }

  if (event.ref !== undefined) {
    return {
      seq,
      stage,
      label: eventLabel(t, event.type),
      value: refName(event.ref, names),
      ...level,
      event,
    }
  }

  const changes = event.changes ?? []
  if (changes.length > 0) return { seq, stage, ...summarise(t, changes), ...level, event }

  return null
}

/**
 * Addressed changes, read back as a decision rather than as a patch.
 *
 * The paths this client writes are worth naming: the character's name, its
 * alignment, the four lines it writes about who the character is, and the six
 * ability scores. Anything else -- a DM's adjustment, a path a later server
 * learned before this client did -- falls back to the log screen's own
 * rendering, on the same principle `eventLabel` follows: an entry drawn plainly
 * is better than one refused.
 */
function summarise(t: Translate, changes: readonly Change[]): { label: string; value: string } {
  const name = changes.find((change) => change.path === 'identity.name')
  if (name !== undefined) return { label: t('settled.name'), value: formatValue(t, name.value) }

  const alignment = changes.find((change) => change.path === 'identity.alignment')
  if (alignment !== undefined) {
    return { label: t('settled.alignment'), value: formatValue(t, alignment.value) }
  }

  // Every line of one, joined: a trait entry may carry two, and reading back
  // only the first would make the second invisible until the sheet.
  const written = changes.find((change) => writtenLabel(change.path) !== undefined)
  if (written !== undefined) {
    const label = writtenLabel(written.path)
    return {
      label: label === undefined ? '' : t(label),
      value: changes
        .filter((change) => change.path === written.path)
        .map((change) => formatValue(t, change.value))
        .join(' · '),
    }
  }

  const scores = new Map<string, Change>()
  for (const change of changes) {
    const [head, tail] = change.path.split('.')
    if (head === 'abilities' && tail !== undefined) scores.set(tail, change)
  }
  if (scores.size > 0) {
    const printed = ABILITY_ORDER.flatMap((ability) => {
      const change = scores.get(ability)
      return change === undefined
        ? []
        : [`${abilityName(t, ability)} ${formatValue(t, change.value)}`]
    })
    const method = scores.get('method')
    return {
      label: t('settled.abilityScores'),
      // formatValue title-cases a slug already, so the method reads
      // "Point Buy" rather than "point-buy".
      value:
        printed.join(' · ') + (method === undefined ? '' : ` · ${formatValue(t, method.value)}`),
    }
  }

  return {
    label: eventLabel(t, 'change'),
    value: changes.map((change) => describeChange(t, change)).join(', '),
  }
}
