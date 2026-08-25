import { describe, expect, it } from 'vitest'

import { activeNavPath, NAV_ITEMS } from './nav'

describe('the navigation table', () => {
  it('has Groups beside Characters', () => {
    expect(NAV_ITEMS.map((item) => item.to)).toEqual(['/', '/groups'])
  })
})

describe('activeNavPath', () => {
  it('keeps a section lit on its nested routes', () => {
    // The bug this replaced: the desktop navbar matched exactly, so opening a
    // group blanked the highlight while the mobile tab bar kept it.
    expect(activeNavPath('/groups')).toBe('/groups')
    expect(activeNavPath('/groups/grp_1')).toBe('/groups')
    expect(activeNavPath('/groups/join')).toBe('/groups')
  })

  it('does not let the root section swallow everything', () => {
    // '/' is a prefix of every path, so it may only ever match itself.
    expect(activeNavPath('/')).toBe('/')
    expect(activeNavPath('/characters/abc')).toBeNull()
    expect(activeNavPath('/account')).toBeNull()
  })
})
