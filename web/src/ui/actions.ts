/**
 * The glyph size every action control draws, and the only one.
 *
 * It exists because the sizes had drifted: a row's buttons were `compact-xs`,
 * a heading's were the default `md`, and the icons were a mix of 14, 16 and the
 * icon package's own 24 -- so the same three actions were drawn three different
 * sizes depending on which screen you were looking at.
 *
 * There used to be an `ACTION_SIZE` beside it, `'xs'`, passed at some twenty
 * call sites. It is gone, and not because the argument for it was wrong -- it
 * is because the argument now has a better home. A control's size is decided in
 * `ui/theme.ts` and `ui/app.css`, once, for the whole app and for each width;
 * a constant that every call site had to remember to pass was the same decision
 * made twenty times, and it silently said the same thing as the theme default
 * it sat beside. Anything left passing `size` is making a claim that this app
 * has exactly one control size per platform and it needs an exception.
 *
 * 16px is the glyph size the rest of the chrome already uses, and it is what
 * pairs with a button's label without crowding it. It is a *glyph* rather than
 * a control, which is why it survived: it is not a touch target and does not
 * change between a phone and a desktop.
 */
export const ACTION_ICON_SIZE = 16

/**
 * What a `Select` inside a `ModalSheet` must pass as `comboboxProps`.
 *
 * Mantine portals a dropdown to `document.body` by default, which puts it
 * *outside* the drawer on a phone -- and a drawer closes when a tap lands on
 * its overlay. So opening the picker in the invite sheet closed the sheet
 * underneath it, leaving the dropdown floating against the bottom of a page it
 * no longer belonged to, and the pair flashed as the two fought.
 *
 * Keeping the dropdown inside the sheet fixes it by making the tap land in the
 * sheet, which is where the person tapping thinks they are.
 *
 * It is a constant rather than a default on the theme because it is only right
 * *here*: a `Select` on an ordinary page may sit inside something with
 * `overflow: hidden` -- the build screen's carousel does -- where an
 * un-portalled dropdown is clipped instead.
 */
export const SHEET_COMBOBOX = { withinPortal: false } as const
