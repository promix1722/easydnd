import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'

import { InviteSheet } from './InviteSheet'

// The helper is mocked rather than the browser API it wraps, for a practical
// reason: userEvent installs a navigator.clipboard stub of its own, so a test
// that poked at the real one would be measuring testing-library. What copyText
// does with a missing clipboard is pinned in lib/clipboard.test.ts instead.
const copyText = vi.hoisted(() => vi.fn(async () => true))
vi.mock('@/lib/clipboard', () => ({ copyText }))

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
  renderAt('desktop', withAuth({}, <InviteSheet groupId="grp_1" opened onClose={() => {}} />))
  await userEvent.click(screen.getByRole('button', { name: 'Create link' }))
  return await screen.findByRole('button', { name: 'Copy link' })
}

beforeEach(() => {
  copyText.mockReset()
  copyText.mockResolvedValue(true)
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
    await userEvent.click(button)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument())
    expect(copyText).toHaveBeenCalledWith(expect.stringContaining(`#${TOKEN}`))
    expect(screen.queryByText('Could not reach the clipboard')).not.toBeInTheDocument()
  })

  // The bug this exists for: Mantine's CopyButton drops useClipboard's error,
  // so outside a secure context the button stayed on "Copy link" and nothing
  // happened at all. Whatever else it does, it must never fail silently.
  it('says so, and selects the link, when it cannot copy', async () => {
    copyText.mockResolvedValue(false)
    const button = await mintLink()
    await userEvent.click(button)

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
