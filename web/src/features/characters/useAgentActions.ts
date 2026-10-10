import { useState } from 'react'
import type { RefObject } from 'react'
import type { NavigateFunction } from 'react-router'

import { chooseAgentRules, controlAgent, startAgentSession } from '@/lib/api/agent'
import type { AgentSession } from '@/lib/api/agent'
import type { PackRecord, PackRelease, RulesLock } from '@/lib/api/packs'
import type { Action } from '@/lib/useAction'

import { namedPack } from './agentChat'
import type { AgentSessionAction } from './useAgentSession'

/**
 * What the player does in a chat -- answering the rules question, sending a
 * message, stopping or resuming a turn, leaving for the character -- with the
 * message and the sheet they have not sent yet.
 */
export function useAgentActions({ session, action, choose, packs, opening, ruled, myTurn, follow, sessionId, folder, navigate }: {
  session: AgentSession | undefined
  action: AgentSessionAction
  choose: Action<[PackRelease[]], RulesLock>
  packs: PackRecord[] | undefined
  opening: boolean
  ruled: boolean
  myTurn: boolean
  follow: RefObject<boolean>
  sessionId: string | undefined
  folder: string | undefined
  navigate: NavigateFunction
}) {
  const [files, setFiles] = useState<File[]>([])
  const [message, setMessage] = useState('')
  async function send(answer = message) {
    if (!session || !myTurn || action.pending || choose.pending || (!answer.trim() && !files.length)) return
    follow.current = true
    let revision = session.revision
    // Written before the rules were pressed, a message may be the answer to
    // that question: the pack it names. One that says nothing more is only
    // that. One that names none leaves the rules to the server, which takes
    // the deployment's own and says so in the chat.
    if (opening && !ruled) {
      const named = namedPack(offered, answer)
      if (named) {
        const chosen = await chooseRules(named.pack.label)
        if (!chosen) return
        if (named.only && !files.length) return setMessage('')
        revision = chosen.session.revision
      }
    }
    const result = await action.run(() =>
      // A sheet is attached to the first message and to no other.
      opening
        ? startAgentSession(session.id, revision, files, answer)
        : controlAgent(session.id, revision, 'message', answer),
    )
    if (result) {
      setMessage('')
      setFiles([])
      if (!sessionId)
        void navigate(
          `/ai-wizard/${result.session.id}${folder ? `?folder=${encodeURIComponent(folder)}` : ''}`,
          { replace: true },
        )
    }
  }
  // The answer to the opening question, by the caption of the button pressed.
  const offered = (packs ?? [])
    .filter((pack) => !pack.archived && pack.releases.length > 0)
    .map((pack) => ({
      id: pack.id,
      title: pack.title,
      label: `${pack.title} v${pack.releases.at(-1)!.version}`,
      release: pack.releases.at(-1)!,
    }))
  async function chooseRules(label: string) {
    const release = offered.find((pack) => pack.label === label)?.release
    if (!session || !release) return null
    const lock = await choose.run([release])
    return lock ? action.run(() => chooseAgentRules(session.id, session.revision, lock)) : null
  }
  async function control(kind: string) {
    if (!session) return
    const result = await action.run(() => controlAgent(session.id, session.revision, kind))
    if (result && kind === 'discard') void navigate('/')
  }
  // Finish is View under the name the end of a conversation has: the
  // character is already real, so there is nothing to save, only somewhere to go.
  const open = (kind: 'view' | 'edit' | 'finish') => {
    if (!session?.characterId) return
    // Finish is what closes the chat: until it is pressed the wizard comes
    // back here, however done the assistant thinks the character is.
    if (kind === 'finish') void controlAgent(session.id, session.revision, 'finish').catch(() => {})
    void navigate(`/characters/${session.characterId}${kind === 'edit' ? '/build' : ''}`)
  }
  return { message, setMessage, files, setFiles, offered, send, chooseRules, control, open }
}
