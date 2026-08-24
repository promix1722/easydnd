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

/**
 * True when the device has a built-in authenticator -- Touch ID, Windows
 * Hello, an Android screen lock. Only used to choose the wording: a security
 * key works either way.
 */
export async function hasPlatformAuthenticator(): Promise<boolean> {
  if (!isPasskeySupported()) return false
  try {
    return await globalThis.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable()
  } catch {
    // Some embedded webviews expose the constructor and then throw here.
    return false
  }
}
