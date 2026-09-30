import { describe, expect, it } from 'vitest'

import type { Choice, Option, Prompt } from '@/lib/api'

import { creationPrompt } from './creationPrompts'

const category: Choice = {
  prompt: 'acolyte/starting-equipment/0', choose: 1, kind: 'equipment',
  from: { kind: 'equipment-category', category: 'holy-symbols' },
}
const item: Option = { key: 'dagger', kind: 'ref', ref: 'item:dagger' }
function prompt(choice: Choice): Prompt {
  return { choice, group: 'background', optional: true, event: { type: 'background' }, heldOnly: false }
}

describe('creationPrompt', () => {
  it('omits an unimplemented category picker', () => {
    expect(creationPrompt(prompt(category))).toBeNull()
  })
  it('retains working equipment alternatives without changing their keys', () => {
    const original = prompt({ ...category, from: { kind: 'explicit', options: [
      { key: 'symbols', kind: 'nested', choice: category }, item,
    ] } })
    expect(creationPrompt(original)?.choice.from.options).toEqual([item])
    expect(original.choice.from.options).toHaveLength(2)
  })
  it('omits a question that cannot supply the requested number of picks', () => {
    expect(creationPrompt(prompt({ ...category, choose: 2, from: { kind: 'explicit', options: [
      { key: 'bundle', kind: 'bundle', items: [item, { key: 'symbols', kind: 'nested', choice: category }] }, item,
    ] } }))).toBeNull()
  })
  it('preserves free-entry score forms and implemented collections', () => {
    const choices: Choice[] = [
      { ...category, kind: 'ability-scores', from: { kind: 'explicit', options: [] } },
      { ...category, kind: 'language', from: { kind: 'collection', collection: 'language' } },
    ]
    for (const choice of choices) expect(creationPrompt(prompt(choice))?.choice).toEqual(choice)
  })
})
