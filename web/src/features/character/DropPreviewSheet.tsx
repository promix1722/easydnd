import { promptLabel } from '@/domain'
import type { Translate } from '@/lib/i18n'
import { useT } from '@/lib/i18n'
import { Badge, Button, Group, ModalSheet, Stack, Text } from '@/ui'

import type { Preview } from './buildModel'
import { eventLabel } from './labels'
import { settledPickName } from './settled'

/** How a drop reason reads, without borrowing a category's word. */
const REASONS = {
  'not-offered': 'build.reason.notOffered',
  'answers-dropped': 'build.reason.answersDropped',
  empty: 'build.reason.empty',
} as const

/** Why an entry was dropped, or the server's own word if this client is behind. */
function reasonLabel(t: Translate, reason: string): string {
  const key = REASONS[reason as keyof typeof REASONS]
  return key === undefined ? reason : t(key)
}

/**
 * What a change would cost, asked before it is paid for.
 *
 * Only ever open because something would be lost. A change that costs
 * nothing else is simply made -- see `price` -- so this dialog means one
 * thing and never cries wolf.
 */
export function DropPreviewSheet({ preview, pending, onCommit, onCancel }: {
  preview: Preview | null
  pending: boolean
  onCommit: () => void
  onCancel: () => void
}) {
  const t = useT()
  return (
    <ModalSheet
      opened={preview !== null}
      onClose={() => onCancel()}
      title={preview?.event === null ? t('build.askAgainTitle') : t('build.changeTitle')}
    >
      {preview !== null && (
        <Stack gap="md">
          <Text size="sm">
            {preview.event === null
              ? t('build.dropWarningAskAgain', { count: preview.dropped.length })
              : t('build.dropWarningChange', { count: preview.dropped.length })}
          </Text>

          {preview.dropped.length > 0 && (
            <Stack gap="xs">
              {preview.dropped.map((entry) => (
                <div key={entry.seq}>
                  <Group gap={6}>
                    <Text size="sm" fw={500}>
                      {eventLabel(t, entry.type)}
                      {entry.ref !== undefined && `: ${preview.names.get(entry.ref) ?? entry.ref}`}
                    </Text>
                    <Badge size="xs" variant="light" color="gray">
                      {reasonLabel(t, entry.reason)}
                    </Badge>
                  </Group>
                  {(entry.lost ?? []).map((lost) => (
                    <Text key={lost.prompt} size="xs" c="dimmed">
                      {promptLabel(lost.prompt)}
                      {lost.picks !== undefined &&
                        `: ${lost.picks
                          .map((pick) => settledPickName(t, pick, preview.names))
                          .join(', ')}`}
                    </Text>
                  ))}
                </div>
              ))}
              <Text size="xs" c="dimmed">
                {t('build.dropReassurance')}
              </Text>
            </Stack>
          )}

          <Group>
            <Button
              color="red"
              loading={pending}
              onClick={() => onCommit()}
            >
              {preview.event === null ? t('build.askAgain') : t('answer.changeIt')}
            </Button>
            <Button variant="subtle" onClick={() => onCancel()}>
              {t('common.cancel')}
            </Button>
          </Group>
        </Stack>
      )}
      </ModalSheet>
  )
}
