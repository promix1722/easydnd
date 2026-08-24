import { describe, expect, it } from 'vitest'

import { fromBase64Url, toBase64Url } from './encoding'

/**
 * Every binary field in a WebAuthn ceremony crosses the wire through these two
 * functions. A padding or alphabet mistake here does not fail loudly -- it
 * produces a challenge the authenticator signs and the server rejects, which
 * looks like "passkeys are broken" and points nowhere near this file.
 */
describe('base64url', () => {
  it('round-trips every byte value', () => {
    const bytes = new Uint8Array(256)
    for (let i = 0; i < 256; i += 1) bytes[i] = i

    const decoded = new Uint8Array(fromBase64Url(toBase64Url(bytes)))
    expect(Array.from(decoded)).toEqual(Array.from(bytes))
  })

  // Lengths 0-3 mod 4 cover every padding case; the server emits unpadded
  // base64url, so all four must decode without one.
  it.each([0, 1, 2, 3, 4, 5, 16, 32])('round-trips a %i-byte value', (length) => {
    const bytes = new Uint8Array(length).map((_, i) => (i * 37) % 256)
    expect(new Uint8Array(fromBase64Url(toBase64Url(bytes)))).toEqual(bytes)
  })

  it('emits the URL-safe alphabet and no padding', () => {
    // 0xfb 0xff produces '+' and '/' under standard base64.
    const encoded = toBase64Url(new Uint8Array([0xfb, 0xff, 0xfe, 0xff, 0xff]))
    expect(encoded).not.toMatch(/[+/=]/)
    expect(encoded).toMatch(/^[A-Za-z0-9_-]+$/)
  })

  it('decodes unpadded input, which is what the server sends', () => {
    // "hello" -> aGVsbG8 (unpadded); atob would reject this without the fix-up.
    expect(new TextDecoder().decode(fromBase64Url('aGVsbG8'))).toBe('hello')
  })

  it('accepts an ArrayBuffer as well as a Uint8Array', () => {
    const bytes = new Uint8Array([1, 2, 3])
    expect(toBase64Url(bytes.buffer)).toBe(toBase64Url(bytes))
  })
})
