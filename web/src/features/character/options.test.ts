import { describe, expect, it } from 'vitest'
import type { Entry, Prompt } from '@/lib/api'
import { testT } from '@/test/i18n'

import { choosableOptions } from './options'

function prompt(overrides: Partial<Prompt>): Prompt {
  return {
    choice: { prompt: 'test', choose: 1, kind: 'proficiency', from: { kind: 'explicit', options: [] } },
    group: 'class',
    optional: false,
    advances: false,
    heldOnly: false,
    event: { type: 'class' },
    ...overrides,
  }
}

const entries = new Map<string, Entry>([
  ['shortbow', { slug: 'shortbow', name: 'Shortbow' }],
  ['arrow', { slug: 'arrow', name: 'Arrow' }],
  ['shortsword', { slug: 'shortsword', name: 'Shortsword' }],
])

describe('choosableOptions', () => {
  // The rogue's starting kit is the case the server's option keys exist for:
  // the bundle has no slug, so it is named by position.
  it('uses the key the server sent, never one it computes', () => {
    const got = choosableOptions(
      testT,
      prompt({
        choice: {
          prompt: 'rogue/starting-equipment/1',
          choose: 1,
          kind: 'equipment',
          from: {
            kind: 'explicit',
            options: [
              {
                key: '#0',
                kind: 'bundle',
                items: [
                  { key: 'shortbow', kind: 'ref', ref: 'item:shortbow', count: 1 },
                  { key: 'arrow', kind: 'ref', ref: 'item:arrow', count: 20 },
                ],
              },
              { key: 'shortsword', kind: 'ref', ref: 'item:shortsword', count: 1 },
            ],
          },
        },
      }),
      entries,
    )

    expect(got.map((o) => o.key)).toEqual(['#0', 'shortsword'])
    expect(got[0]?.label).toBe('Shortbow and Arrow ×20')
    expect(got[1]?.label).toBe('Shortsword')
  })

  it('disables what the character already has', () => {
    const got = choosableOptions(
      testT,
      prompt({
        held: ['shortsword'],
        choice: {
          prompt: 'p',
          choose: 1,
          kind: 'proficiency',
          from: {
            kind: 'explicit',
            options: [
              { key: 'shortbow', kind: 'ref', ref: 'item:shortbow' },
              { key: 'shortsword', kind: 'ref', ref: 'item:shortsword' },
            ],
          },
        },
      }),
      entries,
    )

    expect(got.find((o) => o.key === 'shortbow')?.disabled).toBe(false)
    const held = got.find((o) => o.key === 'shortsword')
    expect(held?.disabled).toBe(true)
    expect(held?.reason).toBe('already have')
  })

  // Expertise inverts it: doubling a proficiency requires having it.
  it('inverts the test when heldOnly is set', () => {
    const got = choosableOptions(
      testT,
      prompt({
        heldOnly: true,
        held: ['shortsword'],
        choice: {
          prompt: 'rogue-expertise-1/expertise/0/0',
          choose: 2,
          kind: 'proficiency',
          from: {
            kind: 'explicit',
            options: [
              { key: 'shortbow', kind: 'ref', ref: 'item:shortbow' },
              { key: 'shortsword', kind: 'ref', ref: 'item:shortsword' },
            ],
          },
        },
      }),
      entries,
    )

    expect(got.find((o) => o.key === 'shortsword')?.disabled).toBe(false)
    const missing = got.find((o) => o.key === 'shortbow')
    expect(missing?.disabled).toBe(true)
    expect(missing?.reason).toBe('not proficient')
  })

  // A set drawn from a collection lists nothing inline; the collection is the
  // option list and an entry's own slug is its key.
  it('falls back to the loaded collection for a non-explicit set', () => {
    const got = choosableOptions(
      testT,
      prompt({
        choice: {
          prompt: 'character/race',
          choose: 1,
          kind: 'race',
          from: { kind: 'collection', collection: 'race' },
        },
      }),
      entries,
    )
    expect(got.map((o) => o.key).sort()).toEqual(['arrow', 'shortbow', 'shortsword'])
  })

  it('labels an ability bonus the way a player reads it', () => {
    const got = choosableOptions(
      testT,
      prompt({
        choice: {
          prompt: 'half-elf/ability-bonus/0',
          choose: 2,
          kind: 'ability-bonus',
          from: {
            kind: 'explicit',
            options: [{ key: 'dex', kind: 'ability-bonus', ability: 'dex', bonus: 1 }],
          },
        },
      }),
      new Map(),
    )
    expect(got[0]?.label).toBe('Dexterity +1')
  })
})
