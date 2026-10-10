import { fireEvent, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'
import { AvatarEditor } from './AvatarEditor'
import { cropPortrait, validatePortrait } from './portrait'

const PORTRAIT = 'data:image/webp;base64,cG9ydHJhaXQ='

function browserImage(width = 600, height = 300) {
  const decode = vi.fn(async () => {})
  vi.stubGlobal('Image', class {
    src = ''
    naturalWidth = width
    naturalHeight = height
    decode = decode
  })
  const revoke = vi.fn()
  vi.stubGlobal('URL', class extends URL {
    static override createObjectURL = vi.fn(() => 'blob:portrait')
    static override revokeObjectURL = revoke
  })
  const draw = vi.fn()
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ drawImage: draw } as unknown as CanvasRenderingContext2D)
  const encode = vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue(PORTRAIT)
  return { decode, draw, encode, revoke }
}

afterEach(() => vi.restoreAllMocks())

function Editor({ save, initial = '', pending = false }: { save: (image: string) => void; initial?: string; pending?: boolean }) {
  const [image, setImage] = useState(initial)
  return <AvatarEditor fallback="/avatars/rogue.webp" image={image} pending={pending} onSave={(next) => { setImage(next); save(next) }} />
}

describe.each(['mobile', 'desktop'] as const)('shared avatar editor (%s)', (viewport) => {
  it('opens a crop editor, saves adjusted crops, replaces and removes avatars', async () => {
    const user = setupUser()
    const { draw, encode, revoke } = browserImage()
    const save = vi.fn()
    const { container } = renderAt(viewport, <Editor save={save} />)
    const input = container.querySelector<HTMLInputElement>('input[type=file]')!
    await user.upload(input, new File(['image'], 'hero.png', { type: 'image/png' }))
    const confirm = await screen.findByRole('button', { name: 'Use image' })
    await waitFor(() => expect(confirm).not.toBeDisabled())
    expect(save).not.toHaveBeenCalled()
    fireEvent.change(screen.getByLabelText('Zoom'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('Horizontal position'), { target: { value: '100' } })
    fireEvent.change(screen.getByLabelText('Vertical position'), { target: { value: '0' } })
    expect(screen.getByAltText('Image crop preview')).toHaveStyle({ left: '-768px', top: '0px' })
    await user.click(confirm)
    expect(draw).toHaveBeenCalledWith(expect.anything(), 450, 0, 150, 150, 0, 0, 256, 256)
    expect(encode).toHaveBeenCalledWith('image/webp', 0.85)
    expect(revoke).toHaveBeenCalledWith('blob:portrait')
    expect(save).toHaveBeenLastCalledWith(PORTRAIT)
    expect(screen.getByAltText('')).toHaveAttribute('src', PORTRAIT)

    const replacement = 'data:image/webp;base64,bmV3'
    encode.mockReturnValue(replacement)
    await user.upload(input, new File(['replacement'], 'hero.png', { type: 'image/png' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Use image' })).not.toBeDisabled())
    await user.click(screen.getByRole('button', { name: 'Use image' }))
    expect(save).toHaveBeenLastCalledWith(replacement)
    await user.click(screen.getByRole('button', { name: 'Remove image' }))
    expect(save).toHaveBeenLastCalledWith('')
    expect(screen.getByAltText('')).toHaveAttribute('src', '/avatars/rogue.webp')
  })

  it('cancels without replacing the saved avatar', async () => {
    const user = setupUser()
    const { revoke } = browserImage()
    const save = vi.fn()
    const { container } = renderAt(viewport, <Editor save={save} initial={PORTRAIT} />)
    await user.upload(container.querySelector<HTMLInputElement>('input[type=file]')!, new File(['image'], 'hero.png', { type: 'image/png' }))
    await user.click(await screen.findByRole('button', { name: 'Cancel' }))
    expect(save).not.toHaveBeenCalled()
    expect(screen.getByAltText('')).toHaveAttribute('src', PORTRAIT)
    expect(revoke).toHaveBeenCalled()
  })

  it('rejects invalid images without changing the current avatar', async () => {
    const user = setupUser()
    const { decode } = browserImage()
    decode.mockRejectedValue(new Error('corrupt image'))
    const save = vi.fn()
    const { container } = renderAt(viewport, <Editor save={save} initial={PORTRAIT} />)
    await user.upload(container.querySelector<HTMLInputElement>('input[type=file]')!, new File(['corrupt'], 'bad.png', { type: 'image/png' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Choose a valid JPEG, PNG or WebP')
    expect(screen.getByRole('button', { name: 'Use image' })).toBeDisabled()
    expect(save).not.toHaveBeenCalled()
    expect(screen.getByAltText('')).toHaveAttribute('src', PORTRAIT)
  })

  it('disables upload and removal while saving', () => {
    renderAt(viewport, <Editor save={vi.fn()} initial={PORTRAIT} pending />)
    expect(screen.getByRole('button', { name: 'Upload image' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove image' })).toBeDisabled()
  })
})

it('center-crops tall images and bounds file and encoded sizes', () => {
  const { draw, encode } = browserImage(300, 600)
  const image = new Image()
  expect(cropPortrait(image, 50, 50, 1)).toBe(PORTRAIT)
  expect(draw).toHaveBeenCalledWith(expect.anything(), 0, 150, 300, 300, 0, 0, 256, 256)
  expect(() => validatePortrait(new File(['svg'], 'bad.svg', { type: 'image/svg+xml' }))).toThrow()
  expect(() => validatePortrait(new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'huge.png', { type: 'image/png' }))).toThrow()
  encode.mockReturnValue('data:image/webp;base64,' + 'A'.repeat(256 * 1024))
  expect(() => cropPortrait(image, 50, 50, 1)).toThrow()
})

it('repositions the crop by dragging and clamps at the image edge', async () => {
  const user = setupUser()
  browserImage()
  vi.stubGlobal('PointerEvent', MouseEvent)
  const { container } = renderAt('desktop', <Editor save={vi.fn()} />)
  await user.upload(container.querySelector<HTMLInputElement>('input[type=file]')!, new File(['image'], 'hero.png', { type: 'image/png' }))
  const preview = await screen.findByAltText('Image crop preview')
  const area = preview.parentElement!
  area.setPointerCapture = vi.fn()
  fireEvent.pointerDown(area, { clientX: 128, clientY: 128 })
  fireEvent.pointerMove(area, { clientX: 0, clientY: 128 })
  expect(screen.getByLabelText('Horizontal position')).toHaveValue('100')
  fireEvent.pointerUp(area)
  fireEvent.pointerMove(area, { clientX: 256, clientY: 128 })
  expect(screen.getByLabelText('Horizontal position')).toHaveValue('100')
})

it('uses the same 48px avatar with a black border for saved images and placeholders', () => {
  const { container } = renderAt('desktop', <Editor save={vi.fn()} initial={PORTRAIT} />)
  const avatar = container.querySelector('.mantine-Avatar-root')!
  expect((avatar as HTMLElement).style.border).toBe('1px solid black')
  expect(avatar).toHaveStyle({ background: '#252c3b', '--avatar-radius': 'calc(0.5rem * var(--mantine-scale))' })
  expect(avatar.getAttribute('style')).toContain('--avatar-size: calc(3rem')
})
