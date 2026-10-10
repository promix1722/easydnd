export function validatePortrait(file: File) {
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 5 * 1024 * 1024) {
    throw new Error('invalid portrait')
  }
}

/** Normalized positioning makes the preview and saved crop agree at every zoom. */
export function portraitCrop(image: HTMLImageElement, x: number, y: number, zoom: number) {
  const side = Math.min(image.naturalWidth, image.naturalHeight) / zoom
  return { side, left: (image.naturalWidth - side) * x / 100, top: (image.naturalHeight - side) * y / 100 }
}

export function cropPortrait(image: HTMLImageElement, x: number, y: number, zoom: number): string {
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = 256
  const context = canvas.getContext('2d')
  if (!context) throw new Error('canvas unavailable')
  const { side, left, top } = portraitCrop(image, x, y, zoom)
  if (!side) throw new Error('empty portrait')
  context.drawImage(image, left, top, side, side, 0, 0, 256, 256)
  const result = canvas.toDataURL('image/webp', 0.85)
  if (!result.startsWith('data:image/webp;base64,') || result.length > 256 * 1024) throw new Error('invalid portrait')
  return result
}
