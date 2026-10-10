import { createContext, useContext } from 'react'
import { DEFAULT_APPEARANCE, type Appearance } from './preferences'

interface AppearanceState {
  appearance: Appearance
  loading: boolean
  loadFailed: boolean
  reload: () => void
  saving: boolean
  failed: boolean
  change: (next: Appearance) => Promise<void>
}
export const AppearanceContext = createContext<AppearanceState | null>(null)

// AppTheme also renders in tests without an authenticated application around it.
export function useAppearance(): AppearanceState {
  return useContext(AppearanceContext) ?? {
    appearance: DEFAULT_APPEARANCE, loading: false, loadFailed: false, reload: () => {}, saving: false, failed: false, change: async () => {},
  }
}

