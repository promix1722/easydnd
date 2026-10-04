import { fireEvent, render, screen } from '@testing-library/react'
import { SpellIcon } from './spellIcon'

it('displays the selected pack artwork and resets a failed image when it changes', () => {
  const first = 'data:image/webp;base64,Zmlyc3Q='
  const second = 'data:image/webp;base64,c2Vjb25k'
  const view = render(<SpellIcon icon={first} size={32} />)
  const image = screen.getByAltText('')
  expect(image).toHaveAttribute('src', first)
  expect(image).toHaveAttribute('width', '32')
  fireEvent.error(image)
  expect(image).toHaveStyle({ visibility: 'hidden' })
  view.rerender(<SpellIcon icon={second} size={32} />)
  expect(screen.getByAltText('')).toHaveAttribute('src', second)
  expect(screen.getByAltText('')).not.toHaveStyle({ visibility: 'hidden' })
})

it('renders no image when a pack has no icon', () => {
  const { container } = render(<SpellIcon icon={undefined} size={40} />)
  expect(container.querySelector('img')).toBeNull()
})
