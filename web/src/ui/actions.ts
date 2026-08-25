/**
 * One size for every action control, and one for the glyph inside it.
 *
 * These exist because the sizes had drifted: a row's buttons were
 * `compact-xs`, a heading's were the default `md`, and the icons were a mix of
 * 14, 16 and the icon package's own 24 -- so the same three actions were drawn
 * three different sizes depending on which screen you were looking at.
 *
 * Small, because these are secondary to the rows they sit beside: a table is
 * the content and its controls are not. 16px is the glyph size the rest of the
 * chrome already uses, and it is what pairs with an `xs` button without
 * crowding its label.
 */
export const ACTION_SIZE = 'xs'
export const ACTION_ICON_SIZE = 16
