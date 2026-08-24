import { request } from './client'

/**
 * Folders.
 *
 * A folder is a named place one account files its characters, and nothing
 * more: one owner, nothing shared with anybody. It is deliberately not called a
 * group -- that word is kept for a group of *players*, which is a different
 * idea with a different owner model, and the two must not be confusable in the
 * one place a reader looks them up.
 *
 * Shapes mirror internal/api/http/v1/folder.
 */

export interface Folder {
  id: string
  name: string
  /**
   * The folder every account has and none can delete. A client leaves the
   * delete control off this row, and it is where a character with no folder
   * named will land.
   */
  default: boolean
}

/**
 * Lists the account's folders, the default first.
 *
 * It never comes back empty: the server creates the default folder on the
 * first read, so a brand-new account already has one by the time this resolves.
 */
export function listFolders(signal?: AbortSignal): Promise<{ folders: Folder[] }> {
  return request<{ folders: Folder[] }>('/folders', signal ? { signal } : {})
}

export function createFolder(name: string): Promise<Folder> {
  return request<Folder>('/folders', { method: 'POST', body: { name } })
}

/** Renames a folder. The default folder can be renamed like any other. */
export function renameFolder(id: string, name: string): Promise<Folder> {
  return request<Folder>(`/folders/${id}`, { method: 'PATCH', body: { name } })
}

/**
 * Deletes a folder **and every character in it**.
 *
 * There is no undo and characters live in memory, so there is not even a
 * backup to go back to. Callers must confirm first, and the confirmation should
 * say how many characters are about to go. The default folder is refused.
 */
export function deleteFolder(id: string): Promise<void> {
  return request<void>(`/folders/${id}`, { method: 'DELETE' })
}
