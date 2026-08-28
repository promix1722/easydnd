import { install, useInstallOffer } from '@/lib/install'

import { InstallAction } from './InstallAction'

/**
 * Wires the install offer to the button that acts on it.
 *
 * Split for the same reason UpdateGate is split from UpdateRequired: this half
 * is all module state and nothing to look at, and `InstallAction` takes both
 * its state and its action as props, which is what makes it testable without
 * mocking a module. The suite shares one module registry, so `vi.mock` is
 * banned repo-wide.
 */
export function InstallButton() {
  return <InstallAction offer={useInstallOffer()} onInstall={() => void install()} />
}
