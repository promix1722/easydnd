import type { Prompt } from '@/lib/api'

import type { SettledRow } from './settled'

/** The question on screen, and the entry answering it would replace. */
export interface Asking {
  prompt: Prompt
  /** Null when the answer is an append: a question being asked for the first time. */
  replaces: SettledRow | null
}

/**
 * One thing on a tab: something decided, or something still asked.
 *
 * A settled row and an open prompt are the same object at two moments -- the
 * choice of a race is the question "which race?" once it has an answer -- so
 * they are one list with two cases rather than two lists. What tells them
 * apart on screen is the highlight and the wording, not their position.
 */
export type Block =
  | { kind: 'settled'; key: string; level?: number; row: SettledRow; changeable: boolean }
  | { kind: 'open'; key: string; level?: number; prompt: Prompt }

/**
 * Everything a tab draws, in the order it draws it.
 *
 * Two rules, and the second one is the important one.
 *
 * **Where a block goes when it first appears** is by level, because a
 * character is built in levels and a list that interleaves "you took rogue at
 * 1" with "you still owe two skills at 3" reads as the story it is. Everything
 * without a level comes first: a race, a name and a background belong to no
 * level and are not waiting on one. Within a level, settled before open, each
 * in the order it arrived -- the log's for what is decided, the server's for
 * what is asked. The sort is stable, which is what preserves both.
 *
 * **Where a block goes afterwards is where it already was.** `order` is the
 * caller's memory of that, and it wins: answering a question adds what the
 * answer brought with it and moves nothing else. A list that re-sorted itself
 * on every answer would move the thing under the cursor and reshuffle the six
 * blocks around it, which is a page you have to re-read to find your place in
 * -- and re-reading it is the cost of a rule that only ever mattered the first
 * time each block was drawn.
 *
 * A key the order has never seen goes to the end, in level order among the
 * others like it, and is remembered there. That is what "new questions arrive
 * underneath" means, and it is also why an order that remembers nothing gives
 * the plain level ordering.
 *
 * Placing is what this does rather than something a caller does afterwards: a
 * block has to be given its place in the same pass that draws it, or the list
 * paints once in the wrong order and corrects itself on the next render.
 */
export function blocksFor(
  settled: readonly SettledRow[],
  prompts: readonly Prompt[],
  order: BlockOrder = blockOrder(),
): Block[] {
  const blocks: Block[] = [
    ...settled.map<Block>((row) => ({
      kind: 'settled',
      key: keyForRow(row),
      ...(row.level !== undefined ? { level: row.level } : {}),
      row,
      // A bare level entry is a fact about the character rather than a
      // control: nothing writes one any more -- a level comes from the
      // declared level -- and there was never a question about which class it
      // went into. What a level *granted* is a different thing wearing the
      // same event type, and stays changeable: an improvement, an Expertise,
      // a feature's pick all arrive as level events carrying answers, and
      // locking them with the levels took a whole class of real decisions off
      // the screen.
      //
      // The ruleset is fixed for its own reason: choosing it is final, and
      // the server holds every change to it to the one value already in
      // effect.
      changeable: !isTakenLevel(row) && !setsRuleset(row),
    })),
    ...prompts.map<Block>((prompt) => ({
      kind: 'open',
      key: keyForPrompt(prompt),
      ...(prompt.level !== undefined ? { level: prompt.level } : {}),
      prompt,
    })),
  ]
  // Real levels start at 1, so -1 sorts the un-levelled first without a
  // sentinel a level could ever collide with.
  blocks.sort((a, b) => (a.level ?? -1) - (b.level ?? -1))
  for (const block of blocks) place(order, block.key)
  const at = (block: Block) => order.places.get(block.key) ?? Number.MAX_SAFE_INTEGER
  return blocks.sort((a, b) => at(a) - at(b))
}

/** A level with nothing to re-ask: it took a level and granted no choice. */
function isTakenLevel(row: SettledRow): boolean {
  return row.event.type === 'level' && (row.event.choices ?? []).length === 0
}

