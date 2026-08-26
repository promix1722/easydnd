import { useMediaQuery } from '@mantine/hooks'

import { DESKTOP_MEDIA_QUERY } from '@/theme/tokens'

/**
 * The one viewport decision in the app.
 *
 * Feature code should almost never call this -- reach for a responsive
 * primitive (`ModalSheet`, `DataList`, `Columns`, `SectionDeck`) or Mantine's
 * responsive props instead. It is exported for the shell, which genuinely has
 * to pick between two different chromes, and for the rare component whose two
 * renderings share no markup at all.
 *
 * Returns false on the first render in SSR-less jsdom and before
 * `matchMedia` resolves, i.e. mobile-first: the narrow layout is the one that
 * degrades gracefully on a wide screen, not the other way round.
 */
export function useIsDesktop(): boolean {
  return useMediaQuery(DESKTOP_MEDIA_QUERY, false, { getInitialValueInEffect: false }) ?? false
}
