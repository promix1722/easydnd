import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { FolderTreeSheet } from './FolderTreeSheet'

function stub() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      const body = url.includes('/v1/folders')
        ? {
            folders: [
              { id: 'fld_1', name: 'Default', default: true },
              { id: 'fld_2', name: 'Curse of Strahd', default: false },
              { id: 'fld_3', name: 'Retired', default: false },
            ],
          }
        : {
            characters: [
              { id: 'chr_1', folder: 'fld_1', name: 'Ada', level: 3, classes: [] },
              { id: 'chr_2', folder: 'fld_2', name: 'Bram', level: 2, classes: [] },
            ],
          }
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
}

function render(viewport: Viewport, seated = new Set<string>()) {
  stub()
  const user = userEvent.setup()
  renderAt(
    viewport,
    withAuth(
      {},
      <MemoryRouter>
        <FolderTreeSheet
          opened
          seated={seated}
          pending={false}
          onClose={() => {}}
          onAdd={() => {}}
        />
      </MemoryRouter>,
    ),
  )
  return { user }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('FolderTreeSheet (%s)', (viewport) => {
  it('shows your folders collapsed, and no character until one is opened', async () => {
    render(viewport)

    await waitFor(() => expect(screen.getByText('Default')).toBeInTheDocument())
    expect(screen.getByText('Curse of Strahd')).toBeInTheDocument()
    expect(screen.queryByText('Ada')).not.toBeInTheDocument()
  })

  it('opens a folder to its characters', async () => {
    const { user } = render(viewport)

    await waitFor(() => expect(screen.getByText('Curse of Strahd')).toBeInTheDocument())
    await user.click(screen.getByText('Curse of Strahd'))
    await waitFor(() => expect(screen.getByText('Bram')).toBeVisible())
    // The other shelf stays shut. Asserted by its control rather than by
    // absence: Mantine keeps a panel's contents mounted once any panel has
    // been opened, and hides them with a collapse.
    expect(screen.getByRole('button', { name: /Default/ })).toHaveAttribute(
      'aria-expanded',
      'false',
    )
  })

  it('leaves out a folder with nothing left to seat', async () => {
    // Retired holds nobody, and Ada is already at the game.
    render(viewport, new Set(['chr_1']))

    await waitFor(() => expect(screen.getByText('Curse of Strahd')).toBeInTheDocument())
    expect(screen.queryByText('Retired')).not.toBeInTheDocument()
    expect(screen.queryByText('Default')).not.toBeInTheDocument()
  })
})
