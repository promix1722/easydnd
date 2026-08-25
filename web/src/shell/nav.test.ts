import { describe, expect, it } from 'vitest'

import { activeNavPath, NAV_ITEMS, navLabel } from './nav'

describe('the navigation table', () => {
  it('has Characters, Groups and Games', () => {
    expect(NAV_ITEMS.map((item) => item.to)).toEqual(['/', '/groups', '/games'])
  })

  // The mobile dropdown uses labels as accessible names, so two the same would
  // make a menu item impossible to address -- in a test or with a screen
  // reader.
  it('labels every section, distinctly', () => {
    const labels = NAV_ITEMS.map((item) => item.label)

    expect(labels.every((label) => label.length > 0)).toBe(true)
    expect(new Set(labels).size).toBe(labels.length)
  })
})

describe('activeNavPath', () => {
  it('keeps a section lit on its nested routes', () => {
    // The bug this replaced: the desktop navbar matched exactly, so opening a
    // group blanked the highlight while the mobile chrome kept it.
    expect(activeNavPath('/groups')).toBe('/groups')
    expect(activeNavPath('/groups/grp_1')).toBe('/groups')
    expect(activeNavPath('/groups/join')).toBe('/groups')
    expect(activeNavPath('/games')).toBe('/games')
  })

  it('lights Games on a game, not Groups', () => {
    // A game is its own section: it is played at a table but it is not
    // reached through one, and the highlight has to say so.
    expect(activeNavPath('/games')).toBe('/games')
    expect(activeNavPath('/games/gam_1')).toBe('/games')
  })

  it('does not let the root section swallow everything', () => {
    // '/' is a prefix of every path, so it may only ever match itself.
    expect(activeNavPath('/')).toBe('/')
    expect(activeNavPath('/characters/abc')).toBeNull()
    expect(activeNavPath('/account')).toBeNull()
  })
})

describe('navLabel', () => {
  it('names the section a path belongs to', () => {
    expect(navLabel('/')).toBe('Characters')
    expect(navLabel('/groups/grp_1')).toBe('Groups')
    expect(navLabel('/games')).toBe('Games')
  })

  it('falls back to the word for the control on a path in no section', () => {
    // The desktop navbar can leave every entry unlit here; the dropdown is one
    // control and cannot be left unlabelled.
    expect(navLabel('/characters/abc')).toBe('Menu')
    expect(navLabel('/account')).toBe('Menu')
  })
})
