/** Only a cookie selector is kept in this tab; credentials stay HttpOnly. */
const key = 'easydnd.devSession'

export function developmentSession(): string | undefined {
  if (!import.meta.env.DEV) return undefined
  try {
    return window.sessionStorage.getItem(key) || undefined
  } catch {
    return undefined
  }
}

export function newDevelopmentSession(): string {
  // getRandomValues also works on the plain HTTP development proxy.
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (n) => n.toString(16).padStart(2, '0')).join('')
}

export function saveDevelopmentSession(scope: string): void {
  window.sessionStorage.setItem(key, scope)
}

export function clearDevelopmentSession(): void {
  if (import.meta.env.DEV) {
    try { window.sessionStorage.removeItem(key) } catch { /* Storage may be disabled. */ }
  }
}
