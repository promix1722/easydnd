import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams, useParams } from 'react-router'
import {
  addAgentFiles,
  controlAgent,
  createAgentSession,
  finalizeAgent,
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
import { PackSelector } from '../packs/PackSelector'
import type { RulesLock } from '@/lib/api/packs'
import { progressText } from './agentProgress'
import { BuildScreen } from '../character/BuildScreen'
import { SheetBody } from '../character/SheetBody'
import { loadCompendium } from '../character/compendium'
import type { Compendium } from '../character/compendium'

const STATUS_LABELS = {
  queued: 'agent.status.queued',
  running: 'agent.status.running',
  waiting: 'agent.status.waiting',
  paused: 'agent.status.paused',
  failed: 'agent.status.failed',
  review: 'agent.status.review',
  saved: 'agent.status.saved',
} as const

/** Stream events are rendered from recorded state, so reconnect/reload never
 * creates a second conversation or restarts an import. Chat, view and editor
 * share one route component so unsent input survives page navigation. */
export function AgentImportScreen() {
  const t = useT()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const { sessionId, importView } = useParams()
  const id = sessionId ?? params.get('session')
  const folder = params.get('folder') ?? undefined
  const [view, setView] = useState<AgentView | null>(null)
  const [files, setFiles] = useState<File[]>([])
  const [message, setMessage] = useState('')
  const [selectedRules, setSelectedRules] = useState<RulesLock>()
  const [rulesDirty, setRulesDirty] = useState(false)
  const subview = importView ?? params.get('view')
  const [connected, setConnected] = useState(true)
  const [drafts, setDrafts] = useState<AgentSession[]>([])
  const [compendium, setCompendium] = useState<Compendium>({
    names: null,
    skills: null,
    proficiencies: null,
  })
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
    void loadCompendium().then(setCompendium)
  }, [])
  useEffect(() => {
    let live = true
    setView((old) => (old?.session.id === id ? old : null))
    if (!id) {
      void listAgentSessions().then(
        (s) => {
          if (live)
            setDrafts(s.filter((v) => v.status !== 'saved' && (!folder || v.folder === folder)))
        },
        () => {},
      )
      return () => {
        live = false
      }
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
  const saved = session?.status === 'saved'
  const currentSessionId = session?.id
  useEffect(() => {
    if (currentSessionId && !active && !saved && !subview) composer.current?.focus()
  }, [active, saved, subview, currentSessionId])
  async function send(answer = message) {
    if (
      active ||
      action.pending ||
      saved ||
      (!answer.trim() && !files.length) ||
      (!session && (!selectedRules || rulesDirty))
    )
      return
    follow.current = true
    const result = await action.run(() =>
      session
        ? files.length
          ? addAgentFiles(session.id, session.revision, files, answer)
          : controlAgent(session.id, session.revision, 'message', answer)
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
  const chatPath = id ? `/ai-wizard/${encodeURIComponent(id)}` : '/ai-wizard'
  const trail = [...(id ? [{ label: id, ...(subview ? { to: chatPath } : {}) }] : [])]
  function openSubview(value: 'character' | 'editor' | null) {
    void navigate(`${chatPath}${value ? `/${value}` : ''}`)
    if (value !== 'editor' && session) void action.run(() => getAgentSession(session.id))
  }
  if (subview && !view && !action.error)
    return (
      <Page
        trail={[
          ...trail,
          {
            label: subview === 'editor' ? t('common.edit') : t('agent.preview'),
          },
        ]}
        state={{ kind: 'loading' }}
      />
    )
  if (subview === 'editor' && view) {
    if (active)
      return (
        <Page trail={[...trail, { label: t('common.edit') }]}>
          <Text>{t('agent.waitForReply')}</Text>
          <Button onClick={() => openSubview(null)}>{t('agent.backToChat')}</Button>
        </Page>
      )
    return (
      <BuildScreen
        key={session?.id}
        draftId={saved && session?.characterId ? session.characterId : `import:${id}`}
        trail={[...trail, { label: t('common.edit') }]}
        onDone={() => openSubview(null)}
      />
    )
  }
  if (subview === 'character' && view)
    return (
      <Page
        trail={[...trail, { label: t('agent.preview') }]}
        actions={
          <Group>
            <Button variant="subtle" onClick={() => openSubview(null)}>
              {t('agent.backToChat')}
            </Button>
            <Button disabled={active} onClick={() => openSubview('editor')}>
              {t('common.edit')}
            </Button>
          </Group>
        }
      >
        <SheetBody sheet={view.sheet} compendium={compendium} />
      </Page>
    )
  return (
    <Page trail={trail}>
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
        {session && (
          <Group justify="space-between">
            <Group gap="xs">
              <Badge>{t(STATUS_LABELS[session.status])}</Badge>
              <Text size="xs" c="dimmed">
                {t('agent.chatId', { id: session.id.slice(0, 8) })}
              </Text>
            </Group>
          </Group>
        )}
        {!id && (
          <Stack gap="xs">
            <Text c="dimmed" size="sm">
              {t('agent.rulesRequired')}
            </Text>
            <PackSelector
              value={selectedRules}
              onChange={setSelectedRules}
              onDirtyChange={setRulesDirty}
            />
          </Stack>
        )}
        {(id || selectedRules) && (
          <Card withBorder padding="md" radius="md">
            <Stack gap="md">
              <div
                role="log"
                aria-label={t('agent.chat')}
                aria-live="polite"
                style={{
                  height: 'calc(100dvh - 320px)',
                  minHeight: 240,
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
                  <>
                    <Conversation
                      events={session.events}
                      files={session.files}
                      canAnswer={!active && !saved && !action.pending}
                      onAnswer={(answer) => void send(answer)}
                      onAction={(kind) => {
                        if (kind === 'save' && session)
                          void action.run(() => finalizeAgent(session.id, session.revision))
                        else if (saved && session.characterId)
                          void navigate(
                            `/characters/${session.characterId}${kind === 'edit' ? '/build' : ''}`,
                          )
                        else openSubview(kind === 'view' ? 'character' : 'editor')
                      }}
                      saved={!!saved}
                      canSave={view?.sheet.identity.name !== '…'}
                      review={session.status === 'review'}
                    />
                    {active && (
                      <Text size="sm" c="dimmed" mt="md">
                        {t('agent.thinking')}
                      </Text>
                    )}
                  </>
                ) : (
                  <Stack>
                    <Text>{t('agent.lead')}</Text>
                    {drafts.map((draft) => (
                      <Button
                        key={draft.id}
                        variant="subtle"
                        onClick={() => void navigate(`/ai-wizard/${draft.id}`)}
                      >
                        {t('agent.resumeDraft')} · {draft.id.slice(0, 8)}
                      </Button>
                    ))}
                  </Stack>
                )}
                <div ref={end} />
              </div>
              {saved ? (
                <Alert>{t('agent.saved')}</Alert>
              ) : (
                <form
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
                    onDragOver={(event) => event.preventDefault()}
                    onDrop={(event) => {
                      event.preventDefault()
                      if (!active && !action.pending)
                        setFiles((old) =>
                          [...old, ...Array.from(event.dataTransfer.files)].slice(0, 8),
                        )
                    }}
                  >
                    {!!files.length && (
                      <Group gap="xs">
                        {files.map((file, index) => (
                          <Button
                            key={`${file.name}-${index}`}
                            variant="light"
                            size="compact-xs"
                            aria-label={t('agent.removeFile', {
                              name: file.name,
                            })}
                            onClick={() => setFiles((old) => old.filter((_, i) => i !== index))}
                          >
                            {file.name} ×
                          </Button>
                        ))}
                      </Group>
                    )}
                    <Textarea
                      variant="unstyled"
                      ref={composer}
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
                      <FileButton
                        multiple
                        accept=".pdf,.png,.jpg,.jpeg,.webp,.json,.txt"
                        disabled={active || action.pending}
                        onChange={(incoming) =>
                          setFiles((old) => [...old, ...incoming].slice(0, 8))
                        }
                      >
                        {(props) => (
                          <Button
                            {...props}
                            variant="subtle"
                            leftSection={<IconPaperclip size={18} />}
                            disabled={active || action.pending}
                          >
                            {t('agent.attach')}
                          </Button>
                        )}
                      </FileButton>
                      <Button
                        type="submit"
                        loading={action.pending}
                        disabled={
                          active || (session ? !message.trim() && !files.length : !files.length)
                        }
                      >
                        {t('agent.send')}
                      </Button>
                    </Group>
                    {active && (
                      <Text size="xs" c="dimmed">
                        {t('agent.waitForReply')}
                      </Text>
                    )}
                    {session && (session.status === 'failed' || session.status === 'paused') && (
                      <Button
                        variant="subtle"
                        disabled={action.pending}
                        onClick={() =>
                          void control(session.status === 'failed' ? 'retry' : 'resume')
                        }
                      >
                        {session.status === 'failed' ? t('agent.retry') : t('agent.resume')}
                      </Button>
                    )}
                  </Stack>
                  {!session && (
                    <Text size="xs" c="dimmed" mt="xs">
                      {t('agent.fileHint')}
                    </Text>
                  )}
                </form>
              )}
            </Stack>
          </Card>
        )}
        <Group justify="space-between">
          <Text size="xs" c="dimmed">
            {t('agent.memory')}
          </Text>
          {session && !saved && (
            <Button
              variant="subtle"
              color="red"
              disabled={active || action.pending}
              onClick={() => void control('discard')}
            >
              {t('agent.discard')}
            </Button>
          )}
        </Group>
      </Stack>
    </Page>
  )
}

function Conversation({
  events,
  files,
  canAnswer,
  onAnswer,
  onAction,
  saved,
  canSave,
  review,
}: {
  events: AgentEvent[]
  files: AgentSession['files']
  canAnswer: boolean
  onAnswer: (answer: string) => void
  onAction: (action: 'view' | 'edit' | 'save') => void
  saved: boolean
  canSave: boolean
  review: boolean
}) {
  const t = useT()
  type Row = {
    key: number
    kind: string
    text: string
    files?: AgentEvent['files']
    options?: string[] | undefined
    actions?: AgentEvent['actions']
  }
  const rows: Row[] = []
  const firstUser = events.find((event) => event.kind === 'user')?.id
  const lastQuestion = events.findLast((event) => event.kind === 'question')?.id
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
      const label = progressText(t, event.data)
      if (!label) continue
      const previous = rows.at(-1)
      if (previous?.kind === 'activity') previous.text += '\n' + label
      else rows.push({ key: event.id, kind: 'activity', text: label })
    } else if (event.kind === 'attachments') {
      // Older sessions recorded attachment markers separately from the user.
      continue
    } else if (['user', 'assistant', 'question', 'edit'].includes(event.kind)) {
      rows.push({
        key: event.id,
        kind: event.kind,
        text: event.text ?? '',
        files:
          event.files ??
          (event.id === firstUser && !events.some((item) => item.files) ? files : undefined),
        options: event.options,
        actions: event.actions,
      })
    }
  }
  const lastAssistant = rows.findLast((row) => ['assistant', 'question'].includes(row.kind))
  if (review && lastAssistant && !lastAssistant.actions)
    lastAssistant.actions = ['view', 'edit', 'save']
  return (
    <Stack gap="md">
      {rows.map((row) => (
        <div
          key={row.key}
          style={{
            alignSelf: row.kind === 'user' ? 'flex-end' : 'stretch',
            maxWidth: row.kind === 'user' ? '85%' : '100%',
            overflowWrap: 'anywhere',
            borderRadius: 16,
            padding: row.kind === 'user' ? '12px 16px' : '4px 0',
            background: row.kind === 'user' ? 'var(--mantine-color-default-hover)' : undefined,
          }}
        >
          {row.kind === 'activity' ? (
            <Text size="sm" c="dimmed" style={{ whiteSpace: 'pre-line' }}>
              {row.text}
            </Text>
          ) : (
            <Stack gap="xs">
              {!!row.files?.length && (
                <Group gap="xs">
                  {row.files.map((file, index) => (
                    <Text key={index} size="sm" fw={500}>
                      {file.name}
                    </Text>
                  ))}
                </Group>
              )}
              {row.kind === 'edit' ? (
                <Text size="sm" c="dimmed">
                  {t('agent.edited')}
                </Text>
              ) : row.text ? (
                <Markdown>{row.text}</Markdown>
              ) : null}
              {!!row.actions?.length && (
                <Group gap="xs">
                  {row.actions
                    .filter((kind) => kind !== 'save' || !saved)
                    .map((kind) => (
                      <Button
                        key={kind}
                        variant="light"
                        disabled={
                          (!canAnswer && !saved && kind !== 'view') || (kind === 'save' && !canSave)
                        }
                        onClick={() => onAction(kind)}
                      >
                        {kind === 'view'
                          ? t('import.viewSheet')
                          : kind === 'edit'
                            ? t('common.edit')
                            : t('agent.save')}
                      </Button>
                    ))}
                </Group>
              )}
              {row.kind === 'question' && row.key === lastQuestion && row.key > lastUser && (
                <>
                  {!!row.options?.length && (
                    <Group gap="xs">
                      {row.options.map((option) => (
                        <Button
                          key={option}
                          variant="light"
                          disabled={!canAnswer}
                          onClick={() => onAnswer(option)}
                        >
                          {option}
                        </Button>
                      ))}
                    </Group>
                  )}
                  <Text size="xs" c="dimmed">
                    {t('agent.answerHint')}
                  </Text>
                </>
              )}
            </Stack>
          )}
        </div>
      ))}
    </Stack>
  )
}
