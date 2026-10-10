import { useEffect, useRef } from 'react'
import { useNavigate, useSearchParams, useParams } from 'react-router'
import { useAction } from '@/lib/useAction'
import { useReveal } from './useReveal'
import { useT } from '@/lib/i18n'
import {
  ActionIcon,
  Alert,
  Button,
  Card,
  IconArrowDown,
  Group,
  Page,
  Stack,
} from '@/ui'
import type { AgentEvent } from '@/lib/api/agent'
import { listPacks, resolvePacks } from '@/lib/api/packs'
import { useResource } from '@/lib/useResource'
import { AgentComposer } from './AgentComposer'
import { Bubble, Conversation, MessageText } from './AgentConversation'
import { chatName } from './agentChat'
import { useAgentActions } from './useAgentActions'
import { useAgentSession } from './useAgentSession'
import { useChatScroll } from './useChatScroll'

const EMPTY: AgentEvent[] = []

/** Stream events are rendered from recorded state, so reconnect/reload never
 * creates a second conversation or restarts an import. The chat writes to a
 * real character from its first message: View and Edit are that character's
 * own sheet and builder, not pages of this one.
 *
 * It is a conversation the assistant leads. It asks -- which rules, a sheet or
 * a description, then whatever the build leaves open -- and the player
 * answers: by pressing one of the answers it offers, or by writing, whichever
 * they like and from the first question on. There is nothing above the
 * transcript but the transcript. */
