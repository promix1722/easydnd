/**
 * Putting text on the clipboard, in the two ways a browser offers.
 *
 * `navigator.clipboard` exists only in a **secure context**. Served over plain
 * HTTP on anything but localhost -- which is exactly how this app is reached
 * on a dev box behind a proxy -- the whole object is `undefined`, the same way
 * `PublicKeyCredential` is. Mantine's `useClipboard` reports that by setting
 * an `error` its `CopyButton` then drops on the floor, so the button says
 * "Copy" forever and nothing happens: the worst possible outcome for the one
 * control an invite link is delivered through.
 *
 * So: try the modern API, fall back to the deprecated one that predates the
 * secure-context rule, and -- above all -- **return whether it worked**, so a
 * caller can say so instead of pretending.
 */
export async function copyText(value: string): Promise<boolean> {
  // Typed as always present, actually absent outside a secure context.
  const api = navigator.clipboard as Clipboard | undefined
  if (api !== undefined) {
    try {
      await api.writeText(value)
      return true
    } catch {
      // Permission refused, or a browser that has the object but not the
      // permission. The fallback below sometimes still works.
    }
  }
  return copyBySelection(value)
}

/**
 * The pre-2018 path: select text in a throwaway element and ask the document
 * to copy the selection.
 *
 * `execCommand` is deprecated and still the only thing that works without a
 * secure context. The element is positioned rather than hidden because a
 * `display: none` element cannot hold a selection, and `readonly` keeps a
 * mobile keyboard from opening over the dialog on the way past.
 */
function copyBySelection(value: string): boolean {
  const area = document.createElement('textarea')
  area.value = value
  area.setAttribute('readonly', '')
  area.style.position = 'fixed'
  area.style.top = '0'
  area.style.opacity = '0'
  document.body.appendChild(area)
  try {
    area.select()
    area.setSelectionRange(0, value.length)
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    area.remove()
  }
}
