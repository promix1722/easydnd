/**
 * The wire error vocabulary, mirroring internal/api/http/helpers/errors.go.
 * Every failed request from the API arrives in this envelope; nothing else in
 * the app should parse it.
 */

export interface ApiFieldError {
  field: string
  rule?: string
  message?: string
}

export interface ApiErrorBody {
  code: string
  message: string
  fields?: ApiFieldError[]
  request_id?: string
}

export interface ApiErrorEnvelope {
  error: ApiErrorBody
}

/** The server answered, and said no. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: readonly ApiFieldError[]
  readonly requestId: string | undefined

  constructor(status: number, body: ApiErrorBody) {
    super(body.message || `request failed with ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.code = body.code
    this.fields = body.fields ?? []
    this.requestId = body.request_id
  }

  /** True when retrying the identical request could plausibly succeed. */
  get isRetryable(): boolean {
    return this.status >= 500 || this.status === 429
  }
}

/**
 * The server was never reached, or answered with something that is not the
 * envelope -- offline, DNS failure, a proxy error page, a truncated body.
 *
 * Kept distinct from ApiError so the UI can say "you appear to be offline"
 * rather than inventing a status code the server never sent. `requestId` is
 * still carried when we minted one, because the request may well have reached
 * the server and been logged there before the connection died.
 */
export class TransportError extends Error {
  readonly requestId: string | undefined

  constructor(message: string, requestId?: string, options?: { cause?: unknown }) {
    super(message, options)
    this.name = 'TransportError'
    this.requestId = requestId
  }
}

/** Narrows an unknown value to the API's error envelope. */
export function isApiErrorEnvelope(value: unknown): value is ApiErrorEnvelope {
  if (typeof value !== 'object' || value === null || !('error' in value)) return false
  const body = (value as { error: unknown }).error
  return (
    typeof body === 'object' &&
    body !== null &&
    typeof (body as ApiErrorBody).code === 'string' &&
    typeof (body as ApiErrorBody).message === 'string'
  )
}

/**
 * Turns a thrown value into something worth showing a person.
 *
 * It lives here rather than in the screen that first needed it because every
 * screen needs it, and because the two error classes above are the only thing
 * it knows about -- keeping it beside them is what stops a second, subtly
 * different version appearing next to the second call site.
 */
export function describeError(cause: unknown): string {
  if (cause instanceof ApiError) {
    const id = cause.requestId ? ` (request ${cause.requestId})` : ''
    return `${cause.code}: ${cause.message}${id}`
  }
  if (cause instanceof TransportError) {
    return `${cause.message} - is the API running?`
  }
  return cause instanceof Error ? cause.message : String(cause)
}
