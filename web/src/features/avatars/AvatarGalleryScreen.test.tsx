import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MemoryRouter } from 'react-router'

import { renderAt } from '@/test/render'
import { AvatarGalleryScreen } from './AvatarGalleryScreen'

describe.each(['mobile', 'desktop'] as const)('avatar gallery (%s)', (viewport) => {
  it('shows all 12 class emblems and 24 random icons without sign-in', () => {
    renderAt(viewport, <MemoryRouter initialEntries={['/avatar-gallery']}><AvatarGalleryScreen /></MemoryRouter>)
    expect(screen.getByRole('heading', { name: 'Avatars' })).toBeInTheDocument()
    const portraits = screen.getAllByAltText('')
    expect(portraits).toHaveLength(36)
    expect(new Set(portraits.map((image) => image.getAttribute('src'))).size).toBe(36)
    expect(screen.getByRole('heading', { name: 'Users and NPCs' })).toBeInTheDocument()
    expect(screen.getByText('Wolf')).toBeInTheDocument()
    expect(screen.getByText('Moon')).toBeInTheDocument()
    expect(screen.getByText('Rogue')).toBeInTheDocument()
    expect(screen.queryByText('Adventurer')).not.toBeInTheDocument()
  })
})
