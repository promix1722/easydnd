import { vi } from 'vitest'

/**
 * A fake authenticator, because jsdom has no WebAuthn at all.
 *
 * The ceremony code is otherwise untestable, and the part of it worth testing
 * is a branch on an error path -- what happens when the picker comes back
 * empty-handed. That branch decides whether a first-time visitor gets an
 * account or an error message, and it cannot be reached without something
 * standing in for `navigator.credentials`.
 *
 * Nothing here is cryptographically meaningful. The server is a fetch mock in
 * these tests, so the bytes only have to survive being base64url'd on the way
 * out; no attestation is built and nothing verifies one.
 */

/**
 * Stands in for the browser's credential class.
 *
 * It has to be a real class installed as the global, because ceremony.ts
 * checks `credential instanceof PublicKeyCredential` before trusting what the
 * browser handed back -- an object literal is rejected there, which is the
 * point of the check.
 *
 * Deliberately carries no `parseCreationOptionsFromJSON` or
 * `parseRequestOptionsFromJSON` statics. `nativeParser` prefers the spec's own
 * parsers when a browser has them, so adding them here would route these tests
 * around the hand-rolled decoding that every real jsdom run uses.
 */
class FakePublicKeyCredential {
  id = 'fake-credential'
  rawId = new Uint8Array([1, 2, 3]).buffer
  type = 'public-key'
  authenticatorAttachment = 'platform'
  response: object

  constructor(response: object) {
    this.response = response
  }

  getClientExtensionResults() {
    return {}
  }
}

const bytes = () => new Uint8Array([4, 5, 6]).buffer

/** What a create() resolves to: enough for serializeRegistration. */
export function fakeAttestation(): unknown {
  return new FakePublicKeyCredential({
    clientDataJSON: bytes(),
    attestationObject: bytes(),
    getTransports: () => ['internal'],
  })
}

/** What a get() resolves to: enough for serializeAssertion. */
export function fakeAssertion(): unknown {
  return new FakePublicKeyCredential({
    clientDataJSON: bytes(),
    authenticatorData: bytes(),
    signature: bytes(),
    userHandle: bytes(),
  })
}

/**
 * The options envelope a begin endpoint returns.
 *
 * challenge and user.id must be real base64url, or parseCreationOptions
 * rejects them before the authenticator is ever reached.
 */
export const fakeOptions = {
  publicKey: { challenge: 'AAAA', user: { id: 'AAAA', name: 'x', displayName: 'x' } },
}

export interface FakeAuthenticator {
  create: ReturnType<typeof vi.fn>
  get: ReturnType<typeof vi.fn>
}

/**
 * Installs the fake for one test and returns its two spies.
 *
 * navigator.credentials is defined onto the existing navigator rather than
 * stubbed wholesale: replacing the whole object breaks userEvent, which reads
 * navigator.clipboard. The caller undoes it with removeAuthenticator.
 */
export function fakeAuthenticator(): FakeAuthenticator {
  vi.stubGlobal('PublicKeyCredential', FakePublicKeyCredential)

  const credentials: FakeAuthenticator = {
    create: vi.fn().mockResolvedValue(fakeAttestation()),
    get: vi.fn().mockResolvedValue(fakeAssertion()),
  }
  Object.defineProperty(navigator, 'credentials', { value: credentials, configurable: true })
  return credentials
}

/** Takes the fake back off, for an afterEach. */
export function removeAuthenticator(): void {
  Reflect.deleteProperty(navigator, 'credentials')
}

/** The error a browser raises when the picker closes with nothing chosen. */
export function dismissed(): DOMException {
  return new DOMException('the operation is not allowed', 'NotAllowedError')
}
