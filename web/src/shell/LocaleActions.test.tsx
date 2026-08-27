import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { LocaleActions } from './LocaleActions'

/**
 * The one control in this app that changes what every other control says.
 *
 * What matters here is that it is *reachable* and that both languages are
 * offered by the name each calls itself -- somebody looking for Russian is
 * looking for "Русский", not for whatever English calls it. That the choice
 * then takes effect is `lib/i18n`'s to prove; this is the door.
 */
describe('LocaleActions', () => {
  it('offers every language, named in its own language', async () => {
    const user = setupUser()
    renderAt('desktop', <LocaleActions />)

    await user.click(screen.getByRole('button', { name: 'Change language' }))

    expect(await screen.findByRole('menuitem', { name: 'English' })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: 'Русский' })).toBeInTheDocument()
  })

  // Marked rather than merely styled: which language you are in is the one
  // question this menu exists to answer, and a tick that only a sighted reader
  // can see does not answer it.
  it('marks the language currently in use', async () => {
    const user = setupUser()
    renderAt('desktop', <LocaleActions />, 'ru')

    await user.click(screen.getByRole('button', { name: 'Сменить язык' }))

    expect(await screen.findByRole('menuitem', { name: 'Русский' })).toHaveAttribute(
      'aria-current',
      'true',
    )
    expect(screen.getByRole('menuitem', { name: 'English' })).not.toHaveAttribute('aria-current')
  })

  it('switches the language of everything around it', async () => {
    const user = setupUser()
    renderAt('desktop', <LocaleActions />)

    await user.click(screen.getByRole('button', { name: 'Change language' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Русский' }))

    expect(await screen.findByRole('button', { name: 'Сменить язык' })).toBeInTheDocument()
  })
})
