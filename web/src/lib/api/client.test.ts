import { afterEach, describe, expect, it, vi } from 'vitest'

import { request } from './client'
import { ApiError, TransportError } from './errors'

function respond(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(typeof body === 'string' ? body : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request', () => {
  it('returns the decoded body on success and sends a correlation id', async () => {
    const fetchMock = vi.fn().mockResolvedValue(respond(200, { version: 'abc123' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(request<{ version: string }>('/version')).resolves.toEqual({ version: 'abc123' })

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/v1/version')
    const headers = init.headers as Record<string, string>
    expect(headers['X-Request-Id']).toMatch(/.+/)
    expect(headers.Accept).toBe('application/json')
  })

  it('decodes the Go error envelope into an ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        respond(422, {
          error: {
            code: 'validation_failed',
            message: 'name is required',
            fields: [{ field: 'name', rule: 'required' }],
            request_id: 'req-7',
          },
        }),
      ),
    )

    const error = await request('/characters', { method: 'POST', body: {} }).catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    const apiError = error as ApiError
    expect(apiError.status).toBe(422)
    expect(apiError.code).toBe('validation_failed')
    expect(apiError.message).toBe('name is required')
    expect(apiError.fields).toEqual([{ field: 'name', rule: 'required' }])
    expect(apiError.requestId).toBe('req-7')
    expect(apiError.isRetryable).toBe(false)
  })

  it('marks server faults retryable', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(respond(503, { error: { code: 'internal', message: 'boom' } })),
    )

    const error = (await request('/version').catch((e: unknown) => e)) as ApiError
    expect(error).toBeInstanceOf(ApiError)
    expect(error.isRetryable).toBe(true)
  })

  it('reports an unreachable API as a TransportError, not an ApiError', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))

    const error = await request('/version').catch((e: unknown) => e)

    expect(error).toBeInstanceOf(TransportError)
    expect(error).not.toBeInstanceOf(ApiError)
    expect((error as TransportError).message).toContain('could not reach the API')
  })

  it('treats a non-envelope failure body as a transport fault', async () => {
    // What an nginx error page looks like to the client.
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(502, '<html>Bad Gateway</html>')))

    const error = await request('/version').catch((e: unknown) => e)

    expect(error).toBeInstanceOf(TransportError)
    expect((error as TransportError).message).toContain('502')
  })

  it('rethrows an abort untouched so callers can detect their own cancellation', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new DOMException('aborted', 'AbortError')))

    const error = await request('/version').catch((e: unknown) => e)

    expect(error).toBeInstanceOf(DOMException)
    expect((error as DOMException).name).toBe('AbortError')
  })
})
