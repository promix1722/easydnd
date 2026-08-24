import { describe, expect, it } from 'vitest'

import { parseCreationOptions, parseRequestOptions } from './ceremony'
import { toBase64Url } from './encoding'

/**
 * These exercise the hand-rolled decoding path, which is the one that runs in
 * jsdom and in any browser without PublicKeyCredential.parse*FromJSON.
 *
 * The shapes below are what internal/adapter/webauthn actually emits -- see
 * the ceremony adapter's own tests for the server half of the same contract.
 */

const challenge = toBase64Url(new Uint8Array([1, 2, 3, 4]))
const userId = toBase64Url(new TextEncoder().encode('abc123'))

describe('parseCreationOptions', () => {
  const options = {
    publicKey: {
      rp: { name: 'easydnd', id: 'localhost' },
      user: { id: userId, name: 'Alice', displayName: 'Alice' },
      challenge,
      pubKeyCredParams: [{ type: 'public-key', alg: -7 }],
      authenticatorSelection: { residentKey: 'required', requireResidentKey: true },
      attestation: 'none',
    },
  }

  it('decodes the challenge and the user handle into buffers', () => {
    const parsed = parseCreationOptions(options)

    expect(Array.from(new Uint8Array(parsed.challenge as ArrayBuffer))).toEqual([1, 2, 3, 4])
    expect(new TextDecoder().decode(parsed.user.id as ArrayBuffer)).toBe('abc123')
  })

  // A field this module does not understand must survive: the option set grows
  // with the spec, and silently dropping one would disable a feature the
  // server asked for.
  it('passes unknown fields through untouched', () => {
    const parsed = parseCreationOptions(options) as unknown as Record<string, unknown>

    expect(parsed.attestation).toBe('none')
    expect(parsed.pubKeyCredParams).toEqual([{ type: 'public-key', alg: -7 }])
    expect(parsed.authenticatorSelection).toEqual({
      residentKey: 'required',
      requireResidentKey: true,
    })
  })

  it('decodes excludeCredentials when the server sends them', () => {
    const excluded = toBase64Url(new Uint8Array([9, 9]))
    const parsed = parseCreationOptions({
      publicKey: {
        ...options.publicKey,
        excludeCredentials: [{ type: 'public-key', id: excluded }],
      },
    })

    const first = parsed.excludeCredentials?.[0]
    expect(first).toBeDefined()
    expect(Array.from(new Uint8Array(first!.id as ArrayBuffer))).toEqual([9, 9])
  })

  it('reports a missing challenge rather than handing undefined to the browser', () => {
    expect(() =>
      parseCreationOptions({ publicKey: { ...options.publicKey, challenge: undefined } }),
    ).toThrow(/challenge/)
  })
})

describe('parseRequestOptions', () => {
  it('decodes the challenge', () => {
    const parsed = parseRequestOptions({ publicKey: { challenge, userVerification: 'preferred' } })
    expect(Array.from(new Uint8Array(parsed.challenge as ArrayBuffer))).toEqual([1, 2, 3, 4])
  })

  // Sign-in is discoverable: the server sends no allowCredentials, and an
  // implementation that required one would break the usernameless flow.
  it('works when the server names no credentials at all', () => {
    const parsed = parseRequestOptions({ publicKey: { challenge } })
    expect(parsed.allowCredentials).toBeUndefined()
  })
})
