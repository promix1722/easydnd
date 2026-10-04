import { useState } from 'react'
import { upsertCustomOption } from '@/lib/api/characters'
import type { CustomOption, Sheet } from '@/lib/api/characters'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import type { Stage } from '@/domain'
import {
  Alert,
  Badge,
  Button,
  Checkbox,
  Group,
  ModalSheet,
  NumberInput,
  Select,
  Stack,
  Text,
  Textarea,
  TextInput,
} from '@/ui'

const kinds = {
  class: ['class', 'subclass', 'feature', 'feat', 'cantrip', 'spell'],
  race: ['race', 'subrace', 'trait'],
  background: ['background'],
  cantrips: ['cantrip'],
  spells: ['spell'],
  equipment: ['item'],
  personal: ['note'],
  personality: ['note'],
  rules: [],
  abilities: [],
} satisfies Record<Stage, string[]>
const labels = {
  class: 'agent.field.class',
  subclass: 'agent.field.subclass',
  feature: 'agent.field.feature',
  feat: 'agent.field.feat',
  race: 'agent.field.race',
  subrace: 'agent.field.subrace',
  trait: 'agent.field.trait',
  background: 'agent.field.background',
  cantrip: 'agent.field.cantrip',
  spell: 'agent.field.spell',
  item: 'agent.field.item',
  note: 'custom.note',
} as const
export function CustomOptionsPanel({
  id,
  stage,
  sheet,
  revision,
  onSaved,
}: {
  id: string
  stage: Stage
  sheet: Sheet
  revision: number
  onSaved: () => void
}) {
  const t = useT()
  const [draft, setDraft] = useState<CustomOption | null>(null)
  const action = useAction(upsertCustomOption)
  const choices: readonly string[] = kinds[stage]
  const items = (sheet.customOptions ?? []).filter(
    (c) =>
      choices.includes(c.kind) && !(stage === 'class' && ['spell', 'cantrip'].includes(c.kind)),
  )
  const submit = async (option: CustomOption) => {
    const result = await action.run(id, revision, option)
    if (result) {
      setDraft(null)
      onSaved()
    }
  }
  if (!choices.length) return null
  return (
    <Stack gap="sm">
      {items.map((option) => (
        <Group key={option.id} justify="space-between">
          <Group gap="xs">
            <Text>{option.name}</Text>
            <Badge>{t('custom.manual')}</Badge>
            {!option.selected && (
              <Text size="xs" c="dimmed">
                {t('custom.unselected')}
              </Text>
            )}
          </Group>
          <Button
            variant="light"
            onClick={() => {
              action.reset()
              setDraft(option)
            }}
          >
            {t('custom.edit')}
          </Button>
        </Group>
      ))}
      <Group>
        <Button
          variant="subtle"
          onClick={() => {
            action.reset()
            setDraft({ kind: choices[0]!, name: '', description: '', source: '', selected: true })
          }}
        >
          {t('custom.add')}
        </Button>
      </Group>
      <ModalSheet
        opened={draft !== null}
        onClose={() => {
          if (!action.pending) setDraft(null)
        }}
        title={draft?.id ? t('custom.edit') : t('custom.add')}
      >
        {draft && (
          <form
            onSubmit={(e) => {
              e.preventDefault()
              void submit(draft)
            }}
          >
            <Stack>
              <Text size="sm" c="dimmed">
                {t('custom.hint')}
              </Text>
              <Select
                label={t('custom.kind')}
                value={draft.kind}
                disabled={!!draft.id || action.pending}
                data={choices.map((kind) => ({
                  value: kind,
                  label: t(labels[kind as keyof typeof labels]),
                }))}
                onChange={(kind) => {
                  if (kind)
                    setDraft({
                      name: draft.name,
                      description: draft.description,
                      source: draft.source,
                      selected: draft.selected,
                      kind,
                    })
                }}
              />
              <TextInput
                required
                label={t('custom.name')}
                value={draft.name}
                maxLength={300}
                disabled={action.pending}
                onChange={(e) => setDraft({ ...draft, name: e.currentTarget.value })}
              />
              <Textarea
                label={t('custom.description')}
                value={draft.description}
                maxLength={16000}
                autosize
                minRows={3}
                disabled={action.pending}
                onChange={(e) => setDraft({ ...draft, description: e.currentTarget.value })}
              />
              <TextInput
                label={t('custom.source')}
                value={draft.source}
                maxLength={1000}
                disabled={action.pending}
                onChange={(e) => setDraft({ ...draft, source: e.currentTarget.value })}
              />
              {['subclass', 'subrace', 'feature', 'spell', 'cantrip'].includes(draft.kind) && (
                <Select
                  label={t('custom.parent')}
                  value={draft.parent ?? null}
                  data={(draft.kind === 'subrace'
                    ? [sheet.identity.race].filter((v): v is string => !!v)
                    : [
                        ...(sheet.identity.classes ?? []).map((v) => v.class),
                        ...(['spell', 'cantrip'].includes(draft.kind) && sheet.identity.race
                          ? [sheet.identity.race]
                          : []),
                        ...(draft.parent ? [draft.parent] : []),
                      ].filter((v, i, all) => all.indexOf(v) === i)
                  ).map((value) => ({
                    value,
                    label:
                      sheet.catalogNames?.[
                        `${draft.kind === 'subrace' ? 'races' : 'classes'}:${value}`
                      ] ?? value,
                  }))}
                  onChange={(parent) => setDraft({ ...draft, parent: parent ?? '' })}
                />
              )}
              {['class', 'spell'].includes(draft.kind) && (
                <NumberInput
                  label={t('custom.level')}
                  placeholder={t('custom.unknown')}
                  value={draft.level ?? ''}
                  min={draft.kind === 'class' ? 1 : 1}
                  max={draft.kind === 'class' ? 20 : 9}
                  onChange={(level) => {
                    const next = { ...draft }
                    if (typeof level === 'number') next.level = level
                    else delete next.level
                    setDraft(next)
                  }}
                />
              )}
              {draft.kind === 'class' && (
                <NumberInput
                  label={t('custom.hitDie')}
                  placeholder={t('custom.unknown')}
                  value={draft.hitDie ?? ''}
                  min={4}
                  max={12}
                  onChange={(hitDie) => {
                    const next = { ...draft }
                    if (typeof hitDie === 'number') next.hitDie = hitDie
                    else delete next.hitDie
                    setDraft(next)
                  }}
                />
              )}
              {draft.kind === 'race' && (
                <NumberInput
                  label={t('custom.speed')}
                  placeholder={t('custom.unknown')}
                  value={draft.speed ?? ''}
                  min={0}
                  max={1000}
                  onChange={(speed) => {
                    const next = { ...draft }
                    if (typeof speed === 'number') next.speed = speed
                    else delete next.speed
                    setDraft(next)
                  }}
                />
              )}
              {['spell', 'cantrip', 'subclass', 'class'].includes(draft.kind) && (
                <Select
                  clearable
                  label={t('custom.ability')}
                  value={draft.ability ?? null}
                  data={['str', 'dex', 'con', 'int', 'wis', 'cha'].map((ability) => ({
                    value: ability,
                    label: ability.toUpperCase(),
                  }))}
                  onChange={(ability) => setDraft({ ...draft, ability: ability ?? '' })}
                />
              )}
              {draft.kind === 'spell' && (
                <Select
                  label={t('custom.mode')}
                  value={draft.mode ?? 'known'}
                  data={['known', 'prepared', 'granted', 'spellbook'].map((mode) => ({
                    value: mode,
                    label: t(
                      mode === 'known'
                        ? 'custom.known'
                        : mode === 'prepared'
                          ? 'custom.prepared'
                          : mode === 'spellbook'
                            ? 'custom.spellbook'
                            : 'custom.granted',
                    ),
                  }))}
                  onChange={(mode) => setDraft({ ...draft, mode: mode ?? 'known' })}
                />
              )}
              {draft.kind === 'item' && (
                <>
                  <NumberInput
                    label={t('custom.count')}
                    value={draft.count ?? 1}
                    min={1}
                    max={100000}
                    onChange={(count) =>
                      setDraft({ ...draft, count: typeof count === 'number' ? count : 1 })
                    }
                  />
                  <Select
                    label={t('custom.placement')}
                    value={draft.placement ?? 'backpack'}
                    data={['equipped', 'backpack', 'loot'].map((placement) => ({
                      value: placement,
                      label: t(
                        placement === 'equipped'
                          ? 'custom.equipped'
                          : placement === 'loot'
                            ? 'custom.loot'
                            : 'custom.backpack',
                      ),
                    }))}
                    onChange={(placement) =>
                      setDraft({ ...draft, placement: placement ?? 'backpack' })
                    }
                  />
                </>
              )}
              <Checkbox
                label={t('custom.selected')}
                checked={draft.selected}
                onChange={(e) => setDraft({ ...draft, selected: e.currentTarget.checked })}
              />
              {action.error && <Alert color="red">{action.error}</Alert>}
              <Group>
                <Button type="submit" loading={action.pending} disabled={!draft.name.trim()}>
                  {t('custom.save')}
                </Button>
                <Button variant="subtle" disabled={action.pending} onClick={() => setDraft(null)}>
                  {t('custom.cancel')}
                </Button>
              </Group>
              {draft.id &&
                draft.selected &&
                ['class', 'race', 'background', 'subclass', 'subrace'].includes(draft.kind) && (
                  <Button
                    variant="light"
                    disabled={action.pending}
                    onClick={() => void submit({ ...draft, selected: false })}
                  >
                    {t('custom.replace')}
                  </Button>
                )}
            </Stack>
          </form>
        )}
      </ModalSheet>
    </Stack>
  )
}
