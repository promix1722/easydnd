import { fireEvent, render, screen } from '@testing-library/react'
import { ItemIcon } from './ItemIcon'

it('keeps item artwork at 88px and recovers when a failed source changes', () => {
  const first = 'data:image/webp;base64,Zmlyc3Q='
  const second = 'data:image/webp;base64,c2Vjb25k'
  const view = render(<ItemIcon icon={first} />)
  const image = screen.getByAltText('')
  expect(image).toHaveAttribute('width', '88')
  expect(image).toHaveAttribute('height', '88')
  expect(image).toHaveStyle({ width: '88px', height: '88px', imageRendering: 'pixelated' })
  fireEvent.error(image)
  expect(image).toHaveStyle({ visibility: 'hidden' })
  view.rerender(<ItemIcon icon={second} />)
  expect(screen.getByAltText('')).toHaveAttribute('src', second)
  expect(screen.getByAltText('')).not.toHaveStyle({ visibility: 'hidden' })
  view.rerender(<ItemIcon icon={undefined} />)
  expect(view.container.querySelector('img')).toBeNull()
})
