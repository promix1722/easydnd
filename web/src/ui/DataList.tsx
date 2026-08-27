import { Anchor, Box, Button, Group, Menu, Paper, Stack, Table, Text } from '@mantine/core'
import type { MantineColor } from '@mantine/core'
import { IconDotsVertical } from '@tabler/icons-react'
import type { ComponentProps, ReactNode } from 'react'
import { Link } from 'react-router'

import { ACTION_ICON_SIZE } from './actions'
import { useIsDesktop } from './useIsDesktop'

import { useT } from '@/lib/i18n'
import type { Translate } from '@/lib/i18n'

/**
 * One thing a row can be made to do, as data rather than as a button.
 *
 * The shape is the point. A cluster of `<Button>`s can only be drawn one way,
 * and a phone needs the same three actions drawn as menu items -- so what the
 * caller hands over is the decision, and `DataList` renders it twice. That also
 * moves two rules out of prose and into construction: every action gets its
 * row's name in its accessible name (a column of buttons all called "Delete" is
 * ambiguous to a screen reader and to a test alike), and the size and variant
 * are decided here rather than at each of the twenty call sites that used to
 * spell them out.
 */
export interface RowAction {
  key: string
  /** What it reads, and the first half of what a screen reader announces. */
  label: string
  onClick: () => void
  /** Drawn on the desktop button and in the phone's menu item alike. */
  icon?: ReactNode
  /** `red` for the one that cannot be undone. */
  color?: MantineColor
  disabled?: boolean
}

interface ColumnBase<T> {
  key: string
  header: string
  /** The desktop cell. Every existing call site's `render` is unchanged. */
  render: (item: T) => ReactNode
}

/**
 * Where a column goes on a phone, where there are no columns.
 *
 * - `meta` (the default) joins the dimmed line under the name.
 * - `badge` rides *beside* the name: a `<Badge>` in a run of dimmed text
 *   separated by dots reads as neither.
 * - `block` gets a full-width line of its own under the meta, headed by the
 *   column's own header. For content that is not a value at all -- the event
 *   log's `Detail` is a stack of `<Code>` elements, and joining that with a
 *   middle dot would be nonsense.
 */
export type ColumnSlot = 'meta' | 'badge' | 'block'

export type DataListColumn<T> =
  | (ColumnBase<T> & {
      /**
       * Marks the column that identifies the row. Exactly one should.
       *
       * On a phone it becomes the card's heading, and `DataList` styles it --
       * which is the fix for the thing that made every card look wrong: each
       * call site used to wrap its own name in `<Text size="sm">`, so the
       * heading came out the same size as the fields beneath it.
       */
      primary: true
      /** The name as a string: the card's heading, and every action's suffix. */
      text: (item: T) => string
      /** Where the name goes. The desktop cell keeps its own `render`. */
      to?: (item: T) => string
    })
  | (ColumnBase<T> & { primary?: false; slot?: ColumnSlot })

export interface DataListProps<T> {
  items: readonly T[]
  columns: ReadonlyArray<DataListColumn<T>>
  getKey: (item: T) => string
  /**
   * Marks that ride with the name at both widths -- "You", "Guest", "Yours".
   *
   * Distinct from a `badge` column, and the difference is whether the thing has
   * a column at all: a group's *rank* is a fact with a header, drawn in the
   * table as its own column; "Guest" is a mark on the name that no table ever
   * gave a column to. They used to be the same thing only because both were
   * hand-built inside the primary column's `render`, which is what put a `div`
   * inside the `<p>` that `<Text>` renders.
   */
  badges?: (item: T) => ReactNode
  /** Every action on the row. An empty array draws no control at all. */
  actions?: (item: T) => readonly RowAction[]
  empty?: ReactNode
}

/**
 * A table on desktop, a stack of cards on a phone.
 *
 * The mobile rendering used to re-label each cell -- a bold line, then
 * `Class: --`, then `Level: 0`, then whatever the actions column happened to
 * render, on a text line of its own. Three things were wrong with that and only
 * the first was cosmetic: a card is not a table row with the headers moved, a
 * `<Group>` inside a `<Text>` is a `div` inside a `p` and the browser closes
 * the paragraph early, and a cluster of buttons has nowhere to go.
 *
 * What replaced it is the shape `features/characters/FolderPanel` already had,
 * and which was the one list in the app that read well on a phone: a bordered
 * `Paper`, a `wrap="nowrap"` row with a `flex: 1; minWidth: 0` name, marks
 * beside it, and every action behind one `IconDotsVertical` menu.
 *
 * **The desktop table is unchanged**, down to the actions being spelled out as
 * buttons. That is deliberate: `docs/web.md` argues a table's row actions
 * should be visible without opening anything, and a wide screen has the room.
 * A phone does not, which is the whole of the difference.
 */
