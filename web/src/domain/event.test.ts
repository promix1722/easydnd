import { describe, expect, it } from 'vitest'

import { collectionOfKind, pickLabel, promptLabel } from './index'

describe('collectionOfKind', () => {
  it('names the collection that would name a reference', () => {
    expect(collectionOfKind('race')).toBe('races')
    expect(collectionOfKind('class')).toBe('classes')
    expect(collectionOfKind('feat')).toBe('feats')
  })

  // The same table now answers the build screen's other question -- which
  // collection a prompt's options come out of -- so the kinds a log never
  // stores are in it too. It used to be two tables in two files, and the one
  // missing an entry was whichever one you were not looking at.
  it('covers the kinds only a prompt names', () => {
    expect(collectionOfKind('language')).toBe('languages')
    expect(collectionOfKind('item')).toBe('equipment')
    expect(collectionOfKind('damage-type')).toBe('damage-types')
  })

  it('is null where the compendium serves no collection', () => {
    expect(collectionOfKind('multiclass')).toBeNull()
    expect(collectionOfKind('')).toBeNull()
  })
})

describe('promptLabel', () => {
  it('drops the namespace the character poses its own prompts under', () => {
    expect(promptLabel('character/race')).toBe('Race')
    expect(promptLabel('character/alignment')).toBe('Alignment')
  })

  // The index distinguishes one repeat of a choice from the next; the source
  // stays, because "Proficiency" alone does not say which grant is answered.
  it('keeps the source and drops the index', () => {
    expect(promptLabel('rogue/proficiency/0')).toBe('Rogue · Proficiency')
    expect(promptLabel('skill-versatility/proficiency/0')).toBe('Skill Versatility · Proficiency')
    expect(promptLabel('half-elf/ability-bonus/0')).toBe('Half Elf · Ability Bonus')
  })
})

describe('pickLabel', () => {
  it('title-cases an ordinary option key', () => {
    expect(pickLabel('skill-stealth')).toBe('Skill Stealth')
    expect(pickLabel('dex')).toBe('Dex')
  })

  // A nested option is named by a path, and its own answer is in the same
  // event, so naming the branch is enough.
  it('names the branch a nested option took', () => {
    expect(pickLabel('rogue-expertise-1/expertise/0/0')).toBe('Expertise')
  })

  // An option granting several things at once is named by all of them, so the
  // log row says what was taken. This used to read "#0", which named a
  // position nobody looking at the row could resolve.
  it('reads a composed key back as its parts', () => {
    expect(pickLabel('shortbow+arrow')).toBe('Shortbow, Arrow')
    expect(pickLabel('leather-armor+longbow+arrow')).toBe('Leather Armor, Longbow, Arrow')
  })

  // A bundle can hold a branch, and the branch is named by the pool it draws
  // from -- so the parts of a composed key are not all plain slugs.
  it('reads a branch inside a composed key', () => {
    expect(pickLabel('martial-weapons+shield')).toBe('Martial Weapons, Shield')
    expect(pickLabel('rogue-expertise-1/expertise/0/1/0+thieves-tools')).toBe(
      'Expertise, Thieves Tools',
    )
  })
})
