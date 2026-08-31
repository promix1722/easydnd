import { LOCALE_FLAGS, LOCALE_NAMES, LOCALES, useLocale, useSetLocale, useT } from '@/lib/i18n'
import { ActionIcon, IconCheck, IconLanguage, Menu, Tooltip } from '@/ui'

/**
 * Choosing the language, from the corner of every header.
 *
 * It is drawn by both `AccountActions` and `SignInActions`, so it is reachable
 * signed in, signed out, and as a guest -- which is the whole point of not
 * storing the choice on an account. A visitor who cannot read the landing page
 * cannot be asked to make an account first in order to be able to.
 *
 * Each row in the menu carries its country's flag beside the name. It is
 * decoration and `aria-hidden`: the name is what identifies the language, and a
 * flag would be the wrong thing to identify one by -- see `LOCALE_FLAGS`.
 *
 * A glyph rather than the two words, for the same reason the account and the
 * way out are glyphs: on a 390px phone this row already carries a wordmark, a
 * section dropdown and two controls, and "English"/"Русский" is more of it than
 * a language switcher has earned. The languages inside the menu are written in
 * their own names and never translated -- somebody looking for Russian is
 * looking for "Русский", not for whatever English calls it.
 *
 * The tick marks the current one rather than the trigger naming it. Two
 * languages make a menu that is entirely visible the moment it opens, so the
 * question "which am I in?" is answered by opening it -- and the alternative,
 * a trigger reading "EN", spends the same width on saying what the page around
 * it is already demonstrating.
 */
export function LocaleActions() {
  const t = useT()
  const locale = useLocale()
  const setLocale = useSetLocale()

  return (
    <Menu position="bottom-end" withinPortal>
      <Menu.Target>
        {/* Tooltip inside Target: Mantine's Menu.Target needs a single child
            it can attach its own ref and aria to, and the tooltip wraps the
            control rather than replacing it. Same shape as AccountActions. */}
        <Tooltip label={t('locale.change')} withArrow>
          <ActionIcon variant="subtle" aria-label={t('locale.change')}>
            <IconLanguage size={20} />
          </ActionIcon>
        </Tooltip>
      </Menu.Target>
      <Menu.Dropdown>
        {LOCALES.map((each) => (
          <Menu.Item
            key={each}
            onClick={() => setLocale(each)}
            // The flag is decoration beside the name, not the label -- see
            // LOCALE_FLAGS. Emoji rather than an icon, so it costs nothing.
            leftSection={<span aria-hidden>{LOCALE_FLAGS[each]}</span>}
            // The tick is hidden rather than absent on the languages that are
            // not current, so the names stay in one column as you move down
            // the menu. Same trick as the section dropdown in MobileShell.
            rightSection={
              <IconCheck size={16} style={{ visibility: each === locale ? 'visible' : 'hidden' }} />
            }
            aria-current={each === locale ? 'true' : undefined}
          >
            {LOCALE_NAMES[each]}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  )
}
