import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { UpdateRequired } from './UpdateRequired'

/**
 * The action is injected rather than mocked: the suite shares one module
 * registry, so vi.mock cannot be used here -- see the `test` block in
 * vite.config.ts. It is also the only way to test a control whose real job is
 * to reload the page out from under the test that pressed it.
 */
describe('UpdateRequired', () => {
  it('says nothing while this tab is on the deployed release', () => {
    renderAt('desktop', <UpdateRequired opened={false} onReload={() => {}} />)

    expect(screen.queryByRole('button', { name: 'Reload' })).not.toBeInTheDocument()
  })

  for (const viewport of ['mobile', 'desktop'] as const) {
    it(`offers exactly one way out on ${viewport}`, async () => {
      const user = setupUser()
      const onReload = vi.fn()
      renderAt(viewport, <UpdateRequired opened onReload={onReload} />)

      await user.click(screen.getByRole('button', { name: 'Reload' }))
      expect(onReload).toHaveBeenCalledTimes(1)
    })

    it(`cannot be dismissed on ${viewport}`, () => {
      renderAt(viewport, <UpdateRequired opened onReload={() => {}} />)

      // Mantine draws the dismiss affordance as a button named "Close". Its
      // absence is the whole point: dismissing would leave this tab talking to
      // a newer API with older code, which is the failure the dialog exists to
      // prevent.
      expect(screen.queryByRole('button', { name: /close/i })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Reload' })).toBeInTheDocument()
    })
  }

  it('shows the press was taken, because the reload waits for the new worker', async () => {
    const user = setupUser()
    // A reload that never resolves: the real one waits for the service worker
    // to take control, and until it does the button must not look ignored.
    renderAt('desktop', <UpdateRequired opened onReload={() => new Promise(() => {})} />)

    await user.click(screen.getByRole('button', { name: 'Reload' }))
    expect(screen.getByRole('button', { name: 'Reload' })).toHaveAttribute('data-loading', 'true')
  })
})
