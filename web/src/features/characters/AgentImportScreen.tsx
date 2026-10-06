import { memo, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useNavigate, useSearchParams, useParams } from 'react-router'
import {
  chooseAgentRules,
  controlAgent,
  getAgentSession,
  listAgentSessions,
  openAgentSession,
  pollAgentSession,
  polling,
  startAgentSession,
} from '@/lib/api/agent'
import type { AgentEvent, AgentView, AgentSession } from '@/lib/api/agent'
import { ApiError } from '@/lib/api/errors'
import { useAction } from '@/lib/useAction'
import { useReveal } from './useReveal'
import { useT } from '@/lib/i18n'
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  FileButton,
  IconArrowDown,
  IconPaperclip,
  Group,
  Markdown,
  Page,
  Stack,
  Text,
  Textarea,
} from '@/ui'
import { listPacks, resolvePacks } from '@/lib/api/packs'
import { useResource } from '@/lib/useResource'
import { progressEntry } from './agentProgress'

/** Stream events are rendered from recorded state, so reconnect/reload never
 * creates a second conversation or restarts an import. The chat writes to a
 * real character from its first message: View and Edit are that character's
 * own sheet and builder, not pages of this one.
 *
 * It is a conversation the assistant leads. It asks -- which rules, a sheet or
 * a description, then whatever the build leaves open -- and the player
 * answers; so there is something to type into only when it is the player's
 * turn, and nothing above the transcript but the transcript. */
// What the tab shows after an answer arrives. A whole session replaces the one
// held unless it is older -- a slow read must not undo a newer write -- and a
// tail of events is appended past the ones already here. Event ids are 1..n.
function merge(
  old: AgentView | null,
  next: AgentView | { events: AgentEvent[] },
): AgentView | null {
  if ('session' in next)
    return old &&
      old.session.id === next.session.id &&
      (old.session.revision > next.session.revision ||
        (old.session.revision === next.session.revision &&
          old.session.events.length > next.session.events.length))
      ? old
      : next
  if (!old) return old
  const added = next.events.filter((e) => e.id > old.session.events.length)
  return added.length
    ? { session: { ...old.session, events: [...old.session.events, ...added] } }
    : old
}

const EMPTY: AgentEvent[] = []

