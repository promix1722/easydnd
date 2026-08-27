import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { ModalSheet } from './ModalSheet'

const noop = () => {}

describe('ModalSheet', () => {
  it('renders its content and title at both viewports', () => {
    renderAt(
      'desktop',
      <ModalSheet opened onClose={noop} title="Choose a race">
        <p>Dwarf</p>
      </ModalSheet>,
    )
    expect(screen.getByText('Choose a race')).toBeInTheDocument()
    expect(screen.getByText('Dwarf')).toBeInTheDocument()
  })

  it('renders as a drawer on mobile and a modal on desktop', () => {
    const { unmount } = renderAt(
      'mobile',
      <ModalSheet opened onClose={noop} title="Choose a race">
        <p>Dwarf</p>
      </ModalSheet>,
    )
    // Mantine tags its two overlay families with distinct root classes; that
    // is the only observable difference in jsdom, which has no layout.
    expect(document.querySelector('.mantine-Drawer-content')).not.toBeNull()
    expect(document.querySelector('.mantine-Modal-content')).toBeNull()
    unmount()

    renderAt(
      'desktop',
      <ModalSheet opened onClose={noop} title="Choose a race">
        <p>Dwarf</p>
      </ModalSheet>,
    )
    expect(document.querySelector('.mantine-Modal-content')).not.toBeNull()
    expect(document.querySelector('.mantine-Drawer-content')).toBeNull()
  })

  // The regression this exists for: `size="auto"` resolved to an undefined
  // Mantine custom property, so `height` fell through to its 100% fallback and
  // every sheet on a phone opened as tall as the cap allowed -- a three-line
  // form filling the screen. jsdom computes no layout, so what is asserted is
  // the declaration itself, which is the thing that was wrong.
  it('lets the mobile sheet hug its content', () => {
    renderAt(
      'mobile',
      <ModalSheet opened onClose={noop} title="Choose a race">
        <p>Dwarf</p>
      </ModalSheet>,
    )
    const content = document.querySelector('.mantine-Drawer-content')
    expect(content).not.toBeNull()
    expect((content as HTMLElement).style.height).toBe('auto')
    // And still capped, or the sheet's own header goes behind the browser
    // chrome on a short viewport.
    expect((content as HTMLElement).style.maxHeight).toBe('85svh')
  })

  /**
   * The bug this pair exists for: on a phone, the keyboard's Go key did
   * nothing in eight of the app's eleven dialogs, because they were a field
   * beside a button rather than a form. A browser offers that key on the
   * strength of seeing a form with a submit in it, so nothing short of a real
   * `<form>` brings it back.
   */
  describe('submitting', () => {
    it.each(['mobile', 'desktop'] as const)('submits on the key, at %s', async (viewport) => {
      const onSubmit = vi.fn()
      renderAt(
        viewport,
        <ModalSheet opened onClose={noop} title="New group" onSubmit={onSubmit}>
          <input aria-label="Name" />
          <button type="submit">Create</button>
        </ModalSheet>,
      )

      // Enter in a field is what a soft keyboard's Go key sends, and it only
      // submits when the field is inside a form with a submit button.
      await setupUser().type(screen.getByLabelText('Name'), 'Wednesday{Enter}')
      expect(onSubmit).toHaveBeenCalledTimes(1)
    })

    it('wraps nothing in a form when the dialog is not one', () => {
      renderAt(
        'mobile',
        <ModalSheet opened onClose={noop} title="Delete this group">
          <p>This cannot be undone.</p>
        </ModalSheet>,
      )

      // A confirmation has no field and must not become a form: a stray submit
      // there would fire on a key press nobody aimed at anything.
      expect(document.querySelector('form')).toBeNull()
    })
  })

  it('renders nothing while closed', () => {
    renderAt(
      'mobile',
      <ModalSheet opened={false} onClose={noop} title="Choose a race">
        <p>Dwarf</p>
      </ModalSheet>,
    )
    expect(screen.queryByText('Dwarf')).not.toBeInTheDocument()
  })
})
