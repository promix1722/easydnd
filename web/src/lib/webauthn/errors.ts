/**
 * Turning a WebAuthn DOMException into something worth showing a person.
 *
 * The names below are the ones that actually reach users; everything else
 * falls through to a generic message rather than leaking a spec term.
 *
 * What comes back is a message *key*, not a sentence. This module is a
 * classifier -- it decides which of six things went wrong -- and the words for
 * those six live in web/locales/*.json with every other caption, so that
 * translating the failure a person is most likely to hit does not mean editing
 * a TypeScript file. The caller translates; `lib/auth/AuthProvider` is the one
 * that does.
 */
import type { MessageKey } from '@/lib/i18n'

export interface CeremonyFailure {
  reason: MessageKey
  /** True when the person simply backed out -- not an error worth alarming them about. */
  cancelled: boolean
  /** True when this is our misconfiguration rather than anything they did. */
  ourFault: boolean
}

export function describeCeremonyFailure(cause: unknown): CeremonyFailure {
  if (!(cause instanceof DOMException)) {
    return { reason: 'passkey.generic', cancelled: false, ourFault: false }
  }

  switch (cause.name) {
    case 'NotAllowedError':
      // The browser deliberately does not distinguish "cancelled" from "timed
      // out" from "no matching passkey", so neither can we.
      return {
        reason: 'passkey.notAllowed',
        cancelled: true,
        ourFault: false,
      }
    case 'InvalidStateError':
      return {
        reason: 'passkey.alreadyRegistered',
        cancelled: false,
        ourFault: false,
      }
    case 'SecurityError':
      // The page origin does not match the relying party id. A person can do
      // nothing about this, and it should be loud in development.
      return {
        reason: 'passkey.misconfigured',
        cancelled: false,
        ourFault: true,
      }
    case 'AbortError':
      return { reason: 'passkey.closed', cancelled: true, ourFault: false }
    case 'NotSupportedError':
      return {
        reason: 'passkey.unsupported',
        cancelled: false,
        ourFault: false,
      }
    default:
      return { reason: 'passkey.failed', cancelled: false, ourFault: false }
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
