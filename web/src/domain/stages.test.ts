import { expect, it } from 'vitest'
import { stageOf, STAGES } from './stages'

it('separates cantrip choices and legacy saved cantrips while keeping personality and custom last', () => {
  expect(STAGES.slice(-5)).toEqual(['cantrips', 'spells', 'equipment', 'personality', 'custom'])
  expect(stageOf('class', 'spell', 'custom/spell/0', 'cantrip')).toBe('cantrips')
  expect(stageOf('class', undefined, 'wizard/spell/cantrip/1')).toBe('cantrips')
  expect(stageOf('race', 'spell', 'high-elf-cantrip/spell/0')).toBe('cantrips')
  expect(stageOf('race', undefined, 'high-elf-cantrip/spell/0')).toBe('cantrips')
  expect(stageOf('class', 'spell', 'wizard/spell/spellbook/1', 'spellbook')).toBe('spells')
  expect(stageOf('class', 'spell', 'wizard/spell/prepared/1', 'prepared')).toBe('spells')
  expect(stageOf('class', 'equipment')).toBe('equipment')
  expect(stageOf('class', 'feature')).toBe('class')
})

it('splits identity into personal and rules and routes level to class', () => {
  expect(STAGES.slice(0, 3)).toEqual(['rules', 'personal', 'class'])
  expect(stageOf('identity', 'text', 'character/init')).toBe('personal')
  expect(stageOf('identity', 'text', 'character/ruleset')).toBe('rules')
  expect(stageOf('identity', 'level', 'character/desired-level')).toBe('class')
})
