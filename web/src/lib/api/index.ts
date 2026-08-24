export { request } from './client'
export type { RequestOptions } from './client'
export { ApiError, TransportError, describeError, isApiErrorEnvelope } from './errors'
export type { ApiErrorBody, ApiErrorEnvelope, ApiFieldError } from './errors'
export { getHealth, getVersion } from './system'
export type { HealthResponse, VersionResponse } from './system'
// Sign-in.
export {
  beginLogin,
  beginRegistration,
  finishLogin,
  finishRegistration,
  getSession,
  listProviders,
  signOut,
  ssoLinkUrl,
  ssoStartUrl,
  startGuestSession,
  unlinkProvider,
} from './auth'
export type { AuthProviderInfo, SessionCredential, SessionIdentity, SessionUser } from './auth'

// The compendium.
export {
  bySlug,
  getCollection,
  getEntries,
  getManifest,
  resetCatalogCache,
} from './catalog'
export type {
  Choice,
  Class,
  CollectionInfo,
  Entry,
  Item,
  Manifest,
  Option,
  OptionSet,
  Race,
  Skill as CatalogSkill,
  Spell,
} from './catalog'

// Characters.
export {
  appendEvents,
  createCharacter,
  deleteCharacter,
  getEvents,
  getPrompts,
  getSheet,
  importCharacter,
  listCharacters,
  truncateEvents,
} from './characters'
export type {
  Abilities,
  Answer,
  Change,
  CharacterEvent,
  ClassLevel,
  CreateResponse,
  Equipment,
  ImportEntry,
  ImportReport,
  ImportResponse,
  NewCharacter,
  Prompt,
  PromptEvent,
  PromptsResponse,
  Sheet,
  Skill,
  Status,
  Summary,
  WriteResponse,
} from './characters'