/** Whether an entry is the one that recorded the ruleset, which is final. */
function setsRuleset(row: SettledRow): boolean {
  return (row.event.changes ?? []).some((change) => change.path === 'identity.ruleset')
}

/** One run of blocks under one heading: a class level, or none. */
export interface LevelGroup {
  level?: number
  blocks: Block[]
}

/**
 * The list cut into its levels, for the headings that replace per-card tags.
 *
 * Grouping happens after ordering, so a block keeps its pinned place *within*
 * its level while the levels themselves always read in order -- un-levelled
 * first, then ascending. A tab with no levelled blocks comes back as one
 * heading-less group, which is every tab except the class story.
 */
export function groupByLevel(blocks: readonly Block[]): LevelGroup[] {
  const runs = new Map<number, Block[]>()
  for (const block of blocks) {
    const level = block.level ?? -1
    const run = runs.get(level) ?? []
    run.push(block)
    runs.set(level, run)
  }
  return [...runs]
    .sort(([a], [b]) => a - b)
    .map(([level, grouped]) => (level < 0 ? { blocks: grouped } : { level, blocks: grouped }))
}

/**
 * Where each block sits, remembered from when it first appeared.
 *
 * Deliberately mutable and deliberately not React state. It is written by the
 * render that discovers a block and read by the same one, and nothing should
 * redraw because a block learned where it already is.
 *
 * Places are never given up. A key that has gone is a position nothing claims,
 * and remembering it costs one map entry per selection of one character --
 * where forgetting it would mean an answer changed twice comes back at the
 * bottom of the list the second time.
 */
export interface BlockOrder {
  places: Map<string, number>
  next: number
  /** A place left by a block that is coming back as something else. */
  vacated: number | null
}

export function blockOrder(): BlockOrder {
  return { places: new Map(), next: 0, vacated: null }
}

/** Where a block goes if it has not been anywhere yet. */
function place(order: BlockOrder, key: string): void {
  if (order.places.has(key)) return
  if (order.vacated !== null) {
    order.places.set(key, order.vacated)
    order.vacated = null
    return
  }
  order.places.set(key, order.next)
  order.next += 1
}

/**
 * Holds a block's place for whatever arrives in its stead.
 *
 * An answer that cannot be put again is dropped so that its question comes
 * back, and the question that comes back is a different block with a different
 * key -- so without this it would arrive at the bottom of the list, and the
 * one thing the player was looking at would be the one thing that moved.
 */
export function reclaimPlace(order: BlockOrder, key: string): void {
  order.vacated = order.places.get(key) ?? null
}

/**
 * Gives one block another's place: an answer belongs where its question was.
 *
 * Without it an answered question vanishes from the middle of the list and its
 * answer appears at the bottom, which is two movements to follow for one
 * click that should have changed a block in place.
 */
export function inheritPlace(order: BlockOrder, from: string | null, to: string): void {
  const place = from === null ? undefined : order.places.get(from)
  if (place !== undefined) order.places.set(to, place)
}

/** The key a stored entry is drawn under, wherever one is named from a seq. */
export function settledKey(seq: number): string {
  return `settled:${seq}`
}

/**
 * The key of the block a question is being asked in.
 *
 * Lives here, beside the two spellings it has to agree with, because a screen
 * that opened `settled:2` while the list drew `entry-2` would be a list with
 * nothing open and no error to show for it.
 */
export function keyFor(asking: Asking): string {
  return asking.replaces === null ? keyForPrompt(asking.prompt) : keyForRow(asking.replaces)
}

function keyForRow(row: SettledRow): string {
  return settledKey(row.seq)
}

function keyForPrompt(prompt: Prompt): string {
  return promptKey(prompt.choice.prompt)
}

/**
 * The key an outstanding question is drawn under, named from the prompt alone.
 *
 * The screen needs this for a question it has not been handed yet: answering a
 * nested option, or dropping an entry to put its question again, both make a
 * block appear on the *next* read, and the key is what lets that block be
 * opened the moment it arrives instead of a press later.
 */
export function promptKey(prompt: string): string {
  return `open:${prompt}`
}
