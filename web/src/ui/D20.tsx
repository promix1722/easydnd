import { VisuallyHidden } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { Suspense, lazy, useEffect, useRef, useState } from 'react'

import { d20 } from '@/domain'
import { useT } from '@/lib/i18n'

/**
 * The die, and everything about it that is cheap.
 *
 * `./D20Scene.tsx` -- three.js, cannon-es, a WebGL context and about 180 kB
 * gzipped of it -- is behind the `lazy` below and is **never** in the main
 * bundle. This file is the part that always ships: a placeholder, the result,
 * and the decision about when the heavy half is worth fetching.
 *
 * ## When it loads, and why not sooner
 *
 * On an intersection, not on mount. The die's two homes are both effectively
 * the front door -- the landing page is every visitor's first screen and the
 * menu item is on every signed-in one -- so loading with the component would
 * put the whole of three.js on the critical path of the page a stranger
 * arrives at. It is the fourth panel of a carousel: embla mounts all four, but
 * only the one you have swiped to is on screen, so an `IntersectionObserver`
 * answers exactly the right question. Swipe to the die and it fetches; never
 * swipe and it never does.
 *
 * The service worker had to be told the same thing separately. `vite.config.ts`
 * precaches `**\/*.js`, which would have downloaded the chunk in the background
 * on first visit and made all of the above ceremony -- see the `globIgnores`
 * entry there, which names this chunk and is the other half of this decision.
 */
const D20Scene = lazy(() => import('./D20Scene'))

export interface D20RollProps {
  /**
   * The die, injected.
   *
   * Note what this does and does not decide. A *thrown* die takes its number
   * from the physics -- see `D20Scene.tsx` -- so this is consulted only where
   * there is no throw: the reduced-motion path, and a throw that fails to
   * settle. It is a parameter for the same reason `rollAbilityScores` takes
   * one, and matters more here because this suite bans `vi.mock` outright.
   */
  roll?: () => number
}

/**
 * The die fills whatever it is given.
 *
 * It takes no size. Both of its homes are a box someone else decided the shape
 * of -- a carousel slide on the landing page, a full-screen sheet from the
 * menu -- and a die with a size of its own floated in the middle of the first
 * one, too small and off-centre, while the panel around it stayed empty. So
 * the element measures its container and the scene builds a camera and a
 * physics box to match, which is also what puts the walls exactly at the edges
 * of the picture.
 */
export function D20Roll({ roll = d20 }: D20RollProps) {
  const t = useT()
  const frame = useRef<HTMLDivElement>(null)
  // Initialised rather than set from an effect, so that the no-observer case
  // costs no extra render: a browser without IntersectionObserver gets the die
  // from the first paint. Failing *open* is the right way round for a
  // progressive enhancement -- the alternative is every such visitor getting
  // no die at all.
  const [near, setNear] = useState(() => typeof IntersectionObserver !== 'function')
  const [value, setValue] = useState<number | null>(null)
  const [box, setBox] = useState<{ width: number; height: number } | null>(null)

  const still =
    useMediaQuery('(prefers-reduced-motion: reduce)', false, { getInitialValueInEffect: false }) ??
    false

  useEffect(() => {
    const host = frame.current
    if (!host || near) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) setNear(true)
      },
      { threshold: 0.25 },
    )
    observer.observe(host)
    return () => observer.disconnect()
  }, [near])

  useEffect(() => {
    const host = frame.current
    if (!host || typeof ResizeObserver !== 'function') return

    const observer = new ResizeObserver(([entry]) => {
      const rect = entry?.contentRect
      if (!rect || rect.width < 1 || rect.height < 1) return
      // Rounded, because a fractional resize would tear the WebGL context down
      // and rebuild it for a change nobody can see.
      setBox((was) => {
        const next = { width: Math.round(rect.width), height: Math.round(rect.height) }
        return was && was.width === next.width && was.height === next.height ? was : next
      })
    })
    observer.observe(host)
    return () => observer.disconnect()
  }, [])

  return (
    <div ref={frame} style={{ width: '100%', height: '100%', overflow: 'hidden' }}>
      {near && box ? (
        <Suspense fallback={null}>
          <D20Scene
            width={box.width}
            height={box.height}
            still={still}
            roll={roll}
            label={t('dice.roll')}
            onThrown={() => {
              setValue(null)
              navigator.vibrate?.(10)
            }}
            onSettled={(landed) => {
              setValue(landed)
              // Android only: iOS Safari implements no Vibration API at all,
              // so this is a silent no-op there rather than a gap worth
              // writing a fallback for.
              navigator.vibrate?.(landed === 20 ? [18, 40, 34] : 22)
            }}
          />
        </Suspense>
      ) : null}

      {/*
       * The result, for people who cannot see the die.
       *
       * There is no visible caption and no printed number anywhere here, by
       * design: the camera looks straight down, so the number that landed is
       * simply the one facing you, exactly as it is on a table. That works
       * only if you can see it -- and the number is painted into a WebGL
       * canvas, which is opaque to assistive technology by construction. So
       * this is not a duplicate of something on screen, it is the only channel
       * carrying the result at all.
       */}
      <VisuallyHidden aria-live="polite" aria-atomic>
        {value === null ? '' : t('dice.result', { value })}
      </VisuallyHidden>
    </div>
  )
}
