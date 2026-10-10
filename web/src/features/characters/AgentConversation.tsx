import { memo } from 'react'
import type { ReactNode } from 'react'

import type { AgentEvent, AgentSession } from '@/lib/api/agent'
import { useT } from '@/lib/i18n'
import { Badge, Button, Group, IconPaperclip, Markdown, Stack, Text } from '@/ui'

import { progressEntry } from './agentProgress'

/** A message's text. Memoized because the transcript is rebuilt on every
 * streamed chunk, and parsing every earlier message's Markdown again each time
 * is what a long import spends the main thread on. */
export const MessageText = memo(function MessageText({ text }: { text: string }) {
  return <Markdown>{text}</Markdown>
})

/** One side of the conversation: the assistant on the left, the player on
 * the right. */
export function Bubble({ mine = false, children }: { mine?: boolean; children: ReactNode }) {
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

/** The transcript: every recorded event as the bubble, notice or answers it is shown as. */
export function Conversation({
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
      const said = event.data as { packs?: { id: string; version: string }[]; assumed?: boolean } | undefined
      const chosen = said?.packs ?? []
      if (said?.assumed) {
        // The first message came before any answer, so the rules are the
        // assistant's decision, and it says so as one.
        rows.push({ key: event.id, kind: 'assistant', text: t('agent.rulesAssumed', { rules: packNames(chosen) }) })
        continue
      }
      // The player's answer, and the assistant's next question after it.
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
