/**
 * Turning a WebAuthn DOMException into something worth showing a person.
 *
 * The names below are the ones that actually reach users; everything else
 * falls through to a generic message rather than leaking a spec term.
 */

export interface CeremonyFailure {
  message: string
  /** True when the person simply backed out -- not an error worth alarming them about. */
  cancelled: boolean
  /** True when this is our misconfiguration rather than anything they did. */
  ourFault: boolean
}

export function describeCeremonyFailure(cause: unknown): CeremonyFailure {
  if (!(cause instanceof DOMException)) {
    return { message: 'Something went wrong. Please try again.', cancelled: false, ourFault: false }
  }

  switch (cause.name) {
    case 'NotAllowedError':
      // The browser deliberately does not distinguish "cancelled" from "timed
      // out" from "no matching passkey", so neither can we.
      return {
        message: 'Cancelled, or no passkey was offered. Try again when you are ready.',
        cancelled: true,
        ourFault: false,
      }
    case 'InvalidStateError':
      return {
        message: 'This device already has a passkey for this account.',
        cancelled: false,
        ourFault: false,
      }
    case 'SecurityError':
      // The page origin does not match the relying party id. A person can do
      // nothing about this, and it should be loud in development.
      return {
        message: 'Passkeys are misconfigured for this site. This is our bug, not yours.',
        cancelled: false,
        ourFault: true,
      }
    case 'AbortError':
      return { message: 'The passkey prompt was closed.', cancelled: true, ourFault: false }
    case 'NotSupportedError':
      return {
        message: 'This device cannot create the kind of passkey this site needs.',
        cancelled: false,
        ourFault: false,
      }
    default:
      return { message: 'The passkey prompt failed. Please try again.', cancelled: false, ourFault: false }
  }
}

/**
 * True when the picker ended without a credential.
 *
 * This is the union of "cancelled", "timed out" and "no passkey was found",
 * because the browser deliberately refuses to distinguish them -- see the
 * NotAllowedError case above. A caller that falls back to registration on this
 * is falling back on the whole union, knowingly: somebody who dismissed the
 * picker on purpose is treated exactly like somebody who had nothing to pick.
 *
 * Deliberately not a check for NotAllowedError by name. Naming spec terms
 * outside this module is the thing this module exists to prevent, and
 * AbortError belongs in the same bucket for the same reason -- the prompt
 * closed and no assertion came back.
 */
export function isCeremonyDismissed(cause: unknown): boolean {
  return describeCeremonyFailure(cause).cancelled
}
