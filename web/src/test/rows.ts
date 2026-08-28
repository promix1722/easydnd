import { screen, within } from '@testing-library/react'

import { setupUser } from './user'
import type { Viewport } from './viewport'

/**
 * How a row's actions are reached, which is one of two things depending on the
 * width.
 *
 * `ui/DataList` spells a row's actions out as buttons on a desktop -- where a
 * table has the room, and where `docs/web.md` argues they should be visible
 * without opening anything -- and folds them behind one `Actions for <name>`
 * control on a phone. A test that presses one therefore has to know which it is
 * looking at, and every screen with a list needs the same two lines to find out.
 *
 * They live here rather than in each test file because there are six of those
 * and they had already drifted once: three spelled out `aria-label={`Delete
 * ${name}`}` at the call site and three did not, so half the suite pressed
 * "Delete" and half pressed "Delete Ada".
 */

/** The accessible name a row action carries on a desktop: "Take off Ada". */
export function rowActionName(label: string, row: string): string {
  return `${label} ${row}`
}

/**
 * Presses one row's action at either width.
 *
 * On a phone this opens the row's menu first, which is a real extra press and
 * not a detail the test should have to remember.
 */
export async function pressRowAction(
  viewport: Viewport,
  row: string,
  label: string,
): Promise<void> {
  const user = setupUser()
  // A desktop row draws its actions as buttons unless the list asked for the
  // menu, which is the same control the phone draws -- so fall through.
  const button = screen.queryByRole('button', { name: rowActionName(label, row) })
  if (viewport === 'desktop' && button !== null) {
    await user.click(button)
    return
  }
  await user.click(screen.getByRole('button', { name: `Actions for ${row}` }))
  const menu = await screen.findByRole('menu')
  await user.click(within(menu).getByRole('menuitem', { name: label }))
}

/**
 * How many rows offer a given action -- the question "who may take a character
 * off this table?" is asked as a count in several places.
 *
 * On a phone an action is inside a closed menu and cannot be counted directly,
 * so what is counted is the rows that have a menu at all. That is the same
 * number wherever a row's actions are all-or-nothing, which is every screen
 * that asks: a player may take back their own character and do nothing to
 * anybody else's, so a row either offers everything or offers no control.
 */
export function rowsOffering(viewport: Viewport, label: string): number {
  if (viewport === 'desktop') {
    return screen.queryAllByRole('button', { name: new RegExp(`^${label} `) }).length
  }
  return screen.queryAllByRole('button', { name: /^Actions for / }).length
}

/**
 * What one row offers, as a list of labels, at either width.
 *
 * The two renderings are genuinely different DOM -- three buttons named "Make
 * player Devon", or one control opening a menu of three items named "Make
 * player" -- and a test about *which* actions a rank may take should not have
 * to care which it is looking at. What it asserts either way is the set of
 * things offered, which is the rule the server enforces and the thing worth
 * pinning.
 *
 * Returns them in the order they are drawn, which is also the order the caller
 * declared them.
 */
export async function rowActionLabels(viewport: Viewport, row: string): Promise<string[]> {
  if (viewport === 'desktop') {
    const suffix = ` ${row}`
    return screen
      .queryAllByRole('button')
      .map((button) => button.getAttribute('aria-label') ?? '')
      .filter((name) => name.endsWith(suffix))
      .map((name) => name.slice(0, -suffix.length))
  }

  const trigger = screen.queryByRole('button', { name: `Actions for ${row}` })
  if (trigger === null) return []
  const user = setupUser()
  await user.click(trigger)
  const menu = await screen.findByRole('menu')
  return within(menu)
    .queryAllByRole('menuitem')
    .map((item) => item.textContent ?? '')
}
