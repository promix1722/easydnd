import { request } from './client'
import type { CeremonyOptions, CeremonyResponse } from '@/lib/webauthn'

/**
 * The sign-in endpoints, mirroring internal/api/http/v1/auth.
 *
 * Every call is a POST except the bootstrap, and every one of them carries the
 * X-Request-Id header the client already mints -- which the server's CSRF
 * guard requires, because an HTML form cannot set a custom header.
 */

/** One registered passkey, as GET /v1/auth/me reports it. */
export interface SessionCredential {
  id: string
  created_at: string
  last_used_at: string
  /**
   * True when the passkey syncs to a password manager or cloud keychain. A
   * device-bound passkey is a single point of failure for an account that has
   * no recovery path, so the UI says so.
   */
  backed_up: boolean
}

/** One linked external account, as GET /v1/auth/me reports it. */
export interface SessionIdentity {
  /** The provider id, e.g. 'google'. */
  provider: string
  /**
   * The provider's own stable id for the person. Unlinking has to name one,
   * because an account may hold more than one identity from the same provider.
   */
  subject: string
  email?: string
  display_name?: string
  created_at: string
  last_used_at: string
}

/** The signed-in account -- or, when `anonymous`, the guest standing in for one. */
export interface SessionUser {
  id: string
  display_name: string
  created_at: string
  credentials: SessionCredential[]
  identities: SessionIdentity[]
  /**
   * True when no account backs this session: a guest, whose id lives in the
   * token and nowhere else. Such a session can hold no passkeys and no linked
   * identities and cannot be signed back into, so anything offering account
   * management has to check it.
   */
  anonymous: boolean
}

/** One external sign-in method this deployment offers. */
export interface AuthProviderInfo {
  id: string
  /** What a button should say, e.g. 'Google'. */
  name: string
}

interface SessionResponse {
  user: SessionUser
}

/** Fetches the current account. Throws ApiError with status 401 when signed out. */
export async function getSession(signal?: AbortSignal): Promise<SessionUser> {
  const response = await request<SessionResponse>('/auth/me', {
    ...(signal ? { signal } : {}),
  })
  return response.user
}

/** Starts a sign-up. Takes no argument: the server names the account. */
export function beginRegistration(): Promise<CeremonyOptions> {
  return request<CeremonyOptions>('/auth/register/begin', { method: 'POST', body: {} })
}

/** Completes a sign-up and establishes the session. */
export async function finishRegistration(response: CeremonyResponse): Promise<SessionUser> {
  const body = await request<SessionResponse>('/auth/register/finish', {
    method: 'POST',
    rawBody: JSON.stringify(response),
  })
  return body.user
}

/** Starts a sign-in. Takes no argument: the browser picks the passkey. */
export function beginLogin(): Promise<CeremonyOptions> {
  return request<CeremonyOptions>('/auth/login/begin', { method: 'POST', body: {} })
}

/** Completes a sign-in and establishes the session. */
export async function finishLogin(response: CeremonyResponse): Promise<SessionUser> {
  const body = await request<SessionResponse>('/auth/login/finish', {
    method: 'POST',
    rawBody: JSON.stringify(response),
  })
  return body.user
}

/**
 * Starts a guest session -- one request, no ceremony, no account.
 *
 * The way in for someone who will not, or cannot, make a passkey. What comes
 * back is a session like any other except that nothing behind it is stored, so
 * whatever the guest builds ends when the token does.
 */
export async function startGuestSession(): Promise<SessionUser> {
  const body = await request<SessionResponse>('/auth/anonymous', { method: 'POST', body: {} })
  return body.user
}

/**
 * Lists the external providers this deployment can offer.
 *
 * The credentials behind each are optional configuration, so a button drawn
 * without asking would be a dead end on a deployment that never set them.
 */
export async function listProviders(signal?: AbortSignal): Promise<AuthProviderInfo[]> {
  const body = await request<{ providers: AuthProviderInfo[] }>('/auth/providers', {
    ...(signal ? { signal } : {}),
  })
  return body.providers
}

/**
 * Where to send the browser to sign in with an external provider.
 *
 * A URL rather than a request, and the distinction is the whole design: this
 * must be a top-level navigation. Fetching it would follow the redirect as an
 * XHR, land the provider's consent page in a JavaScript string, and set no
 * cookie anywhere -- so it is deliberately not routed through request().
 */
export function ssoStartUrl(provider: string, returnTo?: string): string {
  return ssoUrl(provider, 'start', returnTo)
}

/** Where to send the browser to connect a provider to the signed-in account. */
export function ssoLinkUrl(provider: string, returnTo?: string): string {
  return ssoUrl(provider, 'link', returnTo)
}

function ssoUrl(provider: string, action: 'start' | 'link', returnTo?: string): string {
  const base = `/v1/auth/sso/${encodeURIComponent(provider)}/${action}`
  return returnTo ? `${base}?return_to=${encodeURIComponent(returnTo)}` : base
}

/**
 * Disconnects an external account.
 *
 * A POST, unlike the rest of the SSO flow, because it changes something and so
 * must travel through the CSRF guard that safe methods are exempt from.
 */
export async function unlinkProvider(provider: string, subject: string): Promise<SessionUser> {
  const body = await request<SessionResponse>(
    `/auth/sso/${encodeURIComponent(provider)}/unlink`,
    { method: 'POST', body: { subject } },
  )
  return body.user
}

/** Clears the session cookie. */
export async function signOut(): Promise<void> {
  await request<{ signed_out: boolean }>('/auth/logout', { method: 'POST', body: {} })
}
