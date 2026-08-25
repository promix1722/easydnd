import { ScrollArea, Tabs } from '@mantine/core'
import { useEffect, useRef } from 'react'
import type { ReactNode } from 'react'

export interface TabRowTab {
  value: string
  label: string
}

export interface TabRowProps {
  tabs: readonly TabRowTab[]
  value: string
  onChange: (value: string) => void
  /** Right-aligned controls that belong to the row rather than to a tab. */
  actions?: ReactNode
  /** The active tab's contents. */
  children?: ReactNode
}

/**
 * A row of tabs that scrolls sideways when it does not fit, with an actions
 * slot pinned to its right.
 *
 * A primitive rather than a call-site arrangement because Mantine's `Tabs.List`
 * offers only `grow` and `justify`: five tabs and two buttons across a 390px
 * phone need a strip that scrolls and a slot that does not, and neither is a
 * prop on the component library.
 *
 * It is the first responsive primitive whose **two renderings are the same
 * markup**. `ModalSheet` and `Columns` genuinely swap components at the
 * breakpoint; this one does not need to. A `ScrollArea type="never"` is inert
 * at a width the content fits in, so the desktop rendering is the mobile one
 * with nothing to scroll -- which means there is no second tree to keep
 * working, and a test at one width is a real test of the other.
 *
 * The active tab is brought into view by setting `scrollLeft` rather than by
 * `scrollIntoView`. The latter scrolls every scrollable ancestor, so selecting
 * a tab would drag the document as well as the strip -- and jsdom does not
 * implement it, which would make the behaviour untestable on top of wrong.
 */
export function TabRow({ tabs, value, onChange, actions, children }: TabRowProps) {
  const viewportRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef(new Map<string, HTMLButtonElement>())

  useEffect(() => {
    const viewport = viewportRef.current
    const tab = tabRefs.current.get(value)
    if (viewport === null || tab === undefined) return

    const left = tab.offsetLeft
    const right = left + tab.offsetWidth
    if (left < viewport.scrollLeft) {
      viewport.scrollLeft = left
    } else if (right > viewport.scrollLeft + viewport.clientWidth) {
      viewport.scrollLeft = right - viewport.clientWidth
    }
  }, [value])

  return (
    <Tabs
      value={value}
      onChange={(next) => {
        if (next !== null) onChange(next)
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--mantine-spacing-sm)' }}>
        <ScrollArea type="never" viewportRef={viewportRef} style={{ flex: 1, minWidth: 0 }}>
          <Tabs.List style={{ flexWrap: 'nowrap' }}>
            {tabs.map((tab) => (
              <Tabs.Tab
                key={tab.value}
                value={tab.value}
                ref={(element: HTMLButtonElement | null) => {
                  if (element === null) tabRefs.current.delete(tab.value)
                  else tabRefs.current.set(tab.value, element)
                }}
              >
                {tab.label}
              </Tabs.Tab>
            ))}
          </Tabs.List>
        </ScrollArea>
        {actions}
      </div>

      <Tabs.Panel value={value} pt="md">
        {children}
      </Tabs.Panel>
    </Tabs>
  )
}
