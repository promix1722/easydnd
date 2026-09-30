import type { Choice, Option, Prompt } from '@/lib/api'

/** Equipment-category menus do not yet have a picker in the creation UI.
 * Keep usable alternatives in mixed/nested choices, and omit a question when
 * too few supported alternatives remain to answer it. The source rules stay
 * intact for replay and for the future equipment workflow. */
export function creationPrompt(prompt: Prompt): Prompt | null {
  const choice = supportedChoice(prompt.choice)
  return choice === null ? null : { ...prompt, choice }
}

function supportedChoice(choice: Choice): Choice | null {
  if (choice.from.kind === 'equipment-category') return null
  if (choice.from.kind !== 'explicit') return choice
  const original = choice.from.options ?? []
  const options = original.flatMap((option) => {
    const supported = supportedOption(option)
    return supported === null ? [] : [supported]
  })
  // Empty explicit sets also describe text/score forms, so only discard a
  // question here when removing an unsupported branch made it unanswerable.
  if (options.length !== original.length) {
    const needed = choice.repeatable ? 1 : choice.choose
    if (options.length < needed) return null
  }
  return { ...choice, from: { ...choice.from, options } }
}

function supportedOption(option: Option): Option | null {
  if (option.choice !== undefined) {
    const choice = supportedChoice(option.choice)
    if (choice === null) return null
    return { ...option, choice }
  }
  if (option.items !== undefined) {
    const items = option.items.map(supportedOption)
    // A bundle promises every item, so it cannot silently lose one component.
    if (items.some((item) => item === null)) return null
    return { ...option, items: items.filter((item) => item !== null) }
  }
  return option
}
