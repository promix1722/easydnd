import type { ChangeLike, Stage, ValueLike } from '@/domain'
import { titleCase } from '@/domain'
import { formatDateTime, type MessageKey, type Translate } from '@/lib/i18n'

/**
 * The words a character log and a build screen are read in.
 *
 * These tables used to live in `src/domain/`, next to the pure helpers that
 * still do. They moved for the reason that directory exists: it may not import
 * React and it holds no prose, so a table of English nouns in it was the one
 * thing there that could not survive a second language. What stayed behind is
 * everything that is genuinely a rule -- the order the abilities print in,
 * which collection names a reference, how a prompt path is segmented.
 *
 * Every function here takes its `t` rather than calling a hook, because most
 * of the callers are pure modules -- `settled.ts`, `options.ts` -- and a hook
 * would make them components. It is the same shape as `InviteSheet`'s
 * `copyLink`, and it is what makes these testable without a provider.
 */

/**
 * The event types in internal/domain/character/event.go, read back as prose.
 *
 * Deliberately past tense and phrased from the character's side: the log is a
 * history, and "Race chosen" is what happened where "race" is only a field.
 *
 * A table of literal keys rather than a template on the type, so that every
 * key in it is a string `check-messages.mjs` can find and the compiler can
 * check. `event.label.${type}` would be neither.
 */
const EVENT_KEYS = {
  init: 'event.init',
  change: 'event.change',
  race: 'event.race',
  subrace: 'event.subrace',
  background: 'event.background',
  class: 'event.class',
  subclass: 'event.subclass',
  level: 'event.level',
  feat: 'event.feat',
  note: 'event.note',
} as const

/**
 * Reads one of the tables above.
 *
 * A slug the table does not name is title-cased rather than rejected. The
 * server may learn a kind before this client does, and a log that refuses to
 * draw the one event you are trying to understand is worse than a log that
 * draws it plainly. That fallback is English-shaped, which is a known and
 * accepted rough edge: it is reached only by something this client has never
 * heard of.
 */
function lookup(table: Record<string, MessageKey>) {
  return (t: Translate, slug: string): string => {
    const key = table[slug]
    return key === undefined ? titleCase(slug) : t(key)
  }
}

/** An event type as a person reads it. */
export const eventLabel = lookup(EVENT_KEYS)

/**
 * A stage's label, which is deliberately the stage's own word.
 *
 * The word appears in exactly one place in the document: the tab. A block is
 * headed by the choice's own name -- "A race" -- or by what was decided --
 * "Race chosen" -- and empty copy names no category at all. That rule is what
 * keeps "race" an unambiguous thing to look for on the page, and it is why
 * this table exists rather than the labels being inlined.
 */
const STAGE_KEYS = {
  identity: 'stage.identity',
  class: 'stage.class',
  race: 'stage.race',
  background: 'stage.background',
  abilities: 'stage.abilities',
  personality: 'stage.personality',
} as const satisfies Record<Stage, string>

export function stageLabel(t: Translate, stage: Stage): string {
  return t(STAGE_KEYS[stage])
}

/**
 * The six abilities' full names, keyed by the slug the API uses.
 *
 * Not fetched, for the reason `ABILITY_ORDER` is not: these six are the one
 * part of the compendium that cannot change -- a sixth-and-a-half ability
 * would be a different game -- so waiting on a round trip to draw six labelled
 * inputs would be a worse first screen for no benefit. The catalogue's
 * abilities collection carries the same names for anywhere that has already
 * loaded it.
 */
const ABILITY_KEYS = {
  str: 'ability.str',
  dex: 'ability.dex',
  con: 'ability.con',
  int: 'ability.int',
  wis: 'ability.wis',
  cha: 'ability.cha',
} as const

/** An ability's full name, or the slug title-cased if it is not one. */
export const abilityName = lookup(ABILITY_KEYS)

/**
 * The three-letter abbreviation a sheet prints: STR, or СИЛ.
 *
 * Its own table rather than a truncation of the full name, because a
 * truncation is an English habit: "Strength" cuts to STR and "Сила" does not
 * cut to СИЛ. Every language picks its own short forms, so they are keys.
 * These follow dnd.su, which is the reference Russian players actually use.
 *
 * The slug was rendered directly before this existed -- `{ability}` with
 * `text-transform: uppercase` over it -- which is why the ability cards read
 * STR DEX CON on a Russian sheet.
 */
const ABILITY_ABBR_KEYS = {
  str: 'ability.str.abbr',
  dex: 'ability.dex.abbr',
  con: 'ability.con.abbr',
  int: 'ability.int.abbr',
  wis: 'ability.wis.abbr',
  cha: 'ability.cha.abbr',
} as const

export function abilityAbbr(t: Translate, slug: string): string {
  const key = ABILITY_ABBR_KEYS[slug as keyof typeof ABILITY_ABBR_KEYS]
  return key === undefined ? slug.toUpperCase() : t(key)
}

