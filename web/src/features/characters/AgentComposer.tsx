import type { Dispatch, RefObject, SetStateAction } from 'react'

import { useT } from '@/lib/i18n'
import { Button, FileButton, Group, IconPaperclip, Stack, Textarea } from '@/ui'

/** The message box under the chat: the text, the sheet attached to a first message, and Send or Stop. */
export function AgentComposer({
  composer,
  ready,
  opening,
  myTurn,
  running,
  pending,
  choosing,
  message,
  setMessage,
  files,
  setFiles,
  onSend,
  onStop,
}: {
  composer: RefObject<HTMLTextAreaElement | null>
  /** There is a chat to write in. */
  ready: boolean
  opening: boolean
  myTurn: boolean
  /** A turn is in flight, so Send's place is Stop's. */
  running: boolean
  pending: boolean
  choosing: boolean
  message: string
  setMessage: Dispatch<SetStateAction<string>>
  files: File[]
  setFiles: Dispatch<SetStateAction<File[]>>
  onSend: () => void
  onStop: () => void
}) {
  const t = useT()
  return (
    <form
      style={{
        marginTop: 'var(--mantine-spacing-md)',
        background: 'var(--mantine-color-body)',
        borderRadius: 18,
      }}
      onSubmit={(event) => {
        event.preventDefault()
        onSend()
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
          // assistant's turn and its first question included:
          // only sending waits.
          disabled={!ready}
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
              onSend()
            }
          }}
        />
        <Group align="center" justify="space-between">
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
                <Button
                  {...props}
                  variant="transparent"
                  size="compact-sm"
                  px={0}
                  disabled={!myTurn}
                  leftSection={<IconPaperclip size={16} />}
                  // A line of text with a clip beside it, flush
                  // with the words above. Waiting dims it and no
                  // more: the grey slab a disabled button is given
                  // has no padding to sit in here, and read as a
                  // button drawn wrong.
                  style={{ background: 'transparent' }}
                >
                  {t('agent.start.attach')}
                </Button>
              )}
            </FileButton>
          )}

          {/* While a turn is in flight Send has nothing to do, so its
              place is where the turn is stopped. It ends paused,
              with Resume: nothing imported so far is lost. */}
          {running ? (
            <Button type="button" variant="default" loading={pending} onClick={onStop}>
              {t('agent.stop')}
            </Button>
          ) : (
            <Button
              type="submit"
              loading={pending}
              disabled={!myTurn || choosing || (!message.trim() && !files.length)}
            >
              {t('agent.send')}
            </Button>
          )}
        </Group>
      </Stack>
    </form>
  )
}
