import { PALETTE, PALETTE_NAME } from './tokens'
import { PALETTES } from './palettes'
import type { Palette, Scheme } from './palettes'

/**
 * The palettes, checked for the things a screenshot would not tell you.
 *
 * This is the only test that can say anything about colour at all. The suite
 * runs with `css: false` and jsdom lays nothing out, so no test anywhere can
 * read a cascaded style or a computed colour -- which means the bindings in
 * `ui/theme.ts` are unobservable and the *data* is the only surface left to
 * hold. That is enough for the failure that actually matters: a palette
 * shipping unreadable because it was authored on one screen, in one scheme,
 * by somebody who never opened the other.
 */

const HEX = /^#[0-9a-f]{6}$/

const entries = Object.entries(PALETTES) as ReadonlyArray<[string, Palette]>

/** Relative luminance, per WCAG 2.1. */
function luminance(hex: string): number {
  const channel = (pair: string) => {
    const value = Number.parseInt(pair, 16) / 255
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  }
  const r = channel(hex.slice(1, 3))
  const g = channel(hex.slice(3, 5))
  const b = channel(hex.slice(5, 7))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05)
}

describe('the palettes', () => {
  it.each(entries)('%s names itself', (name, palette) => {
    // The record's key and the palette's own name are two places one fact is
    // written, and a switch that reads the key while a generator reads the
    // name is how they would come apart.
    expect(palette.name).toBe(name)
  })

  it.each(entries)('%s has a ten-step accent ramp of six-digit hex', (_name, palette) => {
    expect(palette.accent).toHaveLength(10)
    for (const step of palette.accent) expect(step).toMatch(HEX)
  })

  it.each(entries)('%s draws its brand colour from its own ramp', (_name, palette) => {
    // The invariant that lets `brand` be a string rather than an index into
    // `accent` -- see the field's comment.
    expect(palette.accent).toContain(palette.brand)
  })

  it.each(entries)('%s defines both schemes in full', (_name, palette) => {
    // Both, because AppTheme runs defaultColorScheme="auto": the app never
    // gets to choose which one a visitor sees.
    for (const scheme of [palette.light, palette.dark]) {
      const keys: Array<keyof Scheme> = ['background', 'surface', 'text', 'dimmed', 'border']
      for (const key of keys) expect(scheme[key]).toMatch(HEX)
    }
  })

  it.each(entries)('%s keeps body text readable in both schemes', (_name, palette) => {
    // 4.5:1 is WCAG AA for body text. This is the assertion the whole file is
    // for: it is the one mistake that is invisible to whoever made it, because
    // they were looking at the scheme they designed.
    expect(contrast(palette.light.text, palette.light.background)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(palette.dark.text, palette.dark.background)).toBeGreaterThanOrEqual(4.5)
  })

  it.each(entries)('%s keeps dimmed text at least distinguishable', (_name, palette) => {
    // 3:1 rather than 4.5:1, deliberately. Mantine's own default dimmed is
    // #868e96 on white, which is 3.3:1 -- holding a palette to AA body text
    // here would fail the framework's own choice, and dimmed text is by
    // definition the supporting kind. It still has to be legible.
    expect(contrast(palette.light.dimmed, palette.light.background)).toBeGreaterThanOrEqual(3)
    expect(contrast(palette.dark.dimmed, palette.dark.background)).toBeGreaterThanOrEqual(3)
  })

  it('ships the palette the constant names', () => {
    expect(PALETTE).toBe(PALETTES[PALETTE_NAME])
  })

  it('ships dragon, which is the look the app already had', () => {
    // Not taste: dragon's schemes are Mantine's defaults written out longhand
    // and its accent is the red this project has always used, so switching
    // away and back is a no-op rather than an approximation. If this ever
    // fails, somebody has changed the app's appearance while meaning to add a
    // palette beside it.
    expect(PALETTE_NAME).toBe('dragon')
    expect(PALETTES.dragon.brand).toBe('#99051d')
    expect(PALETTES.dragon.light.background).toBe('#ffffff')
    expect(PALETTES.dragon.dark.background).toBe('#242424')
  })
})
