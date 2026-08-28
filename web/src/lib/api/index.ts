export { request } from './client'
export type { RequestOptions } from './client'
export {
  ApiError,
  TransportError,
  describeError,
  describeField,
  fieldMessage,
  isApiErrorEnvelope,
} from './errors'
export type { ApiErrorBody, ApiErrorEnvelope, ApiFieldError } from './errors'
export { getVersion } from './system'
export type { VersionResponse } from './system'
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
  Proficiency as CatalogProficiency,
  Race,
  Skill as CatalogSkill,
  Spell,
} from './catalog'

// Folders: where an account files its characters. Not a group of players.
export { createFolder, deleteFolder, listFolders, renameFolder, reorderFolders } from './folders'
export type { Folder } from './folders'

// Characters.
export {
  appendEvents,
  copyCharacter,
  createCharacter,
  createStubCharacter,
  deleteCharacter,
  deleteEvent,
  getEvents,
  getPrompts,
  getSheet,
  importCharacter,
  listCharacters,
  moveCharacter,
  replaceEvent,
  truncateEvents,
} from './characters'
export type {
  Abilities,
  Answer,
  Change,
  CharacterEvent,
  ClassLevel,
  CreateResponse,
  DropReason,
  Dropped,
  Equipment,
  ImportEntry,
  ImportReport,
  ImportResponse,
  Identity,
  LostAnswer,
  NewCharacter,
  Prompt,
  PromptEvent,
  PromptsResponse,
  ReviseResponse,
  Sheet,
  Skill,
  Status,
  Summary,
  WriteResponse,
} from './characters'

// Groups.
export {
  acceptInvite,
  createGroup,
  createInvite,
  deleteGroup,
  getGroup,
  listGroups,
  previewInvite,
  removeMember,
  renameGroup,
  setMemberRole,
} from './groups'
export type {
  GroupDetail,
  GroupMember,
  GroupRole,
  GroupSummary,
  InvitableRole,
  InviteLink,
  InvitePreview,
} from './groups'

// Games. A game is one sitting run by a DM at a group's table -- never a
// "session", which in this barrel means signing in.
export {
  addToGame,
  createGame,
  deleteGame,
  getGame,
  getSharedSheet,
  listGames,
  listTable,
  removeFromGame,
  renameGame,
  shareCharacter,
  unshareCharacter,
} from './games'
export type { GameDetail, GameSummary, TableCharacter } from './games'