export function DataList<T>({ items, columns, getKey, badges, actions, empty }: DataListProps<T>) {
  const t = useT()
  const isDesktop = useIsDesktop()

  if (items.length === 0) {
    return (
      <Text c="dimmed" size="sm">
        {empty ?? t('dataList.empty')}
      </Text>
    )
  }

  const primary = columns.find((column) => column.primary === true)
  const rest = columns.filter((column) => column.primary !== true)

  /**
   * Whether *any* row here can be acted on.
   *
   * Not whether this one can. A list where some rows carry a menu and others do
   * not would otherwise have a ragged right edge -- the name on an actionless
   * row running 44px further than its neighbours -- and a straight edge is
   * worth an empty gutter on the rows that do not need one. A list where
   * nothing is actionable reserves nothing.
   */
  const anyActions = actions !== undefined && items.some((item) => actions(item).length > 0)

  if (isDesktop) {
    return (
      <Box>
        <Table verticalSpacing="sm">
          <Table.Thead>
            <Table.Tr>
              {columns.map((column) => (
                <Table.Th key={column.key}>{column.header}</Table.Th>
              ))}
              {/* Headerless by design: a column of buttons needs no title. */}
              {anyActions && <Table.Th />}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {items.map((item) => (
              <Table.Tr key={getKey(item)}>
                {columns.map((column) => (
                  <Table.Td key={column.key}>
                    {/* The marks ride with the name at *both* widths -- they
                        are a fact about the row, not a consolation for having
                        no columns. They sit in the identifying cell because
                        that is where each call site used to build them by
                        hand, inside the very `<Text>` that made them a `div`
                        in a paragraph. */}
                    {column.primary === true && badges !== undefined ? (
                      <Group gap="xs" wrap="nowrap">
                        {column.render(item)}
                        {badges(item)}
                      </Group>
                    ) : (
                      column.render(item)
                    )}
                  </Table.Td>
                ))}
                {anyActions && (
                  <Table.Td>
                    <DesktopActions
                      actions={actions(item)}
                      name={primary?.primary === true ? primary.text(item) : ''}
                    />
                  </Table.Td>
                )}
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Box>
    )
  }

  const meta = rest.filter((column) => (column.slot ?? 'meta') === 'meta')
  const badgeColumns = rest.filter((column) => column.slot === 'badge')
  const blocks = rest.filter((column) => column.slot === 'block')

  return (
    <Stack gap="sm">
      {items.map((item) => {
        const name = primary?.primary === true ? primary.text(item) : ''
        const to = primary?.primary === true ? primary.to?.(item) : undefined
        const rowActions = actions?.(item) ?? []
        const facts = meta.map((column) => column.render(item)).filter(saysSomething)

        return (
          <Paper key={getKey(item)} withBorder radius="md" p="xs">
            {/* Centred when the name is the only line, so a one-line row does
                not leave its name sitting at the top of a taller control.
                Top-aligned once there is a second line, where the control
                belongs beside the name rather than between the two. */}
            <Group gap="xs" wrap="nowrap" align={facts.length > 0 ? 'flex-start' : 'center'}>
              <Box style={{ flex: 1, minWidth: 0 }}>
                <Group gap="xs" wrap="nowrap">
                  {/* `minWidth: 0` is what lets `truncate` fire: without it a
                      flex item refuses to shrink below its content, and a long
                      name pushes the marks and the menu off the card. */}
                  <Box style={{ flex: 1, minWidth: 0 }}>
                    {to === undefined ? (
                      <Text fw={650} truncate>
                        {name}
                      </Text>
                    ) : (
                      <Anchor component={Link} to={to} fw={650} truncate>
                        {name}
                      </Anchor>
                    )}
                  </Box>
                  {badges?.(item)}
                  {badgeColumns.map((column) => (
                    <Box key={column.key} style={{ flexShrink: 0 }}>
                      {column.render(item)}
                    </Box>
                  ))}
                </Group>

                {/* Values, joined -- no `Header:` prefix. Two lines at most:
                    a multiclass line can run long, and cutting it mid-word
                    would lose the level that follows it. */}
                {facts.length > 0 && (
                  <Text size="sm" c="dimmed" lineClamp={2}>
                    {facts.map((fact, index) => (
                      <span key={meta[index]?.key ?? index}>
                        {index > 0 && ' · '}
                        {fact}
                      </span>
                    ))}
                  </Text>
                )}
              </Box>

              {rowActions.length > 0 ? (
                <RowMenu actions={rowActions} name={name} />
              ) : (
                // The gutter a row without actions leaves, so a list where only
                // some rows can be acted on keeps one right edge and one row
                // depth. It is the control itself, hidden, rather than a box of
                // the same size: two numbers that have to agree eventually
                // stop agreeing, and this pair cannot.
                anyActions && (
                  <Box aria-hidden style={{ visibility: 'hidden' }}>
                    <ActionsButton name="" />
                  </Box>
                )
              )}
            </Group>

            {blocks.map((column) => (
              <Box key={column.key} mt="xs">
                <Text size="xs" c="dimmed" tt="uppercase">
                  {column.header}
                </Text>
                {column.render(item)}
              </Box>
            ))}
          </Paper>
        )
      })}
    </Stack>
  )
}


/**
 * The control every row's actions hide behind, drawn exactly as
 * `features/characters/FolderPanel` draws its own: an ordinary button at the
 * app's one size, padded in to the width of its glyph.
 *
 * Extracted because the empty gutter beside an actionless row is this same
 * element with `visibility: hidden`, which is the only way to be sure the two
 * are the same size.
 */
function ActionsButton({ name, ...rest }: { name: string } & ComponentProps<typeof Button>) {
  const t = useT()
  return (
    <Button
      variant="subtle"
      px={6}
      style={{ flexShrink: 0 }}
      aria-label={t('list.actions', { name })}
      // Spread last, and this is load-bearing: `Menu.Target` clones its child
      // to attach the ref and the handlers that open the dropdown. A component
      // that drops them is a button that does nothing, which is what the first
      // version of this was.
      {...rest}
    >
      <IconDotsVertical size={ACTION_ICON_SIZE} />
    </Button>
  )
}

/**
 * A value that has something to say.
 *
 * `'--'` is in here because it is what a *table* uses for nothing: a blank cell
 * in a grid of them reads as a rendering fault, so `domain/classLine` returns
 * two dashes and two call sites write `level || '--'` by hand. A card has no
 * column to keep aligned and should simply not mention it -- which is the
 * difference between "Class: --  Level: 0" and a name with nothing under it.
 */
function saysSomething(value: ReactNode): boolean {
  return value !== null && value !== undefined && value !== false && value !== '' && value !== '--'
}

/** Spelled out, because a wide screen has room and a table's actions should be visible. */
function DesktopActions({ actions, name }: { actions: readonly RowAction[]; name: string }) {
  const t = useT()
  if (actions.length === 0) return null
  // Four is where a row stops being able to lay them out -- the same threshold
  // `FolderPanel` settled on for its own header, and the reason it has a menu.
  if (actions.length > 3) return <RowMenu actions={actions} name={name} />

  return (
    <Group gap="xs" justify="flex-end" wrap="nowrap">
      {actions.map((action) => (
        <Button
          key={action.key}
          variant="subtle"
          // Spread rather than passed: `exactOptionalPropertyTypes` makes
          // `color={undefined}` a different thing from omitting `color`.
          {...(action.color === undefined ? {} : { color: action.color })}
          {...(action.disabled === undefined ? {} : { disabled: action.disabled })}
          {...(action.icon === undefined ? {} : { leftSection: action.icon })}
          aria-label={accessibleName(t, action.label, name)}
          onClick={action.onClick}
        >
          {action.label}
        </Button>
      ))}
    </Group>
  )
}

/**
 * Every action behind one control.
 *
 * The target carries the row's name, exactly as `FolderPanel`'s does, so a
 * screen full of these is not a screen full of buttons called the same thing.
 * The items inside do not need it: they are already inside a menu that said
 * whose row it is.
 */
function RowMenu({ actions, name }: { actions: readonly RowAction[]; name: string }) {
  return (
    <Menu position="bottom-end" withinPortal>
      <Menu.Target>
        <ActionsButton name={name} />
      </Menu.Target>
      <Menu.Dropdown>
        {actions.map((action) => (
          <Menu.Item
            key={action.key}
            {...(action.color === undefined ? {} : { color: action.color })}
            {...(action.disabled === undefined ? {} : { disabled: action.disabled })}
            {...(action.icon === undefined ? {} : { leftSection: action.icon })}
            onClick={action.onClick}
          >
            {action.label}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  )
}

/**
 * "Delete Ada" -- the convention every row action in the app already followed.
 *
 * A key rather than two strings joined here, because the order of the two is a
 * fact about the language and not about this component.
 */
function accessibleName(t: Translate, label: string, name: string): string {
  return t('list.rowAction', { label, name })
}
