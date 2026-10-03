import { expect, it } from 'vitest'
import type { Prompt } from '@/lib/api'
import { allocateSpells } from './spellAllocation'

const allowance = (level: number, choose: number, eligible: string[]) => ({
  prompt: { choice: { prompt: `warlock/spell/known/${level}`, choose } } as Prompt, eligible,
})

it('assigns a combined selection to eligible level slots regardless of click order', () => {
  const first = allowance(1, 2, ['a', 'b', 'c'])
  const later = allowance(3, 1, ['a', 'b', 'c', 'higher'])
  const result = allocateSpells([first, later], ['higher', 'a', 'b'])
  expect(result?.get(first.prompt.choice.prompt)?.sort()).toEqual(['a', 'b'])
  expect(result?.get(later.prompt.choice.prompt)).toEqual(['higher'])
  expect(allocateSpells([first, later], ['a', 'a'])).toBeNull()
  expect(allocateSpells([first], ['higher'])).toBeNull()
})

it('reassigns flexible spells so restricted source choices are not blocked', () => {
  const restricted = allowance(1, 1, ['a'])
  const flexible = allowance(2, 1, ['a', 'b'])
  const result = allocateSpells([flexible, restricted], ['a', 'b'])
  expect(result?.get(restricted.prompt.choice.prompt)).toEqual(['a'])
  expect(result?.get(flexible.prompt.choice.prompt)).toEqual(['b'])
  expect(allocateSpells([restricted, flexible], ['a', 'b', 'c'])).toBeNull()
})

it('shares the highest-level cap across historical prompts without constraining lower levels', async () => {
  const { allocateLimitedSpells } = await import('./spellAllocation')
  const first = allowance(1, 2, ['a', 'b', 'c', 'low'])
  const later = allowance(5, 2, ['a', 'b', 'c', 'low'])
  for (const item of [first, later]) { item.prompt.source = 'class:sorcerer'; item.prompt.purpose = 'known' }
  const limits = [{ source: 'class:sorcerer', purpose: 'known', level: 3, remaining: 2 }]
  const levels = new Map([['a', 3], ['b', 3], ['c', 3], ['low', 2]])
  expect(allocateLimitedSpells([first, later], ['a', 'b', 'c'], limits, levels)).toBeNull()
  const legal = allocateLimitedSpells([first, later], ['a', 'b', 'low'], limits, levels)
  expect([...legal!.values()].flat().sort()).toEqual(['a', 'b', 'low'])
  const other = allowance(5, 1, ['a', 'b', 'c'])
  other.prompt.source = 'class:bard'
  other.prompt.choice.prompt = 'bard/spell/known/5'
  other.prompt.purpose = 'known'
  const multi = allocateLimitedSpells([first, later, other], ['a', 'b', 'c'], limits, levels)
  expect(multi?.get(other.prompt.choice.prompt)).toHaveLength(1)
})
