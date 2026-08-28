import { reloadOntoDeployedRelease, useReleaseWatch } from '@/lib/version'

import { UpdateRequired } from './UpdateRequired'

/**
 * Watches for a deploy and puts the update dialog over everything when it
 * finds one.
 *
 * The two halves are separate files on purpose. This one is all wiring and
 * nothing to look at; `UpdateRequired` is all appearance and takes both its
 * state and its action as props, which is what makes it testable without
 * mocking a module -- the suite shares one module registry, so `vi.mock` is
 * banned repo-wide.
 *
 * Mounted in main.tsx rather than in a shell. The shells are chosen by
 * RootGate, which branches rather than wrapping, and /legal is outside it
 * entirely -- so anywhere lower would leave some of the app without the dialog.
 *
 * There is no exemption for any route, and one consequence is worth knowing:
 * if nginx ever serves a stale index.html, reloading does not fix it and this
 * dialog has no way out. /status used to be the page you could still reach to
 * see the two versions disagreeing; it was removed. Diagnose that case with
 * `curl https://easydnd.org/version.json` against `/v1/version` instead.
 */
export function UpdateGate() {
  return <UpdateRequired opened={useReleaseWatch()} onReload={reloadOntoDeployedRelease} />
}
