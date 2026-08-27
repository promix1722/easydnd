import { ApiError, TransportError, isApiErrorEnvelope } from './errors'
import { requestLocale } from './locale'

/**
 * Same-origin by design. nginx routes /v1/ to the Go process and / to this
 * bundle, and the dev server proxies /v1 to 127.0.0.1:8080 -- so the browser
 * is never cross-origin and the API needs no CORS middleware.
 */
const BASE_URL = '/v1'

/** Header name and semantics come from internal/api/http/middleware/requestid.go. */
const HEADER_REQUEST_ID = 'X-Request-Id'

export interface RequestOptions {
  method?: string
  body?: unknown
  signal?: AbortSignal
  /** Override the generated correlation id. Mostly useful in tests. */
  requestId?: string
  /**
   * Send the raw value as the body instead of JSON-encoding it.
   *
   * A WebAuthn ceremony response is already the exact JSON the Go verifier
   * parses, and the signature covers those bytes -- re-encoding it here would
   * risk changing them.
   */
  rawBody?: string
}

function newRequestId(): string {
  // Available in every browser this app targets and in jsdom; the fallback is
  // only for exotic non-secure contexts, where correlation is nice-to-have.
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return Math.random().toString(36).slice(2) + Date.now().toString(36)
}

/**
 * Appends the chosen language to a path.
 *
 * Every route, not just the catalogue: a character sheet names its race by
 * slug and resolves the word through the compendium, but prompts, the import
 * report and the table all carry prose the server has already resolved, and
 * `helpers.Locale` reads the parameter on all of them. Sending it everywhere
 * is cheaper than maintaining a list of which routes speak.
 *
 * `?slugs=` already exists on one route, so the separator is chosen rather
 * than assumed -- appending a second `?` produces a query the server reads as
 * one parameter with a very strange name.
 */
function withLocale(path: string): string {
  return `${path}${path.includes('?') ? '&' : '?'}locale=${requestLocale()}`
}

/**
 * Performs one API call and returns the decoded body.
 *
 * The correlation id is minted here rather than left to the server, so a
 * failure visible in the browser can be grepped straight out of the server
 * logs: internal/logging tags every line of the request with the same id, and
 * the middleware adopts an inbound one instead of replacing it.
 *
 * @throws ApiError when the server answered with the error envelope.
 * @throws TransportError when the server was not reached or spoke nonsense.
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const requestId = options.requestId ?? newRequestId()
  const headers: Record<string, string> = {
    Accept: 'application/json',
    [HEADER_REQUEST_ID]: requestId,
  }

  const init: RequestInit = {
    method: options.method ?? 'GET',
    headers,
    // The session cookie is same-origin and HttpOnly. This is already the
    // browser default; stating it makes the dependency legible, and stops a
    // future refactor to `omit` from silently signing everyone out.
    credentials: 'same-origin',
    ...(options.signal ? { signal: options.signal } : {}),
  }

  if (options.rawBody !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = options.rawBody
  } else if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(options.body)
  }

  let response: Response
  try {
    response = await fetch(`${BASE_URL}${withLocale(path)}`, init)
  } catch (cause) {
    // An aborted request is the caller's own doing, not a network fault --
    // rethrow it untouched so `signal.aborted` checks still work upstream.
    if (cause instanceof DOMException && cause.name === 'AbortError') throw cause
    throw new TransportError(`could not reach the API (${options.method ?? 'GET'} ${path})`, requestId, {
      cause,
    })
  }

  const text = await response.text()
  const payload: unknown = text === '' ? undefined : safeParse(text)

  if (!response.ok) {
    if (isApiErrorEnvelope(payload)) {
      throw new ApiError(response.status, payload.error)
    }
    // A non-envelope failure body means something other than the Go handler
    // answered -- an nginx error page, a proxy timeout, a captive portal.
    throw new TransportError(
      `API returned ${response.status} without an error envelope`,
      response.headers.get(HEADER_REQUEST_ID) ?? requestId,
    )
  }

  // 204 is a body-less success, not a truncated response: DELETE answers
  // with one. Anything else empty is a server that failed to say anything,
  // which is worth reporting rather than handing on as undefined.
  if (response.status === 204) return undefined as T
  if (payload === undefined) {
    throw new TransportError(`API returned an empty body for ${path}`, requestId)
  }
  return payload as T
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}
