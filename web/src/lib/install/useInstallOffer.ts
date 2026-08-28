import { useSyncExternalStore } from 'react'

import { getInstallOffer, subscribeToInstall, type InstallOffer } from './state'

/**
 * What to offer this browser, if anything. Re-renders when the answer changes
 * -- which it does when Chrome decides the app is installable, and again when
 * it has been installed.
 */
export function useInstallOffer(): InstallOffer {
  return useSyncExternalStore(subscribeToInstall, getInstallOffer, getInstallOffer)
}
