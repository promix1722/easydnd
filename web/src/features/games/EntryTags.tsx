import { useState } from 'react'

import type { GameEntry } from '@/lib/api'
import { patchGameEntry } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import { ACTION_ICON_SIZE, ActionIcon, Alert, Badge, Button, Group, IconPlus, Stack, TextInput } from '@/ui'

/** Tags are game values, edited directly in the row, independent of dialog drafts. */
export function EntryTags({ gameId, entry, onChange }: { gameId: string; entry: GameEntry; onChange: () => void }) {
  const t = useT()
  const save = useAction(patchGameEntry)
  const incoming = JSON.stringify(entry.tags ?? [])
  const [observed, setObserved] = useState(incoming)
  const [tags, setTags] = useState(entry.tags ?? [])
  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState('')
  const [invalid, setInvalid] = useState(false)
  if (observed !== incoming) {
    setObserved(incoming)
    setTags(entry.tags ?? [])
  }
  async function update(next: string[]) {
    if (!entry.can_edit || save.pending) return false
    const result = await save.run(gameId, entry.id, { tags: next })
    if (result === null) return false
    setTags(result.entries.find((item) => item.id === entry.id)?.tags ?? next)
    onChange()
    return true
  }
  async function add() {
    const tag = draft.trim()
    if (!tag || tag.length > 100 || tags.length >= 20) { setInvalid(true); return }
    if (tags.includes(tag) || await update([...tags, tag])) {
      setDraft(''); setAdding(false); setInvalid(false)
    }
  }
  if (tags.length === 0 && !entry.can_edit && !adding && save.error === null) return null
  return <Stack gap="xs">
    {(tags.length > 0 || (entry.can_edit && !adding)) && <Group gap="xs" role="group" aria-label={t('game.tags')}>
      {tags.map((tag) => entry.can_edit ? (
        <Button key={tag} size="xs" variant="light" disabled={save.pending}
          styles={{ root: { height: 'auto', minHeight: 28, maxWidth: '100%', paddingBlock: 4 }, label: { whiteSpace: 'normal', overflowWrap: 'anywhere', fontSize: 14, lineHeight: '20px' } }}
          aria-label={t('game.removeTag', { tag })} onClick={() => void update(tags.filter((value) => value !== tag))}>{tag} ×</Button>
      ) : <Badge key={tag} variant="light" size="lg"
        styles={{ root: { textTransform: 'none', height: 'auto', minHeight: 28, maxWidth: '100%', paddingBlock: 4 }, label: { whiteSpace: 'normal', overflowWrap: 'anywhere', fontSize: 14, lineHeight: '20px' } }}>{tag}</Badge>)}
      {entry.can_edit && !adding && <Button variant="subtle" size="xs" h={28} disabled={save.pending}
        leftSection={<IconPlus size={14} />} onClick={() => { save.reset(); setAdding(true) }}>{t('game.addTag')}</Button>}
    </Group>}
    {adding && <Group gap="xs" align="flex-start" wrap="nowrap">
      <TextInput autoFocus size="sm" aria-label={t('game.tags')} placeholder={t('game.tags')} value={draft}
        style={{ flex: 1, minWidth: 0 }} disabled={!entry.can_edit || save.pending}
        error={invalid ? t('game.tagInvalid') : undefined}
        onChange={(event) => { setDraft(event.currentTarget.value); setInvalid(false) }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') { event.preventDefault(); void add() }
          if (event.key === 'Escape') { setAdding(false); setDraft(''); setInvalid(false) }
        }} />
      <ActionIcon size={36} variant="light" aria-label={t('game.addTag')} disabled={!entry.can_edit || save.pending || !draft.trim()}
        onClick={() => void add()}><IconPlus size={ACTION_ICON_SIZE} /></ActionIcon>
      <Button variant="subtle" size="sm" h={36} disabled={save.pending} onClick={() => { setAdding(false); setDraft(''); setInvalid(false) }}>{t('common.cancel')}</Button>
    </Group>}
    {save.error && <Alert color="red">{save.error}</Alert>}
  </Stack>
}
