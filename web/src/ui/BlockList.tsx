import { Accordion } from '@mantine/core'
import type { ReactNode } from 'react'

export interface BlockListItem {
  key: string
  /** Text, badges, a value line -- anything that is not itself pressable. */
  header: ReactNode
  /** Absent where the block is a statement rather than a way in. */
  body?: ReactNode
  /** Drawn to stand out: something is still wanted here. */
  highlighted?: boolean
}

export interface BlockListProps {
  items: readonly BlockListItem[]
  /** The key of the one block that is open, or null for none. */
  open: string | null
  onOpen: (key: string | null) => void
}

/**
 * A list of blocks, one of which is open, each opening onto its own contents.
 *
 * The pattern this replaced was a list of headings in one card and the thing
 * they opened in another, which asks a reader to hold a name in their head
 * while they look somewhere else for what it means. A block that opens where
 * it stands keeps the question and its answer in one place.
 *
 * **One open at a time.** `open` is a key, not a set: there is one thing being
 * dealt with, and on a 390px screen two open bodies are a scroll rather than a
 * choice. Pressing the open block closes it, which arrives here as `null`.
 *
 * **A body is mounted only while its block is open.** Not a styling nicety --
 * a body may fetch what it needs on mount, and a list of ten collapsed blocks
 * that each opened a request on every paint would be paying ten times over for
 * nine things nobody is looking at. `body` stays an element rather than a
 * thunk: building it is free, and only mounting it runs anything.
 *
 * **A block with no body is a statement, not a disabled control.** A disabled
 * button says "not just now"; some rows are facts, and a fact should not look
 * like a thing that failed to be pressable.
 *
 * Like `TabRow`, and unlike `ModalSheet` and `Columns`, its two renderings are
 * the same markup: a stack of bordered disclosures is right at 390px and at
 * 1440px, so there is no second tree to keep working and a test at one width
 * is a real test of the other.
 *
 * Headers must contain nothing interactive. The control is a `<button>`, and a
 * button inside a button is neither clickable nor legal.
 */
export function BlockList({ items, open, onOpen }: BlockListProps) {
  return (
    <Accordion
      variant="separated"
      radius="md"
      chevronPosition="right"
      value={open}
      onChange={onOpen}
    >
      {items.map((item) => (
        <Accordion.Item
          key={item.key}
          value={item.key}
          data-highlighted={item.highlighted === true ? 'true' : undefined}
          style={
            item.highlighted === true
              ? { borderColor: 'var(--mantine-primary-color-filled)' }
              : undefined
          }
        >
          {item.body === undefined ? (
            <div style={{ padding: 'var(--mantine-spacing-md)' }}>{item.header}</div>
          ) : (
            <>
              <Accordion.Control>{item.header}</Accordion.Control>
              <Accordion.Panel>{open === item.key ? item.body : null}</Accordion.Panel>
            </>
          )}
        </Accordion.Item>
      ))}
    </Accordion>
  )
}