export function AgentImportScreen() {
  const t = useT()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const { sessionId } = useParams()
  // The chat left unfinished, found when the wizard is opened without one
  // named. Held here rather than navigated to: the page is the wizard either
  // way, and there is no second step that can fail to happen.
  const [resumed, setResumed] = useState<string | null>(null)
  const id = sessionId ?? params.get('session') ?? resumed
  const folder = params.get('folder') ?? undefined
  const [view, setView] = useState<AgentView | null>(null)
  const [files, setFiles] = useState<File[]>([])
  const [message, setMessage] = useState('')
  const packs = useResource('pack-selection', listPacks)
  // Choosing the rules is answering a question like any other: one button per
  // pack. What a pack depends on comes with it.
  const choose = useAction(resolvePacks)
  const [connected, setConnected] = useState(true)
  const boot = useRef<{ folder: string | undefined; found: Promise<string | undefined> }>(undefined)
  const composer = useRef<HTMLTextAreaElement>(null)
  const end = useRef<HTMLDivElement>(null)
  const follow = useRef(true)
  // What the poll loop holds, which is the view one render early: its cursor
  // has to move with an answer, or the next request would ask for the same
  // thing again before React has drawn it.
  const latest = useRef(view)
  latest.current = view
  const action = useAction(async (work: () => Promise<AgentView>) => {
    const result = await work()
    latest.current = merge(latest.current, result)
    setView((old) => merge(old, result))
    return result
  })
  useEffect(() => {
    setView((old) => (old?.session.id === id ? old : null))
    if (!id) {
      // The wizard opens on the chat that was left unfinished, the latest if
      // there are several; a finished one is reached from its character. With
      // none, it opens a new one -- so there is a session, and a first event
      // to draw, before anything is asked.
      //
      // Asked once per arrival, however often this effect runs: a second run
      // that listed before the first had opened would open a second chat.
      let gone = false
      if (boot.current?.folder !== folder || !boot.current)
        boot.current = {
          folder,
          found: listAgentSessions().then((s) => {
            const last = s
              .filter((v) => !v.finished && (!folder || v.folder === folder))
              .sort((a, b) => (b.created ?? '').localeCompare(a.created ?? ''))[0]
            return last?.id ?? action.run(() => openAgentSession(folder)).then((made) => made?.session.id)
          }),
        }
      void boot.current.found
        .then((found) => {
          if (found && !gone) setResumed(found)
        })
        .catch(() => {})
      return () => {
        gone = true
      }
    }
    // One poll a second, each answered at once with what this tab lacks.
    // Reads never disable the message composer. See docs/polling.md.
    const stop = new AbortController()
    const pause = (ms: number) => new Promise((done) => window.setTimeout(done, ms))
    const visible = () =>
      new Promise<void>((done) => {
        if (!document.hidden) return done()
        const shown = () => {
          if (document.hidden && !stop.signal.aborted) return
          document.removeEventListener('visibilitychange', shown)
          stop.signal.removeEventListener('abort', shown)
          done()
        }
        document.addEventListener('visibilitychange', shown)
        stop.signal.addEventListener('abort', shown)
      })
    void (async () => {
      await action.run(() => getAgentSession(id)).catch(() => {})
      while (!stop.signal.aborted) {
        await visible()
        const held = latest.current?.session.id === id ? latest.current.session : undefined
        // A finished chat is a record: nothing will ever be added to it.
        if (held?.finished) return
        try {
          const next = held
            ? await pollAgentSession(id, held.revision, held.events.length, stop.signal)
            : await getAgentSession(id)
          if (stop.signal.aborted) return
          setConnected(true)
          if (next) {
            latest.current = merge(latest.current, next)
            setView((old) => merge(old, next))
          }
          if (held) await pause(polling.every)
        } catch (cause) {
          if (stop.signal.aborted) return
          // Discarded, or never this account's: nothing left to wait for.
          if (cause instanceof ApiError && cause.status === 404) return
          setConnected(false)
          await pause(1000)
        }
      }
    })()
    return () => stop.abort()
    // action.run is stable; switching session is the only subscription boundary.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, folder])
  const session = view?.session
  // The transcript as it is shown: one bubble at a time, the assistant's words
  // typed out. Until all of it is on screen the turn is still the assistant's.
  const reveal = useReveal(session?.id, session?.events ?? EMPTY)
  // The page follows the conversation for as long as the reader is at the
  // end of it, and stops the moment they scroll up to read something.
  //
  // Only scrolling *up* stops it. Being far from the end does not: a bubble
  // taller than any threshold arrives in one step, and the smooth scroll that
  // follows it reports every position on the way down -- taking those for the
  // reader's would stop following exactly when there was most to follow.
  const [away, setAway] = useState(false)
  useEffect(() => {
    let last = window.scrollY
    const scrolled = () => {
      const page = document.documentElement
      const y = window.scrollY
      if (page.scrollHeight - y - window.innerHeight < 80) follow.current = true
      else if (y < last) follow.current = false
      last = y
      setAway(!follow.current)
    }
    window.addEventListener('scroll', scrolled, { passive: true })
    return () => window.removeEventListener('scroll', scrolled)
  }, [])
  // Text is laid out after it is rendered -- Markdown, fonts, a wrapping
  // line -- so the end moves without anything React knows of having changed.
  // Watching the page's height is what keeps it at the end regardless, from
  // the moment it is opened -- the page's and not the chat's, because a notice
  // appearing above the chat moves the end just as far as a message does.
  useEffect(() => {
    const grown = new ResizeObserver(() => {
      if (follow.current) end.current?.scrollIntoView?.({ block: 'end' })
    })
    grown.observe(document.body)
    return () => grown.disconnect()
  }, [])
  const typedTo = reveal.events.at(-1)?.text?.length
  useEffect(() => {
    if (follow.current) end.current?.scrollIntoView?.({ behavior: 'smooth', block: 'end' })
  }, [reveal.events.length, typedTo, reveal.settled, session?.status])
  const active = session?.status === 'running' || session?.status === 'queued' || !reveal.settled
  // The player's turn: the assistant has asked or finished, or the opening has
  // reached "tell me about the character". Only then is there a composer.
  const stalled = reveal.settled && (session?.status === 'paused' || session?.status === 'failed')
  // A finished chat is a record: nothing more is said in it.
  const finished = session?.finished === true
  // An opened chat is waiting for its rules, and then for the first message.
  const opening = session?.status === 'opening'
  const ruled = session?.events.some((event) => event.kind === 'rules') === true
  const myTurn = !!session && !active && !finished && (!opening || ruled)
  useEffect(() => {
    // Without scrolling: a focus brings its field into view by the shortest
    // way, which stops the smooth scroll to the end half way and leaves the
    // foot of the message box under the window's edge.
    if (myTurn) composer.current?.focus({ preventScroll: true })
  }, [myTurn])
  async function send(answer = message) {
    if (!session || !myTurn || action.pending || (!answer.trim() && !files.length)) return
    follow.current = true
    const result = await action.run(() =>
      // A sheet is attached to the first message and to no other.
      opening
        ? startAgentSession(session.id, session.revision, files, answer)
        : controlAgent(session.id, session.revision, 'message', answer),
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
  const offered = (packs.data?.packs ?? [])
    .filter((pack) => !pack.archived && pack.releases.length > 0)
    .map((pack) => ({ label: `${pack.title} v${pack.releases.at(-1)!.version}`, release: pack.releases.at(-1)! }))
  async function chooseRules(label: string) {
    const release = offered.find((pack) => pack.label === label)?.release
    if (!session || !release) return
    const lock = await choose.run([release])
    if (lock) await action.run(() => chooseAgentRules(session.id, session.revision, lock))
  }
  async function control(kind: string) {
    if (!session) return
    const result = await action.run(() => controlAgent(session.id, session.revision, kind))
    if (result && kind === 'discard') void navigate('/characters')
  }
  const trail = id ? [{ label: id }] : []
  // Finish is View under the name the end of a conversation has: the
  // character is already real, so there is nothing to save, only somewhere to go.
  const open = (kind: 'view' | 'edit' | 'finish') => {
    if (!session?.characterId) return
    // Finish is what closes the chat: until it is pressed the wizard comes
    // back here, however done the assistant thinks the character is.
    if (kind === 'finish') void controlAgent(session.id, session.revision, 'finish').catch(() => {})
    void navigate(`/characters/${session.characterId}${kind === 'edit' ? '/build' : ''}`)
  }
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
                  boot.current = undefined
                  setResumed(null)
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
          // A short chat still fills the window, so the box starts at its foot.
          <div>
          <Card withBorder padding="md" radius="md" style={{ background: 'var(--mantine-color-body)' }}>
              <div
                role="log"
                aria-label={t('agent.chat')}
                aria-live="polite"
                style={{ minHeight: 'calc(100dvh - 360px)' }}
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
                <form
                  style={{
                    marginTop: 'var(--mantine-spacing-md)',
                    background: 'var(--mantine-color-body)',
                    borderRadius: 18,
                  }}
                  onSubmit={(event) => {
                    event.preventDefault()
                    void send()
                  }}
                >
                  <Stack
                    gap="xs"
                    p="sm"
                    style={{
                      border: '1px solid var(--mantine-color-default-border)',
                      borderRadius: 18,
                    }}
                  >
                    <Textarea
                      variant="unstyled"
                      // Waiting is said by the cursor and the placeholder, not
                      // by a grey slab inside the box.
                      styles={{ input: { background: 'transparent', opacity: 1 } }}
                      ref={composer}
                      // Always there and always open for writing, the
                      // assistant's turn included: only sending waits.
                      disabled={!session || (opening && !ruled)}
                      aria-label={opening ? t('agent.instructions') : t('agent.message')}
                      placeholder={t('agent.messagePlaceholder')}
                      value={message}
                      onChange={(event) => setMessage(event.currentTarget.value)}
                      autosize
                      minRows={2}
                      maxRows={6}
                      maxLength={16000}
                      onKeyDown={(event) => {
                        if (
                          event.key === 'Enter' &&
                          !event.shiftKey &&
                          !event.nativeEvent.isComposing
                        ) {
                          event.preventDefault()
                          void send()
                        }
                      }}
                    />
                    <Group align="end" justify="space-between">
                      {files.length ? (
                        <Group gap="xs">
                          {files.map((file, index) => (
                            <Button
                              key={`${file.name}-${index}`}
                              variant="light"
                              size="compact-sm"
                              aria-label={t('agent.removeFile', { name: file.name })}
                              onClick={() => setFiles((old) => old.filter((_, i) => i !== index))}
                            >
                              {file.name} ×
                            </Button>
                          ))}
                        </Group>
                      ) : !opening ? (
                        <span />
                      ) : (
                        // A sheet goes with the first message and no other, so
                        // the way to attach one lives in the first message's
                        // box: a quiet line of its own, replaced by the file.
                        <FileButton
                          multiple
                          accept=".pdf,.png,.jpg,.jpeg,.webp,.json,.txt"
                          disabled={!myTurn}
                          onChange={(incoming) => setFiles(incoming.slice(0, 8))}
                        >
                          {(props) => (
                            <Button {...props} variant="transparent" size="compact-sm" px={0} disabled={!myTurn} leftSection={<IconPaperclip size={16} />}>
                              {t('agent.start.attach')}
                            </Button>
                          )}
                        </FileButton>
                      )}

                      <Button
                        type="submit"
                        loading={action.pending}
                        disabled={!myTurn || (!message.trim() && !files.length)}
                      >
                        {t('agent.send')}
                      </Button>
                    </Group>
                  </Stack>
                </form>
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
                      onClick={() => {
                        follow.current = true
                        setAway(false)
                        end.current?.scrollIntoView?.({ behavior: 'smooth', block: 'end' })
                      }}
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

/** A message's text. Memoized because the transcript is rebuilt on every
 * streamed chunk, and parsing every earlier message's Markdown again each time
 * is what a long import spends the main thread on. */
const MessageText = memo(function MessageText({ text }: { text: string }) {
  return <Markdown>{text}</Markdown>
})

/** One side of the conversation: the assistant on the left, the player on
 * the right. */
function Bubble({ mine = false, children }: { mine?: boolean; children: ReactNode }) {
  return (
    <div
      className="chat-bubble"
      style={{
        alignSelf: mine ? 'flex-end' : 'flex-start',
        // The assistant's messages are one column: the same width whatever
        // they hold, so a short line and a long one start and end together.
        ...(mine ? { maxWidth: '85%' } : { width: '85%' }),
        overflowWrap: 'anywhere',
        padding: '10px 14px',
        borderRadius: 16,
        ...(mine ? { borderBottomRightRadius: 4 } : { borderBottomLeftRadius: 4 }),
        background: mine
          ? 'var(--mantine-primary-color-light)'
          : 'var(--mantine-color-default-hover)',
      }}
    >
      {children}
    </div>
  )
}

function Conversation({
  events,
  files,
  canAnswer,
  onAnswer,
  active,
  packNames,
  packs,
  canChoose,
  onRules,
  stalled,
  actions,
}: {
  events: AgentEvent[]
  files: AgentSession['files']
  canAnswer: boolean
  onAnswer: (answer: string) => void
  active: boolean
  packNames: (releases: readonly { id: string; version: string }[]) => string
  /** The answers to the opening question, and whether one can still be given. */
  packs: string[]
  canChoose: boolean
  onRules: (label: string) => void
  /** What the last message offers once the character is done. */
  actions: ReactNode
  /** What a paused or failed run leaves the player to press. */
  stalled: ReactNode
}) {
  const t = useT()
  type Row = {
    key: number
    kind: string
    text: string
    field?: string
    files?: AgentEvent['files']
    options?: string[] | undefined
    /** The opening question, whose answers are the rule packs. */
    opening?: boolean
  }
  const rows: Row[] = []
  const firstUser = events.find((event) => event.kind === 'user')?.id
  const lastUser = events.findLast((event) => event.kind === 'user')?.id ?? 0
  // The rules may be answered again until the first message; the answer that
  // stands is the last, and it is the one shown.
  const rules = events.findLast((event) => event.kind === 'rules')?.id
  for (const event of events) {
    if (event.kind === 'delta') {
      const previous = rows.at(-1)
      if (previous?.kind === 'delta') previous.text += event.text ?? ''
      else rows.push({ key: event.id, kind: 'delta', text: event.text ?? '' })
    } else if (event.kind === 'response') {
      if (rows.at(-1)?.kind === 'delta') rows.pop()
      if (event.text) rows.push({ key: event.id, kind: 'assistant', text: event.text })
    } else if (event.kind === 'status') {
      if (rows.at(-1)?.kind === 'delta') rows[rows.length - 1]!.kind = 'assistant'
    } else if (event.kind === 'tool') {
      continue
    } else if (event.kind === 'progress') {
      // Every write is a message of its own: what was imported, and its value.
      const entry = progressEntry(t, event.data)
      if (!entry) continue
      rows.push({ key: event.id, kind: 'activity', text: entry.value, field: entry.field })
    } else if (event.kind === 'opening') {
      // The assistant's first words: which rules. Its answers are the packs.
      rows.push({ key: event.id, kind: 'assistant', text: t('agent.lead'), opening: true })
    } else if (event.kind === 'rules') {
      if (event.id !== rules) continue
      // The player's answer, and the assistant's next question after it.
      const chosen = (event.data as { packs?: { id: string; version: string }[] } | undefined)?.packs ?? []
      rows.push({ key: event.id, kind: 'user', text: packNames(chosen) })
      rows.push({ key: -event.id, kind: 'assistant', text: t('agent.askStart') })
    } else if (event.kind === 'attachments') {
      // Older sessions recorded attachment markers separately from the user.
      continue
    } else if (['user', 'assistant', 'question', 'edit'].includes(event.kind)) {
      // A run of builder edits is one notice, however many entries it wrote.
      if (event.kind === 'edit' && rows.at(-1)?.kind === 'edit') continue
      rows.push({
        key: event.id,
        kind: event.kind,
        text: event.text ?? '',
        files:
          event.files ??
          (event.id === firstUser && !events.some((item) => item.files) ? files : undefined),
        options: event.options,
      })
    }
  }
  const lastAssistant = rows.findLast((row) => ['assistant', 'question'].includes(row.kind))
  return (
    <Stack gap="md">
      {rows.map((row) =>
        row.kind === 'activity' ? (
          // What the assistant wrote, in full and never folded: the field as
          // a caption, what it now holds underneath.
          <Bubble key={row.key}>
            <Text size="sm" fw={700}>
              {t('agent.progress.imported', { field: row.field! })}
            </Text>
            <Text size="sm" style={{ whiteSpace: 'pre-line' }}>
              {row.text}
            </Text>
          </Bubble>
        ) : row.kind === 'edit' ? (
          <Text key={row.key} size="xs" c="dimmed" ta="center">
            {t('agent.edited')}
          </Text>
        ) : (
          <Bubble key={row.key} mine={row.kind === 'user'}>
            <Stack gap="xs">
              {!!row.files?.length && (
                <Group gap="xs">
                  {row.files.map((file, at) => (
                    <Badge
                      key={at}
                      variant="default"
                      leftSection={<IconPaperclip size={12} />}
                      tt="none"
                    >
                      {file.name}
                    </Badge>
                  ))}
                </Group>
              )}
              {row.text ? <MessageText text={row.text} /> : null}
              {/*
                Prepared answers, never an open question. They stay under the
                message that offered them -- pressed or not, nothing in the
                log folds away -- and only the latest can still be pressed.
                The last one opens the text field instead of answering.
              */}
              {(() => {
                const latest = row === lastAssistant && row.key > lastUser
                // Not under the question about a sheet, which is answered by
                // writing, nor under text that is still arriving.
                if (!['assistant', 'question'].includes(row.kind) || row.key < 0) return null
                // Only what the assistant actually offered: a button that
                // says nothing but "go on" is not a choice.
                const offered = row.opening ? packs : (row.options ?? [])
                if (offered.length === 0) return null
                const open = row.opening ? canChoose : latest && canAnswer
                return (
                  <Group gap="xs">
                    {offered.map((option) => (
                      <Button key={option} variant="default" disabled={!open} onClick={() => (row.opening ? onRules(option) : onAnswer(option))}>
                        {option}
                      </Button>
                    ))}
                  </Group>
                )
              })()}
            </Stack>
          </Bubble>
        ),
      )}
      {active && (
        <Bubble>
          <span className="chat-typing" role="status" aria-label={t('agent.thinking')}>
            <span />
            <span />
            <span />
          </span>
        </Bubble>
      )}
      {/* The end of a turn that asked nothing -- finished, stopped short or
          failed -- is the same four buttons. A question's own answers come
          first: they are under the question. */}
      {stalled}
      {actions && !(lastAssistant?.options?.length && lastAssistant.key > lastUser) && <Bubble>{actions}</Bubble>}
    </Stack>
  )
}