export function AgentImportScreen() {
  const t = useT()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const { sessionId } = useParams()
  const folder = params.get('folder') ?? undefined
  const packs = useResource('pack-selection', listPacks)
  // Choosing the rules is answering a question like any other: one button per
  // pack. What a pack depends on comes with it.
  const choose = useAction(resolvePacks)
  const { id, view, action, connected, forget } = useAgentSession(sessionId ?? params.get('session'), folder)
  const composer = useRef<HTMLTextAreaElement>(null)
  const session = view?.session
  // The transcript as it is shown: one bubble at a time, the assistant's words
  // typed out. Until all of it is on screen the turn is still the assistant's.
  const reveal = useReveal(session?.id, session?.events ?? EMPTY)
  const { frame, end, follow, away, toEnd } = useChatScroll(reveal, session?.status)
  const active = session?.status === 'running' || session?.status === 'queued' || !reveal.settled
  // The player's turn: the assistant has asked or finished. Its first
  // question -- which rules -- is a question like the rest.
  const stalled = reveal.settled && (session?.status === 'paused' || session?.status === 'failed')
  // A finished chat is a record: nothing more is said in it.
  const finished = session?.finished === true
  // An opened chat is waiting for its rules, and then for the first message.
  const opening = session?.status === 'opening'
  const ruled = session?.events.some((event) => event.kind === 'rules') === true
  const myTurn = !!session && !active && !finished
  useEffect(() => {
    // Without scrolling: a focus brings its field into view by the shortest
    // way, which stops the smooth scroll to the end half way and leaves the
    // foot of the message box under the window's edge.
    if (myTurn) composer.current?.focus({ preventScroll: true })
  }, [myTurn])
  const { message, setMessage, files, setFiles, offered, send, chooseRules, control, open } = useAgentActions({
    session, action, choose, packs: packs.data?.packs, opening, ruled, myTurn, follow, sessionId, folder, navigate,
  })
  // Named once the chat has said when it was opened: a name that began as a
  // piece of the id and grew a date a moment later would be two names.
  const trail = session ? [{ label: chatName(session.created, session.id) }] : []
  // The same four buttons, drawn the same, beside the page's name and under
  // the assistant's last message.
  const actions = session?.characterId ? (
    <Group gap="xs">
      <Button variant="default" onClick={() => open('view')}>
        {t('import.viewSheet')}
      </Button>
      <Button variant="default" onClick={() => open('edit')}>
        {t('common.edit')}
      </Button>
      {/* Whenever the assistant has stopped, whatever it stopped on: a chat
          can always be closed. */}
      {!active && !finished && <Button onClick={() => open('finish')}>{t('agent.finish')}</Button>}
      {!finished && (
        <Button disabled={active || action.pending} onClick={() => void control('discard')}>
          {t('agent.discard')}
        </Button>
      )}
    </Group>
  ) : undefined
  const packNames = (releases: readonly { id: string; version: string }[]) =>
    releases
      .map((release) => `${packs.data?.packs.find((pack) => pack.id === release.id)?.title ?? release.id} v${release.version}`)
      .join(', ')
  return (
    <Page
      trail={trail}
      // What can be done with the character, beside the page's name and not
      // in the conversation: it is always the same four things, and a chat is
      // for what changes.
      actions={actions}
    >
      <Stack gap="md">
        {action.error && (
          <Alert color="red">
            {action.error}
            {id && !view && (
              <Button
                variant="subtle"
                onClick={() => {
                  action.reset()
                  forget()
                  setParams(folder ? { folder } : {})
                }}
              >
                {t('agent.newImport')}
              </Button>
            )}
          </Alert>
        )}
        {!connected && <Alert color="yellow">{t('agent.reconnecting')}</Alert>}
        {
          // The chat is the page: the transcript is as long as it is and the
          // page scrolls. The card is the chat's own ground over the page's
          // pattern, and the message box is a box of its own underneath it --
          // two blocks one after the other, which can never cover each other.
          // A short chat still fills the window -- the card takes what the
          // box leaves -- so the box is at the window's foot from the first
          // question to the last. See the effect that measures `frame`.
          <div ref={frame} style={{ display: 'flex', flexDirection: 'column' }}>
          <Card withBorder padding="md" radius="md" style={{ background: 'var(--mantine-color-body)', flex: '1 0 auto' }}>
              <div
                role="log"
                aria-label={t('agent.chat')}
                aria-live="polite"
              >
                {session && (
                  <Conversation
                    events={reveal.events}
                    files={session.files}
                    canAnswer={myTurn && !action.pending}
                    // The assistant has stopped without a question of its own:
                    // what can be done with the character is what is offered,
                    // so no conversation ends on nothing to press.
                    actions={!active && !finished ? actions : null}
                    onAnswer={(answer) => void send(answer)}
                    active={active}
                    packNames={packNames}
                    packs={offered.map((pack) => pack.label)}
                    canChoose={opening && !action.pending && !choose.pending}
                    onRules={(label) => void chooseRules(label)}
                    stalled={
                      stalled ? (
                        // Said in words, and before the buttons: a turn that
                        // stopped short used to leave only something to press.
                        <Bubble>
                          <Stack gap="xs" align="flex-start">
                            <MessageText text={session.status === 'failed' ? t('agent.failed') : t('agent.paused')} />
                            <Button variant="default" disabled={action.pending} onClick={() => void control(session.status === 'failed' ? 'retry' : 'resume')}>
                              {session.status === 'failed' ? t('agent.retry') : t('agent.resume')}
                            </Button>
                          </Stack>
                        </Bubble>
                      ) : null
                    }
                  />
                )}
                {choose.error && <Alert color="red">{choose.error}</Alert>}
              </div>
          </Card>
              {!finished && (
                <AgentComposer
                  composer={composer}
                  ready={!!session}
                  opening={opening}
                  myTurn={myTurn}
                  running={session?.status === 'running' || session?.status === 'queued'}
                  pending={action.pending}
                  choosing={choose.pending}
                  message={message}
                  setMessage={setMessage}
                  files={files}
                  setFiles={setFiles}
                  onSend={() => void send()}
                  onStop={() => void control('stop')}
                />
              )}
                {/* The end the page follows. Its margin reaches past the foot of
                    the page, so following stops at the page's own end, with
                    the message box whole and its gap under it, rather than
                    with the box's last edge on the window's. */}
                <div ref={end} style={{ scrollMarginBottom: 200 }} />
                {/* Back to the end of the conversation, for a reader who
                    scrolled up and has been left behind by it. It rides the
                    foot of the window and takes up no room of its own. */}
                {away && (
                  <div style={{ position: 'sticky', bottom: 16, height: 0, display: 'flex', justifyContent: 'center' }}>
                    <ActionIcon
                      variant="default"
                      radius="xl"
                      size="lg"
                      aria-label={t('agent.scrollDown')}
                      style={{ transform: 'translateY(-100%)' }}
                      onClick={toEnd}
                    >
                      <IconArrowDown size={18} />
                    </ActionIcon>
                  </div>
                )}
          </div>
        }
      </Stack>
    </Page>
  )
}
