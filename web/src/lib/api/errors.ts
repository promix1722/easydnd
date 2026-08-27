import en from '@locales/en.json'
import type { MessageKey, Translate } from '../i18n'

/**
 * The wire error vocabulary, mirroring internal/api/http/helpers/errors.go.
 * Every failed request from the API arrives in this envelope; nothing else in
 * the app should parse it.
 */

export interface ApiFieldError {
  field: string
  rule?: string
  /** A message key. Absent means "use `rule`", which is a key too. */
  reason?: string
  args?: Record<string, unknown>
}

/**
 * The failure, as keys rather than prose.
 *
 * There is no message. `code` is one of seven and decides the status; `reason`
 * names a sentence in web/locales; `args` carries whatever has to be
 * interpolated into it. The English the error was raised with is in the server
 * log, tagged with this same `request_id` -- see helpers.FormatError.
 */
export interface ApiErrorBody {
  code: string
  reason?: string
  args?: Record<string, unknown>
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
  readonly reason: string | undefined
  readonly args: Record<string, unknown> | undefined
  readonly fields: readonly ApiFieldError[]
  readonly requestId: string | undefined

  constructor(status: number, body: ApiErrorBody) {
    // `Error.message` is for a stack trace and a console, not for a screen:
    // it is the only thing here that cannot be translated, so it says the
    // machine-readable part and nothing a person would be shown.
    super(`${body.code}${body.reason ? `: ${body.reason}` : ''}`)
    this.name = 'ApiError'
    this.status = status
    this.code = body.code
    this.reason = body.reason
    this.args = body.args
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
  // `code` alone. It is the only field the server always sends, and requiring
  // a message here is what would make an otherwise good envelope unreadable
  // the moment prose left the wire.
  return typeof body === 'object' && body !== null && typeof (body as ApiErrorBody).code === 'string'
}

/**
 * Turns a thrown value into something worth showing a person.
 *
 * It lives here rather than in the screen that first needed it because every
 * screen needs it, and because the two error classes above are the only thing
 * it knows about -- keeping it beside them is what stops a second, subtly
 * different version appearing next to the second call site.
 *
 * It takes a translator, because the server no longer sends words. The order
 * is the specific answer first, then the general one:
 *
 *   1. `error.<reason>` -- the sentence the server named.
 *   2. `error.code.<code>` -- what the *kind* of failure means, for a reason
 *      this build has not learned yet. The server may grow one before the
 *      browser does, and a bare slug on screen is worse than a vaguer sentence.
 */
export function describeError(t: Translate, cause: unknown): string {
  if (cause instanceof ApiError) {
    return t(reasonKey(cause.reason, cause.code), cause.args)
  }
  if (cause instanceof TransportError) {
    return t('error.unreachable')
  }
  // Anything else is a thrown Error from inside this client -- a rejected
  // guard, a bug. Its message is English written for whoever is reading the
  // console, and putting it on screen would be the one place prose leaks back
  // into the interface after all the work to get it out. It goes to the
  // console instead, and the reader is told something true and general.
  if (cause instanceof Error) console.error('unhandled failure', cause)
  return t('error.code.server_error')
}

/**
 * What the named field's rejection says, or undefined if it was accepted.
 *
 * The shape a form wants: `error={fieldMessage(t, action.fields, 'name')}`
 * hands Mantine either a sentence or nothing, which is exactly what its
 * `error` prop takes.
 */
export function fieldMessage(
  t: Translate,
  fields: readonly ApiFieldError[],
  name: string,
): string | undefined {
  const found = fields.find((each) => each.field === name)
  return found === undefined ? undefined : describeField(t, found)
}

/** What one rejected field says, in words. */
export function describeField(t: Translate, field: ApiFieldError): string {
  if (field.reason !== undefined && known(`error.${field.reason}`)) {
    return t(`error.${field.reason}` as MessageKey, field.args)
  }
  // `rule` is a small closed vocabulary the server has always sent --
  // required, max, oneof -- and saying "this is required" beats saying nothing.
  return field.rule === undefined ? t('error.code.validation_error') : t(ruleKey(field.rule))
}

/**
 * The catalogue key for a reason, or the best available stand-in.
 *
 * The `error.` prefix is added here rather than sent by the server: a slug on
 * the wire names a *reason*, and where its words live is the client's business.
 */
function reasonKey(reason: string | undefined, code: string): MessageKey {
  if (reason !== undefined && known(`error.${reason}`)) return `error.${reason}` as MessageKey
  const byCode = `error.code.${code}`
  return (known(byCode) ? byCode : 'error.code.server_error') as MessageKey
}

function ruleKey(rule: string): MessageKey {
  const key = `error.rule.${rule}`
  return (known(key) ? key : 'error.code.validation_error') as MessageKey
}

/**
 * Whether the catalogue has a word for this key.
 *
 * Checked against the English file rather than the displayed one, because
 * English is the only locale guaranteed complete -- a key Russian has not
 * reached still resolves, by falling back. Without this a reason the server
 * grew first would render as its own slug.
 */
function known(key: string): boolean {
  return Object.hasOwn(en, key)
}
