import { useState } from 'react'
import type { Stage } from '@/domain'
import type { CharacterEvent, Change } from '@/lib/api/characters'
import { replaceEvent } from '@/lib/api/characters'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import { Alert, Button, Checkbox, Group, NumberInput, Stack, Text, Textarea } from '@/ui'
import { formatValue } from './labels'
import { titleCase } from '@/domain'
function fieldLabel(path: string) {
  return path
    .split('.')
    .filter((part) => !['identity', 'finalAbilities', 'abilities', 'base', 'status'].includes(part))
    .map(titleCase)
    .join(' · ')
}
function fieldStage(path: string): Stage {
  if (path.startsWith('finalAbilities.') || path.startsWith('abilities.')) return 'abilities'
  if (path.startsWith('equipment.')) return 'equipment'
  if (
    path.startsWith('skills.') ||
    path.startsWith('savingThrows.') ||
    path.startsWith('proficiencies.')
  )
    return 'class'
  return 'personal'
}
export function ImportedFieldsPanel({
  id,
  stage,
  events,
  revision,
  seq,
  onSaved,
}: {
  id: string
  stage: Stage
  events: readonly CharacterEvent[]
  revision: number
  seq: number
  onSaved: () => void
}) {
  const t = useT()
  const [editing, setEditing] = useState<{ event: CharacterEvent; changes: Change[] } | null>(null)
  const action = useAction(replaceEvent)
  const imported = events.some((event) => event.note?.startsWith('import.session:'))
  const fields = events
    .filter((event) => event.observed || (imported && event.type === 'init'))
    .map((event) => ({
      ...event,
      changes: (event.changes ?? []).filter(
        (ch) =>
          fieldStage(ch.path) === stage &&
          !['identity.name', 'identity.ruleset', 'identity.desiredLevel'].includes(ch.path),
      ),
    }))
    .filter((event) => event.changes?.length)
  if (!fields.length) return null
  return (
    <Stack gap="xs">
      <Text size="sm" fw={600}>
        {t('imported.edit')}
      </Text>
      {fields.map((event) => (
        <Stack key={event.seq} gap="xs">
          <Group justify="space-between">
            <Text size="sm">
              {event.changes
                ?.map((ch) => fieldLabel(ch.path) + ': ' + formatValue(t, ch.value))
                .join(', ')}
            </Text>
            <Button
              variant="subtle"
              onClick={() => {
                action.reset()
                setEditing({ event, changes: structuredClone(event.changes ?? []) })
              }}
            >
              {t('common.edit')}
            </Button>
          </Group>
          {editing && editing.event.seq === event.seq && (
            <form
              onSubmit={async (e) => {
                e.preventDefault()
                const result = await action.run(
                  id,
                  event.seq!,
                  seq,
                  { ...event, changes: editing.changes },
                  false,
                  revision,
                )
                if (result) {
                  setEditing(null)
                  onSaved()
                }
              }}
            >
              <Stack>
                {editing.changes.map((change, index) => {
                  const update = (value: Change['value']) =>
                    setEditing({
                      ...editing,
                      changes: editing.changes.map((ch, i) =>
                        i === index ? { ...ch, value } : ch,
                      ),
                    })
                  const label = fieldLabel(change.path)
                  return change.value.kind === 'int' ? (
                    <NumberInput
                      key={label}
                      label={label}
                      value={change.value.int ?? 0}
                      onChange={(value) => {
                        if (typeof value === 'number') update({ kind: 'int', int: value })
                      }}
                    />
                  ) : change.value.kind === 'bool' ? (
                    <Checkbox
                      key={label}
                      label={label}
                      checked={!!change.value.bool}
                      onChange={(e) => update({ kind: 'bool', bool: e.currentTarget.checked })}
                    />
                  ) : (
                    <Textarea
                      key={label}
                      label={label}
                      autosize
                      value={
                        change.value.kind === 'slugs'
                          ? (change.value.slugs?.join('\n') ?? '')
                          : (change.value.string ?? change.value.slug ?? change.value.dice ?? '')
                      }
                      onChange={(e) => {
                        const value = e.currentTarget.value
                        update(
                          change.value.kind === 'slugs'
                            ? { kind: 'slugs', slugs: value.split('\n').filter(Boolean) }
                            : change.value.kind === 'slug'
                              ? { kind: 'slug', slug: value }
                              : change.value.kind === 'dice'
                                ? { kind: 'dice', dice: value }
                                : { kind: 'string', string: value },
                        )
                      }}
                    />
                  )
                })}
                {action.error && <Alert color="red">{action.error}</Alert>}
                <Group>
                  <Button type="submit" loading={action.pending}>
                    {t('imported.save')}
                  </Button>
                  <Button variant="subtle" onClick={() => setEditing(null)}>
                    {t('custom.cancel')}
                  </Button>
                </Group>
              </Stack>
            </form>
          )}
        </Stack>
      ))}
    </Stack>
  )
}
