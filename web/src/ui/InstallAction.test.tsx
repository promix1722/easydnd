import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { InstallAction } from './InstallAction'

/**
 * The action is injected rather than mocked -- the suite shares one module
 * registry, so `vi.mock` is banned repo-wide -- and it is also the only way to
 * assert on a control whose real job is to open a browser dialog no test can
 * see.
 */
describe('InstallAction', () => {
  it('draws nothing when there is nothing to offer', () => {
    // Already installed, or a browser that cannot: the header is exactly as it
    // was, which is the point of rendering null rather than a disabled button.
    renderAt('desktop', <InstallAction offer="none" onInstall={() => {}} />)

    expect(screen.queryByRole('button', { name: 'Install' })).not.toBeInTheDocument()
  })

  for (const viewport of ['mobile', 'desktop'] as const) {
    it(`asks the browser to install on ${viewport}`, async () => {
      const user = setupUser()
      const onInstall = vi.fn()
      renderAt(viewport, <InstallAction offer="prompt" onInstall={onInstall} />)

      await user.click(screen.getByRole('button', { name: 'Install' }))
      expect(onInstall).toHaveBeenCalledTimes(1)
    })

    it(`tells an iOS visitor what to tap on ${viewport}`, async () => {
      const user = setupUser()
      const onInstall = vi.fn()
      renderAt(viewport, <InstallAction offer="ios" onInstall={onInstall} />)

      await user.click(screen.getByRole('button', { name: 'Install' }))

      // iOS has no install API at all, so there is nothing to call -- the two
      // taps are the whole mechanism and the sheet has to name them.
      expect(await screen.findByText(/Share button/)).toBeInTheDocument()
      expect(screen.getByText(/Add to Home Screen/)).toBeInTheDocument()
      expect(onInstall).not.toHaveBeenCalled()
    })
  }
})
