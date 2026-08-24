/**
 * base64url <-> ArrayBuffer, the conversion the WebAuthn API needs and does
 * not provide.
 *
 * The server speaks JSON, so every binary field -- challenges, credential ids,
 * the user handle -- crosses the wire as base64url. `navigator.credentials`
 * wants ArrayBuffers, and hands back ArrayBuffers that have to go home as
 * base64url again. Everything here is that translation and nothing else.
 *
 * Written by hand rather than leaning on `PublicKeyCredential.parse*FromJSON`:
 * those helpers are absent from jsdom, so a test suite could never reach the
 * code that matters. `ceremony.ts` prefers the native ones at runtime when the
 * browser has them.
 */

/** Decodes base64url (unpadded, `-` and `_`) into bytes. */
export function fromBase64Url(value: string): ArrayBuffer {
  // atob only understands standard base64 with padding.
  const padded = value.replace(/-/g, '+').replace(/_/g, '/')
  const binary = atob(padded.padEnd(Math.ceil(padded.length / 4) * 4, '='))

  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes.buffer
}

/** Encodes bytes as unpadded base64url. */
export function toBase64Url(value: ArrayBuffer | Uint8Array): string {
  const bytes = value instanceof Uint8Array ? value : new Uint8Array(value)

  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}
