import { afterEach, describe, expect, it, vi } from 'vitest'

import { copyText } from './clipboard'

/**
 * Replaces navigator.clipboard for one test.
 *
 * Assigned through defineProperty rather than vi.stubGlobal('navigator', ...):
 * the property is a getter on the prototype, so spreading navigator drops it
 * and half the rest of the object with it.
 */
function withClipboard(clipboard: Clipboard | undefined) {
  Object.defineProperty(navigator, 'clipboard', {
    value: clipboard,
    configurable: true,
    writable: true,
  })
}

afterEach(() => {
  withClipboard(undefined)
  vi.restoreAllMocks()
})

describe('copyText', () => {
  it('uses the clipboard API when there is one', async () => {
    const writeText = vi.fn(async () => {})
    withClipboard({ writeText } as unknown as Clipboard)

    await expect(copyText('hello')).resolves.toBe(true)
    expect(writeText).toHaveBeenCalledWith('hello')
  })

  // The case that started this: navigator.clipboard exists only in a secure
  // context, so over plain HTTP on anything but localhost it is undefined.
  it('falls back to a selection copy when the clipboard API is absent', async () => {
    withClipboard(undefined)
    const exec = vi.fn(() => true)
    document.execCommand = exec as unknown as typeof document.execCommand

    await expect(copyText('hello')).resolves.toBe(true)
    expect(exec).toHaveBeenCalledWith('copy')
  })

  it('falls back when the clipboard API rejects', async () => {
    withClipboard({
      writeText: vi.fn(async () => {
        throw new Error('denied')
      }),
    } as unknown as Clipboard)
    const exec = vi.fn(() => true)
    document.execCommand = exec as unknown as typeof document.execCommand

    await expect(copyText('hello')).resolves.toBe(true)
    expect(exec).toHaveBeenCalledWith('copy')
  })

  // Reporting the failure is the whole point of the boolean: the bug this
  // replaced was a button that could not copy and never said so.
  it('reports failure rather than pretending', async () => {
    withClipboard(undefined)
    document.execCommand = (() => false) as unknown as typeof document.execCommand

    await expect(copyText('hello')).resolves.toBe(false)
  })

  it('leaves no scratch element behind either way', async () => {
    withClipboard(undefined)
    document.execCommand = (() => true) as unknown as typeof document.execCommand

    await copyText('hello')
    expect(document.querySelectorAll('textarea')).toHaveLength(0)
  })
})
