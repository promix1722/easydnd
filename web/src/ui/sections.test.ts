import { describe, expect, it } from 'vitest'

import { SECTIONS, sectionFor } from './sections'

describe('the section table', () => {
  it('has Characters, Groups and Games', () => {
    expect(SECTIONS.map((section) => section.to)).toEqual(['/', '/groups', '/games'])
  })

  // The mobile dropdown uses labels as accessible names, so two the same would
  // make a menu item impossible to address -- in a test or with a screen
  // reader.
  it('labels every section, distinctly', () => {
    const labels = SECTIONS.map((section) => section.label)

    expect(labels.every((label) => label.length > 0)).toBe(true)
    expect(new Set(labels).size).toBe(labels.length)
  })

  it('gives every section a glyph', () => {
    // The navbar, the phone dropdown and every breadcrumb draw one, so a
    // section without it renders a hole in three places rather than one.
    // Defined, not callable: a tabler glyph is a forwardRef object rather
    // than a plain function, so `typeof` reports 'object' for all of them.
    expect(SECTIONS.every((section) => section.icon !== undefined)).toBe(true)
  })
})

describe('sectionFor', () => {
  it('keeps a section lit on its nested routes', () => {
    // The bug this replaced: the desktop navbar matched exactly, so opening a
    // group blanked the highlight while the mobile chrome kept it.
    expect(sectionFor('/groups')?.label).toBe('Groups')
    expect(sectionFor('/groups/grp_1')?.label).toBe('Groups')
    expect(sectionFor('/groups/join')?.label).toBe('Groups')
  })

  it('lights Games on a game, not Groups', () => {
    // A game is its own section: it is played at a table but it is not
    // reached through one, and the highlight has to say so.
    expect(sectionFor('/games')?.label).toBe('Games')
    expect(sectionFor('/games/gam_1')?.label).toBe('Games')
  })

  it('claims a character sheet for Characters', () => {
    // Inverted deliberately. `activeNavPath` used to answer null here, because
    // matching on the link target alone meant Characters could only ever be
    // `/` exactly. A breadcrumb on this page has to start at "Characters", and
    // a trail saying one thing while the navbar says nothing is the drift the
    // table exists to prevent -- so the section now owns `/characters` as well
    // as linking to `/`.
    expect(sectionFor('/characters/abc')?.label).toBe('Characters')
    expect(sectionFor('/characters/abc/log')?.label).toBe('Characters')
    expect(sectionFor('/characters/new')?.label).toBe('Characters')
  })

  it('does not let the root section swallow everything', () => {
    // '/' is a prefix of every path, so it is matched exactly and never as a
    // prefix. That property is what `owns` was split out to preserve.
    expect(sectionFor('/')?.label).toBe('Characters')
    expect(sectionFor('/account')).toBeNull()
    expect(sectionFor('/login')).toBeNull()
    expect(sectionFor('/nonsense')).toBeNull()
  })

  it('matches whole path segments, not string prefixes', () => {
    // Without the trailing slash in the prefix test this answers Groups, which
    // is what the old implementation did.
    expect(sectionFor('/groupsfoo')).toBeNull()
    expect(sectionFor('/gameslist')).toBeNull()
  })
})
