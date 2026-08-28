/**
 * Whether this browser can be offered a home-screen install, and how.
 *
 * Three answers, and they are a string rather than an object on purpose:
 * useSyncExternalStore compares snapshots by identity, so a getSnapshot that
 * built a fresh object each call would re-render for ever.
 *
 *   'none'    already installed, or nothing to offer
 *   'prompt'  Chrome fired beforeinstallprompt and we kept the event
 *   'ios'     an iOS device, where there is no event and never will be
 *
 * Module state rather than a context, like @/lib/version: the answer belongs to
 * the tab rather than to any part of the tree, and it arrives from a window
 * event nobody asked for. Same obligation follows -- resetInstallOffer() has to
 * be in the afterEach in src/test/setup.ts, because the suite shares one module
 * registry.
 *
 * The listeners below are registered when this module is first imported, which
 * is the one structural difference from @/lib/version. beforeinstallprompt
 * fires early and is not replayed: a listener attached after it fired has
 * missed it, and the offer would be lost for the life of the page.
 */

export type InstallOffer = 'none' | 'prompt' | 'ios'

/**
 * The event Chrome fires when it is willing to install. Not in lib.dom, because
 * it is Chromium's rather than a standard, so the two members we use are
 * declared here rather than asserted away at the call site.
 */
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

let deferred: BeforeInstallPromptEvent | null = null
let installed = false
const listeners = new Set<() => void>()

function announce(): void {
  for (const listener of listeners) listener()
}

/**
 * Whether the page is already running as an installed app.
 *
 * Two checks because they cover different vintages: display-mode is the
 * standard one every current browser answers, and navigator.standalone is the
 * non-standard iOS property that predates it and is still what older iOS
 * reports. Read live rather than cached -- a page can be launched into
 * standalone without this module being re-imported.
 */
function isStandalone(): boolean {
  if (typeof window === 'undefined') return false
  if ((navigator as { standalone?: boolean }).standalone === true) return true
  // Guarded because this is a getSnapshot: useSyncExternalStore calls it during
  // render, so a throw here would take the whole tree down rather than lose a
  // button. Every browser has matchMedia; a bare jsdom does not.
  return typeof window.matchMedia === 'function'
    ? window.matchMedia('(display-mode: standalone)').matches
    : false
}

/**
 * Whether this is an iOS device.
 *
 * Deliberately not a browser check. Since iOS 16.4 Chrome, Edge and Firefox all
 * install through the same Share sheet Safari does, so "which browser" stopped
 * being a question worth asking; "is this iOS" is the whole of it.
 *
 * The second half is iPadOS 13 and later, which reports itself as a Macintosh.
 * A Mac with a touch screen would be a false positive; there is no such thing.
 */
function isIos(): boolean {
  if (typeof navigator === 'undefined') return false
  const ua = navigator.userAgent
  return /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1)
}

/** What, if anything, to offer. Shaped for useSyncExternalStore. */
export function getInstallOffer(): InstallOffer {
  if (installed || isStandalone()) return 'none'
  if (deferred) return 'prompt'
  return isIos() ? 'ios' : 'none'
}

/** Subscribes to the answer changing. Shaped for useSyncExternalStore. */
export function subscribeToInstall(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/**
 * Opens the browser's own install dialog.
 *
 * Must be called from a user gesture -- Chrome rejects a prompt() that cannot
 * be traced to a click -- which is why the button calls this directly rather
 * than an effect reacting to state.
 *
 * The event is single use. Whatever the person chooses, it cannot be prompted
 * with again, so it is dropped either way: accepting brings `appinstalled`
 * along behind it, and declining should leave the button alone rather than
 * offering a dialog that would refuse to open.
 */
export async function install(): Promise<void> {
  const event = deferred
  if (!event) return
  deferred = null
  announce()
  await event.prompt()
}

/** Clears the offer. For tests; see src/test/setup.ts. */
export function resetInstallOffer(): void {
  deferred = null
  installed = false
  listeners.clear()
}

if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (event) => {
    // Without this Chrome shows its own promotion as well as ours, which is two
    // offers of the same thing in one viewport.
    event.preventDefault()
    deferred = event as BeforeInstallPromptEvent
    announce()
  })

  // Fires however the install happened -- our button, or Chrome's omnibox icon,
  // which is still there and still works. Latched rather than re-derived,
  // because the tab that did the installing is not itself standalone and would
  // otherwise go on offering.
  window.addEventListener('appinstalled', () => {
    deferred = null
    installed = true
    announce()
  })
}
