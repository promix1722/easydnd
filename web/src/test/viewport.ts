import { DESKTOP_FROM } from '@/theme/tokens'

/**
 * jsdom has no layout engine and therefore no window.matchMedia, so every
 * component that switches on viewport would otherwise be untestable -- which
 * is how a desktop-only regression reaches a phone unnoticed.
 *
 * This installs a matchMedia that answers `(min-width: …)` queries against a
 * chosen width, letting a test render the same tree at both sizes.
 */

const listeners = new Set<() => void>()
let currentWidth: number | null = null

function emParam(query: string): number | null {
  const match = /\(min-width:\s*([\d.]+)em\)/.exec(query)
  return match?.[1] ? Number(match[1]) : null
}

/** Root font size jsdom reports; Mantine's em breakpoints resolve against it. */
const ROOT_FONT_PX = 16

export type Viewport = 'mobile' | 'desktop'

/** Width in px just either side of the one breakpoint that matters. */
export const VIEWPORT_WIDTHS: Record<Viewport, number> = {
  mobile: 390,
  desktop: 1440,
}

export function setViewport(viewport: Viewport): void {
  currentWidth = VIEWPORT_WIDTHS[viewport]

  window.matchMedia = ((query: string): MediaQueryList => {
    const minEm = emParam(query)
    const matches = minEm === null ? false : (currentWidth ?? 0) >= minEm * ROOT_FONT_PX

    const list: MediaQueryList = {
      media: query,
      matches,
      onchange: null,
      addEventListener: (_type: string, listener: EventListener) => {
        listeners.add(listener as unknown as () => void)
      },
      removeEventListener: (_type: string, listener: EventListener) => {
        listeners.delete(listener as unknown as () => void)
      },
      addListener: (listener: () => void) => listeners.add(listener),
      removeListener: (listener: () => void) => listeners.delete(listener),
      dispatchEvent: () => true,
    }
    return list
  }) as typeof window.matchMedia
}

export function resetViewport(): void {
  listeners.clear()
  currentWidth = null
  Reflect.deleteProperty(window, 'matchMedia')
}

/**
 * Guards the assumption the widths above encode: that both test viewports sit
 * on the intended side of the single breakpoint declared in the tokens.
 */
export const DESKTOP_BREAKPOINT_PX = Number(DESKTOP_FROM.replace('em', '')) * ROOT_FONT_PX
