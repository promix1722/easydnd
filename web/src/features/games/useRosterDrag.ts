import { useRef, useState, type PointerEvent } from 'react'

import type { GameEntry } from '@/lib/api'

/** The roster's drag-and-drop ordering: one pointer gesture, and where it would land. */
export function useRosterDrag({ gameId, entries, draggable, onMove }: {
  gameId: string
  entries: readonly GameEntry[]
  draggable: boolean
  onMove: (entryId: string, beforeId: string) => void
}) {
  const [dragging, setDragging] = useState<string | null>(null)
  const gesture = useRef<{ id: string; pointer: number; x: number; y: number; moved: boolean } | null>(null)
  const [over, setOver] = useState<string | null>(null)

  function endDrag() {
    gesture.current = null
    setDragging(null)
    setOver(null)
  }

  // Match folder ordering while using one gesture for mouse, touch and stylus.
  // Hit-test the real pointer position because pointer capture keeps events
  // arriving on the grip even when the pointer is over another row.
  function beforeAtPoint(from: string, x: number, y: number): string | null {
    const start = entries.findIndex((entry) => entry.id === from)
    if (start < 0) return null
    const element = document.elementFromPoint?.(x, y)
    if (element?.closest('[data-game-roster]')?.getAttribute('data-game-roster') !== gameId) return null
    if (element?.closest('[data-game-drop-end]')) return ''
    const target = element?.closest('[data-game-entry]')?.getAttribute('data-game-entry')
    const index = entries.findIndex((entry) => entry.id === target)
    if (index < 0) return null
    const gap = start < index ? index + 1 : index
    return entries[gap]?.id ?? ''
  }

  function startDrag(event: PointerEvent<HTMLButtonElement>, id: string) {
    if (!draggable || gesture.current || event.isPrimary === false || (event.pointerType === 'mouse' && event.button !== 0)) return
    event.preventDefault()
    event.currentTarget.setPointerCapture?.(event.pointerId)
    gesture.current = { id, pointer: event.pointerId, x: event.clientX, y: event.clientY, moved: false }
  }

  function moveDrag(event: PointerEvent<HTMLButtonElement>) {
    const drag = gesture.current
    if (!drag || event.pointerId !== drag.pointer) return
    if (!draggable || !entries.some((entry) => entry.id === drag.id)) { endDrag(); return }
    drag.moved ||= Math.hypot(event.clientX - drag.x, event.clientY - drag.y) >= 8
    if (!drag.moved) return
    event.preventDefault()
    setDragging(drag.id)
    setOver(beforeAtPoint(drag.id, event.clientX, event.clientY))
  }

  function drop(event: PointerEvent<HTMLButtonElement>) {
    const drag = gesture.current
    if (!drag || event.pointerId !== drag.pointer) return
    const before = beforeAtPoint(drag.id, event.clientX, event.clientY)
    if (draggable && drag.moved && before !== null && before !== drag.id) {
      onMove(drag.id, before)
    }
    endDrag()
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
  }

  function cancelDrag(event: PointerEvent<HTMLButtonElement>) {
    if (event.pointerId === gesture.current?.pointer) endDrag()
  }

  return { dragging, over, startDrag, moveDrag, drop, cancelDrag }
}
