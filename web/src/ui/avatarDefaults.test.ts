import { describe, expect, it } from 'vitest'

import { AVATAR_RANDOM, characterAvatar, playerAvatar } from './avatarDefaults'

const classes = ['barbarian', 'bard', 'cleric', 'druid', 'fighter', 'monk', 'paladin', 'ranger', 'rogue', 'sorcerer', 'warlock', 'wizard']

describe('default avatars', () => {
  it('uses the starting class, including pack-qualified and multiclass characters', () => {
    for (const name of classes) expect(characterAvatar([{ class: name }])).toBe(`/avatars/${name}.webp`)
    expect(characterAvatar([{ class: 'dnd-2014/rogue' }, { class: 'wizard' }])).toBe('/avatars/rogue.webp')
    expect(characterAvatar()).toBeUndefined()
    expect(characterAvatar([])).toBeUndefined()
    expect(characterAvatar([{ class: 'artificer' }])).toBeUndefined()
  })

  it('gives accounts and classless NPCs stable selections from all 24 random icons', () => {
    const ids = ['dev:master', 'dev:player1', 'dev:player2', 'alice', 'bob', 'anon:guest', 'npc:guard']
    const selected = ids.map(playerAvatar)
    expect(ids.map(playerAvatar)).toEqual(selected)
    expect(new Set(selected).size).toBeGreaterThan(1)
    expect(AVATAR_RANDOM).toHaveLength(24)
    for (const path of selected) expect(AVATAR_RANDOM.map((name) => `/avatars/random/${name}.webp`)).toContain(path)
    expect(new Set(Array.from({ length: 128 }, (_, i) => playerAvatar(`user-${i}`))).size).toBe(24)
  })
})
