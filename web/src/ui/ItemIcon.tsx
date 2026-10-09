/** Decorative pack artwork: the adjacent item name remains the accessible label. */
export function ItemIcon({ icon }: { icon: string | undefined }) {
  if (!icon) return null
  return <img
    key={icon}
    src={icon}
    alt=""
    loading="lazy"
    decoding="async"
    width={88}
    height={88}
    onError={(event) => { event.currentTarget.style.visibility = 'hidden' }}
    style={{
      display: 'block', flexShrink: 0, objectFit: 'contain', imageRendering: 'pixelated',
      width: 88, height: 88, padding: 3, boxSizing: 'border-box',
      background: '#252c3b', borderRadius: 8,
    }}
  />
}
