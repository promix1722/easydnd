import { useState } from 'react'

import { reorderFolders } from '@/lib/api'
import type { Folder } from '@/lib/api'
import { useAction } from '@/lib/useAction'

/** The order the folders are drawn in: the request that changes it, and the drag that asks for one. */
export function useFolderOrder(folderList: Folder[], folders: { refresh: () => void }) {
  const reorder = useAction(reorderFolders)

  // The folders that can move, in the order they are drawn. The default is not
  // among them: it leads the listing, and the server refuses an order naming
  // it.
  const movable = folderList.filter((folder) => !folder.default)

  /** Sends a whole new order, then reloads. There is no optimistic list. */
  const commit = (ids: readonly string[]) => {
    void reorder.run(ids).then((done) => {
      if (done !== null) folders.refresh()
    })
  }

  /** Moves the folder at `from` so that it sits at `to`. */
  const moveTo = (from: number, to: number) => {
    if (from === to) return
    const ids = movable.map((folder) => folder.id)
    const [moved] = ids.splice(from, 1)
    if (moved === undefined) return
    ids.splice(to, 0, moved)
    commit(ids)
  }

  /*
   * Dragging, hand-rolled over the native events -- the same shape as the
   * ability-score assignment, which is this app's other drag and was written
   * this way for the same two reasons. There is no drag library below `@/ui`
   * and adding one for two screens is not a trade worth making, and a native
   * drag fires on neither a touchscreen nor jsdom, so it can never be the only
   * way to do something. Move up and Move down in each folder's menu are the
   * real path; this is the shortcut for a mouse.
   */
  const [dragging, setDragging] = useState<number | null>(null)
  /**
   * Where the line is drawn: a gap between folders, 0 to `movable.length`, not
   * a folder's own index.
   *
   * A gap rather than a folder is the whole of the off-by-one this kind of list
   * is famous for. Dragging the first folder onto the second does not put it
   * *where* the second is -- it puts it after it -- so a line drawn above
   * whatever the pointer is over promises the opposite of what the drop does.
   * `lineOver` turns "the pointer is on this folder" into "the folder would
   * land in this gap", and `drop` turns the gap back into an index.
   */
  const [over, setOver] = useState<number | null>(null)

  /** The gap a drop on the folder at `at` would land in. */
  const lineOver = (at: number) => (dragging !== null && dragging < at ? at + 1 : at)

  const endDrag = () => {
    setDragging(null)
    setOver(null)
  }

  const drop = (at: number) => {
    if (dragging !== null) {
      const line = lineOver(at)
      // Closing the gap the folder came out of shifts every later index down
      // by one, which is what this subtraction is. It reduces to `at` in both
      // directions -- and that is the point of writing it out rather than
      // writing `at`: the line and the landing are one rule now, instead of
      // two that were supposed to agree and did not.
      moveTo(dragging, line > dragging ? line - 1 : line)
    }
    endDrag()
  }

  return { reorder, movable, moveTo, dragging, setDragging, over, setOver, lineOver, endDrag, drop }
}
