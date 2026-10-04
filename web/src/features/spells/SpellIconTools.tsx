import { useState } from 'react'

import { ApiError, describeError } from '@/lib/api'
import type { SpellSearch } from '@/lib/api'
import type { IconGenerationJob, IconGenerationRequest } from '@/lib/api/spellIcons'
import { useT } from '@/lib/i18n'
import type { SpellIconGeneration } from './useSpellIconGeneration'
import {
  ACTION_ICON_SIZE,
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Group,
  IconWand,
  Loader,
  ModalSheet,
  Stack,
  Text,
  Tooltip,
} from '@/ui'

/**
 * What one confirmation dialog is about to charge for: a single row's icon,
 * or the whole filtered set (slug undefined). `scope` and `search` are passed
 * through to the POST untouched -- the server allowlists the scope and pages
 * the search itself, so "the current filters" is exactly what the screen
 * already asks the catalogue for, minus its own pagination.
 */
export interface SpellIconTarget {
  name?: string
  total?: number
  request: IconGenerationRequest
}

/**
 * One job's mark on its row: spinner while it works, a word when it is done.
 *
 * 'done' draws nothing -- the icon itself arriving is the answer. 'skipped'
 * says so because a batch that never touches existing icons would otherwise
 * leave the row looking like it was never asked for.
 */
export function SpellIconJobBadge({ job }: { job: IconGenerationJob }) {
  const t = useT()
  if (job.state === 'done') return null
  if (job.state === 'skipped') {
    return (
      <Badge size="xs" variant="light" color="gray">
        {t('spells.icons.skipped')}
      </Badge>
    )
  }
  if (job.state === 'failed') {
    // Reasons are slugs, so the failure rides the same error.* lookup the API
    // envelope uses rather than a second table; the code only steers the
    // fallback for a reason the catalogue has not learned yet.
    const detail = describeError(t, new ApiError(500, {
      code: 'server_error',
      ...(job.reason === undefined ? {} : { reason: job.reason }),
    }))
    return (
      <Tooltip label={detail}>
        <Badge size="xs" color="red">
          {t('spells.icons.failed')}
        </Badge>
      </Tooltip>
    )
  }
  return (
    <Box component="span" style={{ display: 'inline-flex', alignItems: 'center' }} title={t('spells.icons.working')}>
      <Loader size={ACTION_ICON_SIZE} />
    </Box>
  )
}

/**
 * The dev server's icon generator, drawn into the spells screen.
 *
 * A separate module rather than folded into SpellsScreen so that the one
 * `import.meta.env.DEV` guard at its call site lets a production bundle drop
 * every control here -- the same pattern as features/characters/StubButton.
 *
 * Every control opens the confirmation dialog rather than charging at once,
 * including when the generator is not configured: the dialog is where the
 * setup instruction lives, so a missing image_generation.api_key looks like
 * a thing to fix rather than a button that never appears.
 */
export function SpellIconTools({
  generation,
  total,
  bulk,
  available,
  target,
  onTarget,
  onCloseTarget,
}: {
  generation: SpellIconGeneration
  /** The filtered result count, so the bulk dialog can say what it will draw. */
  total: number
  /** The scope and current filters for a batch run. */
  bulk: { scope: string; search: SpellSearch }
  available: boolean
  /** The row's dialog target; the toolbar sets it to the batch target. */
  target: SpellIconTarget | null
  onTarget: (target: SpellIconTarget) => void
  onCloseTarget: () => void
}) {
  const t = useT()
  const { state, error, submitting, refresh, generate } = generation
  const [replaceExisting, setReplaceExisting] = useState(false)

  const configured = state?.configured === true
  const busy = submitting || state?.running === true

  // The checkbox defaults off per the paid-operation contract, so it cannot
  // carry over from the last dialog -- opening a fresh target resets it.
  function openTarget(next: SpellIconTarget) {
    setReplaceExisting(false)
    onTarget(next)
  }

  async function submit() {
    if (target === null || !configured || busy) return
    // A single row is always asked for deliberately, so replace: true -- the
    // dialog already said the existing icon goes away, and the one caller who
    // clicked "Generate" on art that exists meant it.
    const replace = target.request.slug === undefined ? replaceExisting : true
    if (await generate({ ...target.request, replace })) onCloseTarget()
  }

  return (
    <>
      <Stack gap="xs">
        <Group gap="sm">
          <Button
            variant="light"
            leftSection={<IconWand size={ACTION_ICON_SIZE} />}
            disabled={!available}
            onClick={() => openTarget({ total, request: { scope: bulk.scope, search: bulk.search, replace: false } })}
          >
            {t('spells.icons.generateBatch')}
          </Button>
          {state !== null && (state.running || state.total > 0) && (
            <Group gap="xs">
              {state.running && <Loader size={ACTION_ICON_SIZE} />}
              <Text size="sm" c="dimmed">
                {t('spells.icons.progress', {
                  completed: state.completed,
                  total: state.total,
                  skipped: state.skipped,
                  failed: state.failed,
                })}
              </Text>
            </Group>
          )}
        </Group>

        {/* A poll that stopped answering is worth a button, not silence: the
            queue itself is unaffected, so the only thing broken is this tab's
            view of it and asking again fixes it. */}
        {error !== null && target === null && (
          <Alert color="orange" title={t('spells.icons.statusUnavailable')}>
            <Group justify="space-between" wrap="nowrap">
              <Text size="sm">{error}</Text>
              <Button variant="light" onClick={refresh}>
                {t('spells.icons.checkAgain')}
              </Button>
            </Group>
          </Alert>
        )}
      </Stack>

      <ModalSheet
        opened={target !== null}
        onClose={onCloseTarget}
        title={t('spells.icons.title')}
        onSubmit={() => void submit()}
      >
        <Stack gap="sm">
          <Text size="sm">
            {target?.request.slug === undefined
              ? t('spells.icons.confirmBatch', { count: target?.total ?? total })
              : t('spells.icons.confirmSingle', { name: target?.name ?? '' })}
          </Text>
          {target?.request.slug === undefined && (
            <Checkbox
              label={t('spells.icons.replaceExisting')}
              checked={replaceExisting}
              onChange={(event) => setReplaceExisting(event.currentTarget.checked)}
            />
          )}
          {state !== null && !state.configured && (
            <Alert color="orange" title={t('spells.icons.notConfigured')}>
              <Text size="sm">{t('error.icon_generation_not_configured')}</Text>
            </Alert>
          )}
          {state?.running === true && (
            <Alert color="blue">
              <Text size="sm">{t('error.icon_generation_busy')}</Text>
            </Alert>
          )}
          {error !== null && (
            <Alert color="red">
              <Text size="sm">{error}</Text>
            </Alert>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={onCloseTarget}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              disabled={!configured || busy}
              loading={submitting}
            >
              {t('spells.icons.generate')}
            </Button>
          </Group>
        </Stack>
      </ModalSheet>
    </>
  )
}
