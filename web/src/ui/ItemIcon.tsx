/** One and a half spell icons. Equipment slots reserve this height. */
export const ITEM_ICON_SIZE = 66

/** Decorative pack artwork: the adjacent item name remains the accessible label. */
export function ItemIcon({ icon }: { icon: string | undefined }) {
  if (!icon) return null
  return <img
    key={icon}
    src={icon}
    alt=""
    loading="lazy"
    decoding="async"
    width={ITEM_ICON_SIZE}
    height={ITEM_ICON_SIZE}
    onError={(event) => { event.currentTarget.style.visibility = 'hidden' }}
    style={{
      display: 'block', flexShrink: 0, objectFit: 'contain', imageRendering: 'pixelated',
      width: ITEM_ICON_SIZE, height: ITEM_ICON_SIZE, padding: 3, boxSizing: 'border-box',
      background: '#252c3b', borderRadius: 8,
    }}
  />
}
