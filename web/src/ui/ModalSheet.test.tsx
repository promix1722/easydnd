import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

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
    expect((content as HTMLElement).style.maxHeight).toBe('85dvh')
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