/** A stored value, printed. */
export function formatValue(t: Translate, value: ValueLike | undefined): string {
  if (value === undefined) return ''
  switch (value.kind) {
    case 'int':
      return String(value.int ?? 0)
    case 'string':
      return value.string ?? ''
    case 'bool':
      return value.bool === true ? t('value.yes') : t('value.no')
    case 'slug':
      return titleCase(value.slug ?? '')
    case 'slugs':
      return (value.slugs ?? []).map(titleCase).join(', ')
    case 'dice':
      return value.dice ?? ''
    default:
      return ''
  }
}

/**
 * One change, as one line: "abilities.dex set 16".
 *
 * The path is left as the log stores it. It is an address, and prettifying it
 * would make the log harder to match against the event that produced it.
 */
export function describeChange(t: Translate, change: ChangeLike): string {
  const printed = formatValue(t, change.value)
  return printed === '' ? `${change.path} ${change.op}` : `${change.path} ${change.op} ${printed}`
}

/**
 * When an event was recorded, in the reader's own timezone and language.
 *
 * Blank is a real case rather than a defensive one: Service.Create seeds the
 * init event without stamping At, so the first event of every character has no
 * time. An unparseable value is printed as it arrived -- losing it would hide
 * exactly the kind of bad record this page exists to show.
 *
 * The locale is passed rather than left to the browser. `toLocaleString()`
 * with no argument follows the *browser's* language, which stopped being the
 * app's the moment there was a switcher: a visitor who chose Russian on an
 * English machine would read Russian captions above English dates.
 */
export function formatAt(locale: string, at: string | undefined): string {
  return at === undefined || at === '' ? '--' : formatDateTime(at, locale)
}

/**
 * The speeds and senses a projected sheet can carry.
 *
 * Both are closed enumerations the Go model owns
 * (`internal/domain/character/names.go`), so they are key tables rather than
 * `titleCase(slug)` -- which is what they were, and which is why a Russian
 * sheet printed "Darkvision 60 фт.". An unknown kind still draws, title-cased:
 * the server may learn a sense before this client does, and a blank card is
 * worse than an English word.
 */
const SPEED_KEYS = {
  walking: 'speed.walking',
  flying: 'speed.flying',
  climbing: 'speed.climbing',
  swimming: 'speed.swimming',
  burrowing: 'speed.burrowing',
} as const

const SENSE_KEYS = {
  darkvision: 'sense.darkvision',
  blindsight: 'sense.blindsight',
  tremorsense: 'sense.tremorsense',
  truesight: 'sense.truesight',
} as const

export const speedName = lookup(SPEED_KEYS)
export const senseName = lookup(SENSE_KEYS)

/** Class-resource keys emitted by the rules engine. */
const RESOURCE_KEYS = {
  'action-surges': 'resource.action-surges',
  'arcane-recovery-levels': 'resource.arcane-recovery-levels',
  'aura-range': 'resource.aura-range',
  'bardic-inspiration-die': 'resource.bardic-inspiration-die',
  'brutal-critical-dice': 'resource.brutal-critical-dice',
  'channel-divinity-charges': 'resource.channel-divinity-charges',
  'destroy-undead-cr': 'resource.destroy-undead-cr',
  'extra-attacks': 'resource.extra-attacks',
  'favored-enemies': 'resource.favored-enemies',
  'favored-terrain': 'resource.favored-terrain',
  'indomitable-uses': 'resource.indomitable-uses',
  'invocations-known': 'resource.invocations-known',
  'ki-points': 'resource.ki-points',
  'magical-secrets-max-5': 'resource.magical-secrets-max-5',
  'magical-secrets-max-7': 'resource.magical-secrets-max-7',
  'magical-secrets-max-9': 'resource.magical-secrets-max-9',
  'martial-arts': 'resource.martial-arts',
  'metamagic-known': 'resource.metamagic-known',
  'mystic-arcanum-level-6': 'resource.mystic-arcanum-level-6',
  'mystic-arcanum-level-7': 'resource.mystic-arcanum-level-7',
  'mystic-arcanum-level-8': 'resource.mystic-arcanum-level-8',
  'mystic-arcanum-level-9': 'resource.mystic-arcanum-level-9',
  'pact-magic-level-1': 'resource.pact-magic-level-1',
  'pact-magic-level-2': 'resource.pact-magic-level-2',
  'pact-magic-level-3': 'resource.pact-magic-level-3',
  'pact-magic-level-4': 'resource.pact-magic-level-4',
  'pact-magic-level-5': 'resource.pact-magic-level-5',
  'rage-count': 'resource.rage-count',
  'rage-damage-bonus': 'resource.rage-damage-bonus',
  'sneak-attack': 'resource.sneak-attack',
  'song-of-rest-die': 'resource.song-of-rest-die',
  'sorcery-points': 'resource.sorcery-points',
  'unarmored-movement': 'resource.unarmored-movement',
  'wild-shape-fly': 'resource.wild-shape-fly',
  'wild-shape-max-cr': 'resource.wild-shape-max-cr',
  'wild-shape-swim': 'resource.wild-shape-swim',
} as const

export const resourceName = lookup(RESOURCE_KEYS)
