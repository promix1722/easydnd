import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { renderAt } from '@/test/render'

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
    const user = userEvent.setup()
    const { fetcher, release } = deferred()
    renderAt('desktop', <Probe fetcher={fetcher} />)

    release('first')
    expect(await screen.findByText('first')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'reload' }))
    expect(screen.getByText('loading')).toBeInTheDocument()
  })

  it('leaves what is on screen alone while it refreshes behind it', async () => {
    const user = userEvent.setup()
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
    const user = userEvent.setup()
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
