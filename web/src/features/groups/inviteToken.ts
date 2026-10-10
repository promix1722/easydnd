/**
 * Holding on to an invitation across a sign-in.
 *
 * Somebody who follows an invite link may well not have an account yet, and
 * the way in costs them a page: the header's "Log in" leaves `/groups/join`,
 * and Google leaves the origin entirely. The token is in the URL *fragment*,
 * which is the safest place to carry it and the least durable -- it survives
 * neither of those trips.
 *
 * So it is copied somewhere that does survive them, the moment the route is
 * reached and before anything can navigate away. sessionStorage rather than
 * localStorage: an invitation is one visit's business, and one left behind in
 * a shared browser is somebody else's group.
 */
const STASH_KEY = 'easydnd.invite'

/**
 * Where a copy link's token is kept. A second key rather than a second module:
 * the trip it has to survive is the same one, and a key of its own is all that
 * stops it and a group invitation overwriting each other.
 */
export const COPY_LINK_STASH_KEY = 'easydnd.copyLink'

/** The fragment, minus its '#'. */
function fromHash(): string {
  return window.location.hash.replace(/^#/, '')
}

/**
 * Records the token this visit arrived with, and returns it.
 *
 * Called from the route rather than the screen, because a signed-out visitor
 * never reaches the screen -- the route renders the invitation prompt instead,
 * and by then the fragment has to be saved already.
 */
export function captureInviteToken(key = STASH_KEY): string {
  const token = fromHash()
  if (token === '') return readInviteToken(key)
  try {
    window.sessionStorage.setItem(key, token)
  } catch {
    // A private-mode browser can refuse storage outright. The token is still
    // in the fragment, so everything works until they leave the page -- which
    // is strictly better than failing here.
  }
  return token
}

/** The token for this visit: whatever is in the URL, else what was saved. */
export function readInviteToken(key = STASH_KEY): string {
  const token = fromHash()
  if (token !== '') return token
  try {
    return window.sessionStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

/** Forgets the invitation, once it has been accepted or declined. */
export function clearInviteToken(key = STASH_KEY): void {
  try {
    window.sessionStorage.removeItem(key)
  } catch {
    // Nothing was stored, so there is nothing to forget.
  }
}

/** The token in a pasted invitation: what follows the link's '#', or the text itself when only the token was copied. */
export function tokenOfLink(pasted: string): string {
  const text = pasted.trim()
  return text.slice(text.lastIndexOf('#') + 1).trim()
}

/** What a link offers: a seat in a group, or a copy of a character. */
export type InvitationKind = 'group' | 'character'

/**
 * Which of the two a link is, from the link and then from the token.
 *
 * The path says it when there is one: the two kinds are sent as two addresses.
 * A bare token says it itself -- it is a signed token whose `knd` claim the
 * server checks, read here only to choose which preview to ask for. Reading it
 * wrong costs a refused preview, never a wrong grant.
 */
export function kindOfLink(link: string): InvitationKind {
  if (link.includes('/characters/receive')) return 'character'
  if (link.includes('/groups/join')) return 'group'
  try {
    const payload = tokenOfLink(link).split('.')[1] ?? ''
    const claims = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/'))) as { knd?: string }
    return claims.knd === 'copylink' ? 'character' : 'group'
  } catch {
    return 'group'
  }
}
