import { describe, expect, it } from 'vitest'

import { chatName, namedPack } from './agentChat'

describe('chatName', () => {
  it('is the local time the chat was opened and the head of its id', () => {
    // Built from local parts and read back as local parts, so the assertion
    // holds in whatever zone the suite runs.
    const opened = new Date(2026, 1, 1, 17, 20, 32).toISOString()
    expect(chatName(opened, '364665a07c743f7351e060120091b96c')).toBe('2026-02-01-17:20:32-364665a0')
    expect(chatName(new Date(2026, 10, 9, 4, 5, 6).toISOString(), 'abcdef0123')).toBe('2026-11-09-04:05:06-abcdef01')
  })

  it('is the head of the id until the chat has said when it was opened', () => {
    expect(chatName(undefined, '364665a07c743f7351e060120091b96c')).toBe('364665a0')
    expect(chatName('never', '364665a07c743f7351e060120091b96c')).toBe('364665a0')
  })
})

describe('namedPack', () => {
  const offered = [
    { id: 'dnd-2014', title: 'D&D 2014', label: 'D&D 2014 v2014.6.0' },
    { id: 'srd-2014', title: 'SRD 5.1', label: 'SRD 5.1 v1.4.0' },
  ]

  it.each([
    ['SRD 5.1', 'srd-2014'],
    ['srd', 'srd-2014'],
    ['d&d 2014', 'dnd-2014'],
    ['dnd 2014', 'dnd-2014'],
    ['D&D 2014 v2014.6.0', 'dnd-2014'],
    // A piece of the button it is on, and of no other -- the SRD's id says
    // 2014 too, and nobody types an id's suffix to mean it.
    ['2014', 'dnd-2014'],
  ])('reads %s as the answer to the question', (text, id) => {
    expect(namedPack(offered, text)).toEqual({ pack: offered.find((pack) => pack.id === id), only: true })
  })

  it('reads a first message that says which rules as more than an answer', () => {
    expect(namedPack(offered, 'Use SRD 5.1 and make me a level 5 wizard')).toEqual({ pack: offered[1], only: false })
  })

  it.each(['', '  ', '!!', 'A tiefling barbarian, level 3', 'D&D 2014 or SRD 5.1?'])(
    'does not guess from %j',
    (text) => {
      expect(namedPack(offered, text)).toBeUndefined()
    },
  )
})
