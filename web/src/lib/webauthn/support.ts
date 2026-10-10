/**
 * What the current browser can actually do.
 *
 * There is no password to fall back on, so a browser without WebAuthn cannot
 * use this app at all. Saying that plainly is better than rendering a button
 * that throws.
 */

/** True when the browser exposes the WebAuthn API at all. */
export function isPasskeySupported(): boolean {
  // Through globalThis, and by property rather than by name: on a browser with
  // no WebAuthn there is no binding to reference, and a bare identifier would
  // throw instead of answering the question.
  return typeof (globalThis as { PublicKeyCredential?: unknown }).PublicKeyCredential === 'function'
}
