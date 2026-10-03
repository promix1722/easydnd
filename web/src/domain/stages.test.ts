import { expect, it } from 'vitest'
import { stageOf, STAGES } from './stages'

it('separates cantrip choices and legacy saved cantrips while keeping equipment last', () => {
  expect(STAGES.slice(-3)).toEqual(['cantrips', 'spells', 'equipment'])
  expect(stageOf('class', 'spell', 'custom/spell/0', 'cantrip')).toBe('cantrips')
  expect(stageOf('class', undefined, 'wizard/spell/cantrip/1')).toBe('cantrips')
  expect(stageOf('race', 'spell', 'high-elf-cantrip/spell/0')).toBe('cantrips')
  expect(stageOf('race', undefined, 'high-elf-cantrip/spell/0')).toBe('cantrips')
  expect(stageOf('class', 'spell', 'wizard/spell/spellbook/1', 'spellbook')).toBe('spells')
  expect(stageOf('class', 'spell', 'wizard/spell/prepared/1', 'prepared')).toBe('spells')
  expect(stageOf('class', 'equipment')).toBe('equipment')
  expect(stageOf('class', 'feature')).toBe('class')
})
