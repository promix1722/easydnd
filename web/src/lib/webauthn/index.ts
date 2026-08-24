export { fromBase64Url, toBase64Url } from './encoding'
export { describeCeremonyFailure, isCeremonyDismissed } from './errors'
export type { CeremonyFailure } from './errors'
export { hasPlatformAuthenticator, isPasskeySupported } from './support'
export {
  createCredential,
  getCredential,
  parseCreationOptions,
  parseRequestOptions,
  serializeAssertion,
  serializeRegistration,
} from './ceremony'
export type { CeremonyOptions, CeremonyResponse } from './ceremony'
