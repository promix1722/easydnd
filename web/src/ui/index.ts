/**
 * The application's design system -- the single import surface for anything
 * visual.
 *
 * Everything outside `src/ui/**` imports from `@/ui`, never from `@mantine/*`
 * directly; `scripts/check-layers.mjs` fails the build otherwise. That one
 * indirection is what makes the component library swappable, gives every
 * project-wide default exactly one home, and keeps the desktop/mobile split
 * confined to this directory instead of leaking into feature code.
 */

// Application chrome. Re-exported so that src/shell -- which builds the two
// layouts -- needs no Mantine import of its own, keeping the rule absolute.
export { AppShell, Burger, NavLink, Tabs } from '@mantine/core'
// Its own package, and the one component here that is not core's. Re-exported
// plainly rather than wrapped the way Columns and ModalSheet are: those exist
// because their phone and desktop renderings share no markup, and a carousel's
// do -- the same slides, a narrower viewport. `Carousel.Slide` rides along as a
// static member.
export { Carousel } from '@mantine/carousel'
export { useDisclosure } from '@mantine/hooks'

// Layout and typography primitives, re-exported unchanged.
export {
  Alert,
  Anchor,
  Badge,
  Box,
  Card,
  Center,
  Code,
  Container,
  Divider,
  Group,
  Loader,
  Paper,
  ScrollArea,
  SimpleGrid,
  Skeleton,
  Space,
  Stack,
  Text,
  Title,
} from '@mantine/core'
export type { MantineColor, MantineSize } from '@mantine/core'

// Controls.
export {
  ActionIcon,
  Button,
  FileInput,
  // Per-row actions -- a roster's members, a party list's characters. The
  // alternative is four buttons in every row, which DataList's mobile card
  // rendering cannot lay out legibly.
  Menu,
  NumberInput,
  Popover,
  Select,
  Switch,
  TextInput,
  Tooltip,
} from '@mantine/core'

// Composed, responsive-by-construction components.
export { BlockList } from './BlockList'
export type { BlockListItem, BlockListProps } from './BlockList'
export { Columns } from './Columns'
export type { ColumnsProps, ColumnsSection } from './Columns'
export { DataList } from './DataList'
export type { DataListColumn, DataListProps } from './DataList'
export { DragonMark } from './DragonMark'
export type { DragonMarkProps } from './DragonMark'
export { ModalSheet } from './ModalSheet'
export type { ModalSheetProps } from './ModalSheet'
export { ProficiencyMark } from './ProficiencyMark'
export type { ProficiencyLevel, ProficiencyMarkProps } from './ProficiencyMark'
export { TabRow } from './TabRow'
export type { TabRowProps, TabRowTab } from './TabRow'

// Viewport access, for the shell and for components whose two renderings share
// no markup. Prefer the primitives above.
export { useIsDesktop } from './useIsDesktop'

// Theme + provider, so main.tsx never imports Mantine either.
export { AppTheme } from './AppTheme'
export { theme } from './theme'
