import { fireEvent, render, screen } from '@testing-library/react'
import { ItemIcon } from './ItemIcon'

it('keeps item artwork at 66px and recovers when a failed source changes', () => {
  const first = 'data:image/webp;base64,Zmlyc3Q='
  const second = 'data:image/webp;base64,c2Vjb25k'
  const view = render(<ItemIcon icon={first} />)
  const image = screen.getByAltText('')
  expect(image).toHaveAttribute('width', '66')
  expect(image).toHaveAttribute('height', '66')
  expect(image).toHaveStyle({ width: '66px', height: '66px', imageRendering: 'pixelated' })
  fireEvent.error(image)
  expect(image).toHaveStyle({ visibility: 'hidden' })
  view.rerender(<ItemIcon icon={second} />)
  expect(screen.getByAltText('')).toHaveAttribute('src', second)
  expect(screen.getByAltText('')).not.toHaveStyle({ visibility: 'hidden' })
  view.rerender(<ItemIcon icon={undefined} />)
  expect(view.container.querySelector('img')).toBeNull()
})
