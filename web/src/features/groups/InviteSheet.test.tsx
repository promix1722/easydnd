import { screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { InviteSheet } from './InviteSheet'

// The component takes its clipboard as a prop, so this touches no global at
// all: no vi.mock, and nothing for the next test file in this worker to trip
// over. Poking the real navigator.clipboard would not work anyway -- userEvent
// installs a stub of its own over it -- and what the real copyText does with a
// missing clipboard is pinned in lib/clipboard.test.ts.
const copyLink = vi.fn(async (_text: string) => true)

const TOKEN = 'a.b.c'

function stubFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify({ token: TOKEN, role: 'player', expires_at: '' }), {
          status: 201,
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )
}

/** Opens the sheet, mints a link, and hands back the copy button. */
async function mintLink() {
  renderAt(
    'desktop',
    withAuth({}, <InviteSheet groupId="grp_1" opened onClose={() => {}} copyLink={copyLink} />),
  )
  await setupUser().click(screen.getByRole('button', { name: 'Create link' }))
  return await screen.findByRole('button', { name: 'Copy link' })
}

beforeEach(() => {
  copyLink.mockReset()
  copyLink.mockResolvedValue(true)
  stubFetch()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('the invitation link', () => {
  it('puts the token in the fragment, never in the query string', async () => {
    await mintLink()
    // A fragment is the one part of a URL no browser sends to any server, so
    // the token stays out of nginx's access log and out of Referer.
    const field = screen.getByLabelText('Invitation link') as HTMLInputElement
    expect(field.value).toContain(`/groups/join#${TOKEN}`)
    expect(field.value).not.toContain('?')
  })

  it('says the link is reusable and cannot be cancelled', async () => {
    await mintLink()
    // The only place that bargain is stated to the person making it.
    expect(screen.getByText(/24 hours/)).toBeInTheDocument()
    expect(screen.getByText(/cannot be cancelled/)).toBeInTheDocument()
  })
})

describe('copying', () => {
  it('copies the link and says it did', async () => {
    const button = await mintLink()
    await setupUser().click(button)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument())
    expect(copyLink).toHaveBeenCalledWith(expect.stringContaining(`#${TOKEN}`))
    expect(screen.queryByText('Could not reach the clipboard')).not.toBeInTheDocument()
  })

  // The bug this exists for: Mantine's CopyButton drops useClipboard's error,
  // so outside a secure context the button stayed on "Copy link" and nothing
  // happened at all. Whatever else it does, it must never fail silently.
  it('says so, and selects the link, when it cannot copy', async () => {
    copyLink.mockResolvedValue(false)
    const button = await mintLink()
    await setupUser().click(button)

    await waitFor(() =>
      expect(screen.getByText('Could not reach the clipboard')).toBeInTheDocument(),
    )
    expect(screen.getByText(/press Ctrl\+C/i)).toBeInTheDocument()
    // Still "Copy link": it did not copy, and must not claim it did.
    expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()

    // The link is selected, so there is something the keyboard can do.
    const field = screen.getByLabelText('Invitation link') as HTMLInputElement
    expect(field.selectionStart).toBe(0)
    expect(field.selectionEnd).toBe(field.value.length)
  })
})
