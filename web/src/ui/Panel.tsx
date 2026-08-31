import { Paper } from '@mantine/core'
import type { ReactNode } from 'react'

export interface PanelProps {
  children: ReactNode
}

/**
 * A sheet for a screen's content to sit on, so it reads against the page.
 *
 * The page's ground is a tiled drawing now -- see `ui/backdrop.ts` -- and a
 * table laid straight onto it is a column of words over a pattern: the header
 * rule and the row separators are hairlines, and hairlines are exactly what a
 * busy background eats. This is the same `Paper` the folder panels on the
 * character list have always used, generalised so that every screen that needs
 * one asks for it by name rather than spelling out three props.
 *
 * `Paper` fills with `--mantine-color-body`, the page's own background colour,
 * so the pattern stops at its border in both colour schemes with nothing said
 * here about either.
 *
 * It is not `ui/Page`'s job. A page's heading, its trail and its actions belong
 * *outside* the sheet -- they say where you are, and where you are is not part
 * of what you are looking at -- and some screens have nothing to frame at all.
 */
export function Panel({ children }: PanelProps) {
  return (
    <Paper withBorder radius="md" p="md">
      {children}
    </Paper>
  )
}
