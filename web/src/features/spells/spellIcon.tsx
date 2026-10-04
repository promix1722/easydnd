/** Artwork is supplied by the selected pack release through the catalog. */
export function SpellIcon({ icon, size }: { icon: string | undefined; size: number }) {
  if (icon === undefined) return null
  return (
    <img
      key={icon}
      src={icon}
      alt=""
      loading="lazy"
      decoding="async"
      width={size}
      height={size}
      onError={(event) => {
        event.currentTarget.style.visibility = 'hidden'
      }}
      style={{ borderRadius: 4, flexShrink: 0, display: 'block' }}
    />
  )
}
