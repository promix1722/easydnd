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
export { Accordion, AppShell, Burger, NavLink, Tabs } from '@mantine/core'
// Its own package, and the one component here that is not core's. Re-exported
// plainly as well as wrapped: a carousel's two renderings are the same markup --
// the same slides, a narrower viewport -- so `routes/LandingPage.tsx` draws one
// directly. `SectionDeck` wraps it for the other reason a primitive exists here,
// which is that its *desktop* rendering is not a carousel at all.
// `Carousel.Slide` rides along as a static member.
export { Carousel } from '@mantine/carousel'
// Arrow keys and the wheel for a carousel that fills its page. Spread on to a
// `Carousel`; see the hook for what it borrows and when it gives it back.
export { useCarouselGestures } from './carouselGestures'
export { useDisclosure } from '@mantine/hooks'

// The icon set, named one glyph at a time. Re-exported for the same reason the
// chrome above is: `shell/` builds the header and may not import a vendor
// package, and `scripts/check-layers.mjs` holds both to that rule. Listing the
// handful the app actually draws -- rather than re-exporting the module -- is
// also what keeps the production bundle to those instead of six thousand.
export {
  IconArrowDown,
  IconArrowUp,
  IconCheck,
  IconChevronDown,
  IconChevronLeft,
  IconChevronRight,
  IconCopy,
  IconDice5,
  IconDotsVertical,
  IconFolder,
  IconFolderPlus,
  IconGripVertical,
  IconLanguage,
  IconLogout,
  IconPencil,
  IconPlus,
  IconShield,
  IconTrash,
  IconUserCircle,
  IconUserPlus,
  IconUsers,
  IconWand,
} from '@tabler/icons-react'

// Layout and typography primitives, re-exported unchanged.
export {
  Alert,
  Anchor,
  Badge,
  Box,
  Card,
  Center,
  Checkbox,
  Code,
  Divider,
  Group,
  Loader,
  Paper,
  // For the one thing that must escape its container: a dragged number, drawn
  // at the pointer. `position: fixed` is relative to the nearest transformed
  // ancestor, and a carousel's track is transformed on every frame.
  Portal,
  ScrollArea,
  SimpleGrid,
  Skeleton,
  Space,
  Stack,
  Text,
  Title,
  VisuallyHidden,
} from '@mantine/core'
export type { MantineColor, MantineSize } from '@mantine/core'

// Controls.
export {
  ActionIcon,
  Button,
  FileInput,
  // Per-row actions -- a roster's members, a character list's rows. The
  // alternative is four buttons in every row, which DataList's mobile card
  // rendering cannot lay out legibly.
  Menu,
  NumberInput,
  Popover,
  Select,
  Switch,
  Textarea,
  TextInput,
  Tooltip,
  // A button with no look of its own, for the one place a control has to be
  // pressable without being drawn as a control: a folder's name, which is a
  // heading you can collapse rather than a button that says its own name.
  UnstyledButton,
} from '@mantine/core'

// Composed, responsive-by-construction components.
export { BlockList } from './BlockList'
export type { BlockListItem, BlockListProps } from './BlockList'
export { Bullet } from './Bullet'
export type { BulletProps } from './Bullet'
export { Columns } from './Columns'
export type { ColumnsProps, ColumnsSection } from './Columns'
export { D20Roll } from './D20'
export type { D20RollProps } from './D20'
export { ACTION_ICON_SIZE, SHEET_COMBOBOX } from './actions'
export { DataList } from './DataList'
export type { ColumnSlot, DataListColumn, DataListProps, RowAction } from './DataList'
export { DragonMark } from './DragonMark'
export type { DragonMarkProps } from './DragonMark'
export { InstallAction } from './InstallAction'
export type { InstallActionProps } from './InstallAction'
export { InstallButton } from './InstallButton'
export { ModalSheet } from './ModalSheet'
export type { ModalSheetProps } from './ModalSheet'
export { UpdateGate } from './UpdateGate'
export { UpdateRequired } from './UpdateRequired'
export type { UpdateRequiredProps } from './UpdateRequired'
export { ProficiencyMark } from './ProficiencyMark'
export type { ProficiencyLevel, ProficiencyMarkProps } from './ProficiencyMark'
export { Page, PageBody } from './Page'
export type { Crumb, PageProps } from './Page'
export { pageState } from './pageState'
export type { PageState } from './pageState'
export { NO_SWIPE } from './swipe'
export { SECTIONS, sectionFor } from './sections'
export type { Section } from './sections'
export { SectionDeck } from './SectionDeck'
export type { DeckSection, SectionDeckProps } from './SectionDeck'
export { TabDeck } from './TabDeck'
export type { DeckPanel, TabDeckProps } from './TabDeck'
export { TabRow } from './TabRow'
export type { TabRowProps, TabRowTab } from './TabRow'

// Viewport access, for the shell and for components whose two renderings share
// no markup. Prefer the primitives above.
export { PAGE_BACKDROP } from './backdrop'
export { Panel } from './Panel'
export type { PanelProps } from './Panel'
export { useIsDesktop } from './useIsDesktop'

// The content cap, re-exported so a screen has one import surface for the
// design system rather than reaching past it into @/theme for one number.
export { CHROME_INSET, CONTENT_MAX_WIDTH, ROW_HEIGHT, TOUCH_TARGET } from '@/theme/tokens'

// Theme + provider, so main.tsx never imports Mantine either.
export { AppTheme } from './AppTheme'
export { theme } from './theme'
