import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { useResource } from './useResource'

/** A fetcher that does not answer until the test says so. */
function deferred() {
  let release: (value: string) => void = () => {}
  const fetcher = () =>
    new Promise<string>((resolve) => {
      release = resolve
    })
  return { fetcher, release: (value: string) => release(value) }
}

function Probe({ fetcher }: { fetcher: () => Promise<string> }) {
  const resource = useResource('probe', fetcher)
  return (
    <div>
      <p>{resource.loading ? 'loading' : (resource.data ?? resource.error)}</p>
      <button onClick={resource.reload}>reload</button>
      <button onClick={resource.refresh}>refresh</button>
    </div>
  )
}

describe('useResource', () => {
  it('takes the screen down to reload, because a reload has nothing to show', async () => {
    const user = setupUser()
    const { fetcher, release } = deferred()
    renderAt('desktop', <Probe fetcher={fetcher} />)

    release('first')
    expect(await screen.findByText('first')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'reload' }))
    expect(screen.getByText('loading')).toBeInTheDocument()
  })

  it('leaves what is on screen alone while it refreshes behind it', async () => {
    const user = setupUser()
    const { fetcher, release } = deferred()
    renderAt('desktop', <Probe fetcher={fetcher} />)

    release('first')
    expect(await screen.findByText('first')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'refresh' }))

    // The whole point: a write the server has already confirmed should not
    // replace the list somebody is reading with a spinner and rebuild it
    // underneath them.
    expect(screen.getByText('first')).toBeInTheDocument()
    expect(screen.queryByText('loading')).not.toBeInTheDocument()

    release('second')
    await waitFor(() => {
      expect(screen.getByText('second')).toBeInTheDocument()
    })
  })

  it('says so when a refresh fails rather than showing what it could not check', async () => {
    const user = setupUser()
    let attempt = 0
    const fetcher = () => {
      attempt += 1
      return attempt === 1 ? Promise.resolve('first') : Promise.reject(new Error('gone'))
    }
    renderAt('desktop', <Probe fetcher={fetcher} />)

    expect(await screen.findByText('first')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'refresh' }))

    // Data quietly out of date is worse than a screen that admits it stopped
    // being able to check.
    await waitFor(() => {
      expect(screen.queryByText('first')).not.toBeInTheDocument()
    })
  })
})

function Reader({ fetcher }: { fetcher: (signal: AbortSignal) => Promise<string> }) {
  const resource = useResource('game:test', fetcher, { pollInterval: 3000, retainOnRefreshError: true })
  return <div>{resource.data}<button onClick={resource.refresh}>refresh</button></div>
}

afterEach(() => { vi.useRealTimers() })

it('polls every three seconds while visible and ignores aborted stale responses', async () => {
  vi.useFakeTimers()
  let resolveOld!: (value: string) => void
  const fetcher = vi.fn<(signal: AbortSignal) => Promise<string>>()
    .mockResolvedValueOnce('initial')
    .mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    .mockResolvedValueOnce('newest')
  renderAt('desktop', <Reader fetcher={fetcher} />)
  await act(async () => {})
  expect(screen.getByText('initial')).toBeInTheDocument()
  await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
  expect(fetcher).toHaveBeenCalledTimes(2)
  await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
  expect(fetcher).toHaveBeenCalledTimes(2)
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'refresh' })) })
  expect(fetcher.mock.calls[1]![0].aborted).toBe(true)
  expect(screen.getByText('newest')).toBeInTheDocument()
  await act(async () => { resolveOld('stale') })
  expect(screen.queryByText('stale')).not.toBeInTheDocument()
})

it('pauses polling while hidden and refreshes when visibility returns', async () => {
  vi.useFakeTimers()
  const fetcher = vi.fn(async () => 'initial')
  const visible = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
  const view = renderAt('desktop', <Reader fetcher={fetcher} />)
  await act(async () => {})
  await act(async () => { await vi.advanceTimersByTimeAsync(6000) })
  expect(fetcher).toHaveBeenCalledTimes(1)
  visible.mockReturnValue('visible')
  await act(async () => { document.dispatchEvent(new Event('visibilitychange')) })
  expect(fetcher).toHaveBeenCalledTimes(2)
  view.unmount()
  await act(async () => { await vi.advanceTimersByTimeAsync(6000) })
  expect(fetcher).toHaveBeenCalledTimes(2)
  visible.mockRestore()
})
