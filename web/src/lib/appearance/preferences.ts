import { PALETTES, type PaletteName } from '@/theme/palettes'

export interface Appearance {
  palette: PaletteName
  color_scheme: 'light' | 'dark' | 'auto'
}

export const DEFAULT_APPEARANCE: Appearance = { palette: 'dragon', color_scheme: 'auto' }
export const BROWSER_APPEARANCE_KEY = 'easydnd.appearance.browser'
export const LAST_ACCOUNT_KEY = 'easydnd.appearance.account'
export function accountAppearanceKey(id: string): string { return `easydnd.appearance.account.${id}` }

export function isAppearance(value: unknown): value is Appearance {
  if (!value || typeof value !== 'object') return false
  const a = value as Partial<Appearance>
  return typeof a.palette === 'string' && Object.hasOwn(PALETTES, a.palette)
    && (a.color_scheme === 'light' || a.color_scheme === 'dark' || a.color_scheme === 'auto')
}

export function readAppearance(key: string): Appearance {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(key) ?? 'null')
    return isAppearance(value) ? value : DEFAULT_APPEARANCE
  } catch { return DEFAULT_APPEARANCE }
}

export function writeAppearance(key: string, appearance: Appearance): void {
  try { localStorage.setItem(key, JSON.stringify(appearance)) } catch { /* Storage may be unavailable. */ }
}

export function startupAppearance(): Appearance {
  try {
    const id = localStorage.getItem(LAST_ACCOUNT_KEY)
    return readAppearance(id ? accountAppearanceKey(id) : BROWSER_APPEARANCE_KEY)
  } catch { return DEFAULT_APPEARANCE }
}
