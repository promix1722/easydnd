import userEvent from '@testing-library/user-event'

/**
 * A user-event session for one test.
 *
 * `delay: null` is the whole reason this exists. By default user-event waits a
 * macrotask between every event it dispatches, and a single `click` is six or
 * more of them -- pointerdown, mousedown, focus, pointerup, mouseup, click --
 * while `type` is one per keystroke. Across this suite that was the largest
 * remaining cost by a wide margin: BuildScreen.test.tsx alone drives 61
 * interactions across two viewports.
 *
 * What the delay is for is letting anything scheduled between two events run
 * before the next one arrives. Nothing here depends on that: the components
 * under test respond to events, not to the gaps between them, and the one
 * place a timer matters -- InviteSheet's "Copied" reverting after two seconds
 * -- is asserted through waitFor either way.
 *
 * Use this rather than calling `userEvent.setup()` directly, so the reasoning
 * lives in one place instead of in fifty-seven.
 */
export function setupUser(): ReturnType<typeof userEvent.setup> {
  return userEvent.setup({ delay: null })
}
