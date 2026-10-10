/** Pack artwork uses one consistent 44px slate tile in every spell view. */
export function SpellIcon({ icon }: { icon: string | undefined }) {
  if (icon === undefined) return null
  return (
    <span style={{
      background: '#252c3b',
      borderRadius: 8,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 44,
      height: 44,
      padding: 3,
      boxSizing: 'border-box',
      flexShrink: 0,
      overflow: 'hidden',
    }}>
      <img
        key={icon}
        src={icon}
        alt=""
        loading="lazy"
        decoding="async"
        width={44}
        height={44}
        onError={(event) => {
          event.currentTarget.style.visibility = 'hidden'
        }}
        style={{ display: 'block', width: '100%', height: '100%', objectFit: 'contain' }}
      />
    </span>
  )
}
