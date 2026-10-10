import { useState } from 'react'

import type { GameEntry } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Alert, Button, Group, ModalSheet, NumberInput, Stack } from '@/ui'

/**
 * One number, on its own: where an entry acts in the round. Emptied, it is
 * cleared -- the entry has not rolled yet.
 */
export function InitiativeSheet({ entry, pending, error, onClose, onSave }: {
  entry: GameEntry; pending: boolean; error: string | null; onClose: () => void; onSave: (initiative: number | null) => Promise<void>
}) {
  const t = useT()
  const [initiative, setInitiative] = useState<number | string>(entry.initiative ?? '')
  const valid = initiative === '' || (typeof initiative === 'number' && Number.isInteger(initiative))
  return <ModalSheet opened onClose={onClose} size="xs" title={entry.name || t('common.unnamed')}
    onSubmit={() => { if (valid && entry.can_edit && !pending) void onSave(initiative === '' ? null : Number(initiative)) }}>
    <Stack gap="sm">
      {error && <Alert color="red">{error}</Alert>}
      {!entry.can_edit && <Alert color="yellow">{t('game.editLocked')}</Alert>}
      <NumberInput label={t('vitals.initiative')} allowDecimal={false} value={initiative} onChange={setInitiative} disabled={!entry.can_edit} data-autofocus />
      <Group justify="flex-end">
        <Button type="submit" loading={pending} disabled={!valid || !entry.can_edit}>{t('game.apply')}</Button>
      </Group>
    </Stack>
  </ModalSheet>
}
