import { fromBase64Url, toBase64Url } from './encoding'

/**
 * Runs the two browser-side halves of a WebAuthn ceremony.
 *
 * The server sends `PublicKeyCredentialCreationOptions` / `RequestOptions` as
 * JSON, which means every binary field arrives as a base64url string. This
 * module decodes them, calls `navigator.credentials`, and encodes the result
 * back into the JSON the Go handler passes straight to the verifier.
 *
 * Shapes mirror what internal/adapter/webauthn emits and what
 * protocol.ParseCredentialCreationResponseBody expects on the way back. They
 * are typed loosely on purpose: the option set grows with the spec, and fields
 * we do not touch must survive the round trip untouched rather than being
 * dropped by a narrow interface.
 */

/** The server's `{"publicKey": {...}}` envelope, binary fields still base64url. */
export interface CeremonyOptions {
  publicKey: Record<string, unknown>
}

/** What we post back to the finish endpoint. */
export type CeremonyResponse = Record<string, unknown>

/** Creates a new passkey. */
export async function createCredential(options: CeremonyOptions): Promise<CeremonyResponse> {
  const request = parseCreationOptions(options)
  const credential = await navigator.credentials.create({ publicKey: request })
  if (!(credential instanceof PublicKeyCredential)) {
    throw new Error('the browser returned no credential')
  }
  return serializeRegistration(credential)
}

/** Signs in with an existing passkey. */
export async function getCredential(options: CeremonyOptions): Promise<CeremonyResponse> {
  const request = parseRequestOptions(options)
  const credential = await navigator.credentials.get({ publicKey: request })
  if (!(credential instanceof PublicKeyCredential)) {
    throw new Error('the browser returned no assertion')
  }
  return serializeAssertion(credential)
}

/**
 * Decodes creation options.
 *
 * The native `parseCreationOptionsFromJSON` is preferred where it exists: it
 * tracks the spec, so a field added next year is handled without a change
 * here. The manual path below is the fallback, and is what the tests exercise
 * since jsdom has neither helper.
 */
export function parseCreationOptions(options: CeremonyOptions): PublicKeyCredentialCreationOptions {
  const native = nativeParser<PublicKeyCredentialCreationOptions>('parseCreationOptionsFromJSON')
  if (native) {
    return native(options.publicKey)
  }

  const source = options.publicKey
  const decoded: Record<string, unknown> = {
    ...source,
    challenge: fromBase64Url(requireString(source.challenge, 'challenge')),
    user: {
      ...asRecord(source.user, 'user'),
      id: fromBase64Url(requireString(asRecord(source.user, 'user').id, 'user.id')),
    },
  }
  if (source.excludeCredentials !== undefined) {
    decoded.excludeCredentials = decodeDescriptors(source.excludeCredentials, 'excludeCredentials')
  }
  return decoded as unknown as PublicKeyCredentialCreationOptions
}

/** Decodes request (sign-in) options. */
export function parseRequestOptions(options: CeremonyOptions): PublicKeyCredentialRequestOptions {
  const native = nativeParser<PublicKeyCredentialRequestOptions>('parseRequestOptionsFromJSON')
  if (native) {
    return native(options.publicKey)
  }

  const source = options.publicKey
  const decoded: Record<string, unknown> = {
    ...source,
    challenge: fromBase64Url(requireString(source.challenge, 'challenge')),
  }
  // Absent for a discoverable sign-in, which is the only kind this app does --
  // but decode it if a future flow ever sends one.
  if (source.allowCredentials !== undefined) {
    decoded.allowCredentials = decodeDescriptors(source.allowCredentials, 'allowCredentials')
  }
  return decoded as unknown as PublicKeyCredentialRequestOptions
}

/** Encodes an attestation for the register/finish endpoint. */
export function serializeRegistration(credential: PublicKeyCredential): CeremonyResponse {
  const response = credential.response as AuthenticatorAttestationResponse
  return {
    id: credential.id,
    rawId: toBase64Url(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    clientExtensionResults: credential.getClientExtensionResults(),
    response: {
      clientDataJSON: toBase64Url(response.clientDataJSON),
      attestationObject: toBase64Url(response.attestationObject),
      // Optional, and only present on newer browsers. Sent when available
      // because the verifier will use it rather than re-deriving it.
      ...(typeof response.getTransports === 'function'
        ? { transports: response.getTransports() }
        : {}),
    },
  }
}

/** Encodes an assertion for the login/finish endpoint. */
export function serializeAssertion(credential: PublicKeyCredential): CeremonyResponse {
  const response = credential.response as AuthenticatorAssertionResponse
  return {
    id: credential.id,
    rawId: toBase64Url(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    clientExtensionResults: credential.getClientExtensionResults(),
    response: {
      clientDataJSON: toBase64Url(response.clientDataJSON),
      authenticatorData: toBase64Url(response.authenticatorData),
      signature: toBase64Url(response.signature),
      // The user handle is what makes a usernameless sign-in resolvable: it is
      // the account id, returned by the authenticator rather than typed.
      userHandle: response.userHandle ? toBase64Url(response.userHandle) : null,
    },
  }
}

/**
 * Looks up one of the spec's own JSON parsers, if this browser has it.
 *
 * Reached through globalThis rather than as a bare identifier: a browser
 * without WebAuthn has no PublicKeyCredential binding at all, and naming it
 * directly is a ReferenceError rather than an undefined -- which would turn a
 * graceful "this browser cannot use passkeys" into a blank page.
 */
function nativeParser<T>(name: string): ((json: unknown) => T) | null {
  const constructor = (globalThis as unknown as { PublicKeyCredential?: Record<string, unknown> })
    .PublicKeyCredential
  const parser = constructor?.[name]
  return typeof parser === 'function' ? (parser as (json: unknown) => T) : null
}

function decodeDescriptors(value: unknown, field: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`${field} is not a list`)
  return value.map((entry, index) => {
    const descriptor = asRecord(entry, `${field}[${index}]`)
    return { ...descriptor, id: fromBase64Url(requireString(descriptor.id, `${field}[${index}].id`)) }
  })
}

function asRecord(value: unknown, field: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null) {
    throw new Error(`${field} is missing from the ceremony options`)
  }
  return value as Record<string, unknown>
}

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string') {
    throw new Error(`${field} is missing from the ceremony options`)
  }
  return value
}
