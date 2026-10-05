import { memo, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useNavigate, useSearchParams, useParams } from 'react-router'
import {
  controlAgent,
  createAgentSession,
  getAgentSession,
  listAgentSessions,
} from '@/lib/api/agent'
import type { AgentEvent, AgentView, AgentSession } from '@/lib/api/agent'
import { developmentSession } from '@/lib/api/devSession'
import { requestLocale } from '@/lib/api/locale'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import {
  Alert,
  Badge,
  Button,
  Card,
  FileButton,
  IconPaperclip,
  Group,
  Markdown,
  Page,
  Stack,
  Text,
  Textarea,
} from '@/ui'
import { listPacks, resolvePacks } from '@/lib/api/packs'
import type { RulesLock } from '@/lib/api/packs'
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
  // The opening, before there is a session to record it: the rules the player
  // confirmed, and whether they have said how they want to start.
  const packs = useResource('pack-selection', listPacks)
  const [selectedRules, setSelectedRules] = useState<RulesLock>()
  // Choosing the rules is answering a question like any other: one button per
  // pack, pressed once. What a pack depends on comes with it.
  const choose = useAction(resolvePacks)
  const [chosen, setChosen] = useState('')
  const [connected, setConnected] = useState(true)
  const composer = useRef<HTMLTextAreaElement>(null)
  const end = useRef<HTMLDivElement>(null)
  const follow = useRef(true)
  const action = useAction(async (work: () => Promise<AgentView>) => {
    const result = await work()
    setView((old) =>
      old &&
      old.session.id === result.session.id &&
      (old.session.revision > result.session.revision ||
        (old.session.revision === result.session.revision &&
          old.session.events.length > result.session.events.length))
        ? old
        : result,
    )
    return result
  })
  useEffect(() => {
    let live = true
    setView((old) => (old?.session.id === id ? old : null))
    if (!id) {
      void listAgentSessions().then(
        (s) => {
          // The wizard opens on the chat that was left unfinished, the latest
          // if there are several. A finished one is reached from its character.
          const last = s
            .filter((v) => !v.finished && (!folder || v.folder === folder))
            .sort((a, b) => (b.created ?? '').localeCompare(a.created ?? ''))[0]
          if (last) setResumed(last.id)
        },
        () => {},
      )
      return
    }
    // A periodic snapshot also recovers a missed terminal status after a proxy
    // drops or buffers the stream. Reads never disable the message composer.
    const refresh = () =>
      void getAgentSession(id).then(
        (next) => {
          if (!live) return
          setView((old) =>
            !old ||
            old.session.id !== next.session.id ||
            next.session.revision > old.session.revision ||
            (next.session.revision === old.session.revision &&
              next.session.events.length >= old.session.events.length)
              ? next
              : old,
          )
        },
        () => {},
      )
    void action.run(() => getAgentSession(id))
    const poll = window.setInterval(refresh, 3000)
    const scope = developmentSession()
    const stream = new EventSource(
      `/v1/agent-sessions/${encodeURIComponent(id)}/events?locale=${requestLocale()}${scope ? `&devSession=${encodeURIComponent(scope)}` : ''}`,
    )
    stream.onopen = () => setConnected(true)
    stream.onerror = () => setConnected(false)
    stream.addEventListener('snapshot', (event: MessageEvent<string>) => {
      if (!live) return
      const next = JSON.parse(event.data) as AgentView
      setView((old) =>
        old &&
        old.session.id === next.session.id &&
        (old.session.revision > next.session.revision ||
          (old.session.revision === next.session.revision &&
            old.session.events.length > next.session.events.length))
          ? old
          : next,
      )
    })
    stream.addEventListener('update', (event: MessageEvent<string>) => {
      if (!live) return
      const next = JSON.parse(event.data) as AgentEvent
      setView((old) =>
        !old || old.session.events.some((e) => e.id === next.id)
          ? old
          : {
              ...old,
              session: {
                ...old.session,
                events: [...old.session.events, next],
              },
            },
      )
    })
    return () => {
      live = false
      window.clearInterval(poll)
      stream.close()
    }
    // action.run is stable; switching session is the only subscription boundary.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, folder])
  useEffect(() => {
    if (follow.current) end.current?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' })
  }, [view?.session.events.length, view?.session.status])
  const session = view?.session
  const active = session?.status === 'running' || session?.status === 'queued'
  // The player's turn: the assistant has asked or finished, or the opening has
  // reached "tell me about the character". Only then is there a composer.
  const stalled = session?.status === 'paused' || session?.status === 'failed'
  // A finished chat is a record: nothing more is said in it.
  const finished = session?.finished === true
  const myTurn = session ? !active && !finished : selectedRules !== undefined
  useEffect(() => {
    if (myTurn) composer.current?.focus()
  }, [myTurn])
  async function send(answer = message) {
    if (
      active ||
      action.pending ||
      (!answer.trim() && !files.length) ||
      (!session && !selectedRules)
    )
      return
    follow.current = true
    const result = await action.run(() =>
      // A sheet is attached to the first message and to no other.
      session
        ? controlAgent(session.id, session.revision, 'message', answer)
        : createAgentSession(files, answer, folder, selectedRules),
    )
    if (result) {
      setMessage('')
      setFiles([])
      if (!id)
        void navigate(
          `/ai-wizard/${result.session.id}${folder ? `?folder=${encodeURIComponent(folder)}` : ''}`,
          { replace: true },
        )
    }
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
        <Button variant="default" disabled={active || action.pending} onClick={() => void control('discard')}>
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
          // The chat is the page: it takes the height the window has left, and
          // the transcript takes what the composer does not.
          <Card withBorder padding="md" radius="md" style={{ height: 'calc(100dvh - 190px)', minHeight: 360 }}>
            <Stack gap="md" h="100%">
              <div
                role="log"
                aria-label={t('agent.chat')}
                aria-live="polite"
                style={{
                  flex: 1,
                  minHeight: 0,
                  overflowY: 'auto',
                  overflowX: 'hidden',
                  padding: 8,
                }}
                onScroll={(event) => {
                  const el = event.currentTarget
                  follow.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80
                }}
              >
                {session ? (
                  <Conversation
                    events={session.events}
                    files={session.files}
                    canAnswer={myTurn && !action.pending}
                    // The assistant has stopped without a question of its own:
                    // what can be done with the character is what is offered,
                    // so no conversation ends on nothing to press.
                    actions={!active && !finished ? actions : null}
                    onAnswer={(answer) => void send(answer)}
                    active={active}
                    packNames={packNames}
                    stalled={
                      stalled ? (
                        <Bubble>
                          <Button variant="default" disabled={action.pending} onClick={() => void control(session.status === 'failed' ? 'retry' : 'resume')}>
                            {session.status === 'failed' ? t('agent.retry') : t('agent.resume')}
                          </Button>
                        </Bubble>
                      ) : null
                    }
                  />
                ) : (
                  <Stack gap="md">
                    <Bubble>
                      <Stack gap="sm">
                        <Text>{t('agent.lead')}</Text>
                        <Group gap="xs">
                          {(packs.data?.packs ?? [])
                            .filter((pack) => !pack.archived && pack.releases.length > 0)
                            .map((pack) => {
                              const release = pack.releases.at(-1)!
                              return (
                                <Button
                                  key={pack.id}
                                  variant="default"
                                  disabled={selectedRules !== undefined}
                                  loading={choose.pending}
                                  onClick={() => void choose.run([release]).then((lock) => { if (lock) { setSelectedRules(lock); setChosen(`${pack.title} v${release.version}`) } })}
                                >
                                  {pack.title} v{release.version}
                                </Button>
                              )
                            })}
                        </Group>
                        {choose.error && <Alert color="red">{choose.error}</Alert>}
                      </Stack>
                    </Bubble>
                    {selectedRules && (
                      <>
                        <Bubble mine>
                          <Group gap="xs">
                            <Text>{chosen}</Text>
                            <Button variant="subtle" size="compact-xs" onClick={() => { setSelectedRules(undefined); setFiles([]) }}>
                              {t('agent.changeRules')}
                            </Button>
                          </Group>
                        </Bubble>
                        <Bubble>
                          <Text>{t('agent.askStart')}</Text>
                        </Bubble>
                      </>
                    )}
                  </Stack>
                )}
                {/* The reply is written in the conversation, straight under the
                    message it answers, not in a box at the foot of the page. */}
              {!finished && (
                <form
                  style={{ marginTop: 'var(--mantine-spacing-md)' }}
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
                      // Always there, at the end of the conversation; open for
                      // writing when the assistant has asked something.
                      disabled={!myTurn || action.pending}
                      aria-label={session ? t('agent.message') : t('agent.instructions')}
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
                      ) : session ? (
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
                <div ref={end} />
              </div>
            </Stack>
          </Card>
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
  stalled,
  actions,
}: {
  events: AgentEvent[]
  files: AgentSession['files']
  canAnswer: boolean
  onAnswer: (answer: string) => void
  active: boolean
  packNames: (releases: readonly { id: string; version: string }[]) => string
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
  }
  const rows: Row[] = []
  const firstUser = events.find((event) => event.kind === 'user')?.id
  const lastUser = events.findLast((event) => event.kind === 'user')?.id ?? 0
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
    } else if (event.kind === 'rules') {
      // The opening, kept: what the assistant asked before there was a
      // session, and what the player answered.
      const chosen = (event.data as { packs?: { id: string; version: string }[] } | undefined)?.packs ?? []
      rows.push({ key: -3, kind: 'assistant', text: t('agent.lead') })
      rows.push({ key: -2, kind: 'user', text: packNames(chosen) })
      rows.push({ key: -1, kind: 'assistant', text: t('agent.askStart') })
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
                // Not under the opening, which was answered before the session, nor
                // under text that is still arriving.
                if (!['assistant', 'question'].includes(row.kind) || row.key < 0) return null
                // Only what the assistant actually offered: a button that
                // says nothing but "go on" is not a choice.
                const offered = row.options ?? []
                if (offered.length === 0) return null
                return (
                  <Group gap="xs">
                    {offered.map((option) => (
                      <Button key={option} variant="default" disabled={!latest || !canAnswer} onClick={() => onAnswer(option)}>
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
          <Text size="sm" c="dimmed">
            {t('agent.thinking')}
          </Text>
        </Bubble>
      )}
      {/* The end of a turn that asked nothing -- finished, stopped short or
          failed -- is the same four buttons. A question's own answers come
          first: they are under the question. */}
      {actions && !(lastAssistant?.options?.length && lastAssistant.key > lastUser) && <Bubble>{actions}</Bubble>}
      {stalled}
    </Stack>
  )
}
