/**
 * What a chat is called: when it was opened, and enough of its id to tell two
 * opened in one second apart -- `2026-02-01-17:20:32-364665a0`.
 *
 * The id alone used to be the page's name, and thirty-two hex digits say
 * nothing to anyone. The time is the reader's own, like every other time the
 * app shows; it is a name and not a sentence, so it is the same in every
 * language and is not a message.
 */
export function chatName(created: string | undefined, id: string): string {
  const short = id.slice(0, 8)
  const at = created === undefined ? new Date(NaN) : new Date(created)
  if (Number.isNaN(at.getTime())) return short
  const two = (value: number) => String(value).padStart(2, '0')
  const day = [at.getFullYear(), two(at.getMonth() + 1), two(at.getDate())].join('-')
  const time = [at.getHours(), at.getMinutes(), at.getSeconds()].map(two).join(':')
  return `${day}-${time}-${short}`
}

/** A rule pack as the opening question offers it. */
export interface OfferedPack {
  id: string
  title: string
  /** The caption of its button: the title and the version. */
  label: string
}

/**
 * The rule pack an answer typed into the message box names.
 *
 * The opening question has a button per pack, and a question that can be
 * answered by pressing can be answered by writing. There is no model to read
 * the reply yet -- the rules are what it will be given -- so the reading is
 * this: the pack whose title or id the text contains, or whose button the
 * text is a piece of ("srd", "2014"). Punctuation and case are nobody's
 * intention, so "dnd 2014" is `dnd-2014`.
 *
 * `only` says the text is the pack's name and nothing besides: an answer to
 * the question, as against a first message that happens to say which rules.
 *
 * Two packs named, or none, is no answer: `undefined`, and the caller leaves
 * the rules to the server.
 */
export function namedPack<T extends OfferedPack>(
  offered: readonly T[],
  text: string,
): { pack: T; only: boolean } | undefined {
  const plain = (value: string) => value.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, '')
  const said = plain(text)
  if (said === '') return undefined
  const names = (pack: T) => [plain(pack.title), plain(pack.id)].filter((name) => name !== '')
  const hits = offered.filter(
    (pack) => names(pack).some((name) => said.includes(name)) || plain(pack.label).includes(said),
  )
  const pack = hits.length === 1 ? hits[0] : undefined
  if (pack === undefined) return undefined
  return { pack, only: plain(pack.label).includes(said) || names(pack).includes(said) }
}
