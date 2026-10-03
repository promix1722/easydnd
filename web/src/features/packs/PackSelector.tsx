import { useState } from 'react'
import { listPacks, resolvePacks, type RulesLock, type PackRelease } from '@/lib/api/packs'
import { describeError } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useT } from '@/lib/i18n'
import { Alert, Button, Checkbox, Group, Select, Stack, Text } from '@/ui'

export function PackSelector({
  value,
  onChange,
  disabled = false,
  onDirtyChange,
}: {
  value?: RulesLock | undefined
  onChange: (lock: RulesLock) => void
  disabled?: boolean
  onDirtyChange?: (dirty: boolean) => void
}) {
  const t = useT()
  const list = useResource('pack-selection', listPacks)
  const [roots, setRoots] = useState<PackRelease[] | null>(null)
  const signature = JSON.stringify(value?.packs)
  const [shown, setShown] = useState(signature)
  if (shown !== signature) {
    setShown(signature)
    setRoots(null)
  }
  const [failure, setFailure] = useState('')
  const [pending, setPending] = useState(false)
  const selected = roots ?? value?.packs ?? list.data?.defaultRules?.packs ?? []
  function select(next: PackRelease[]) {
    setRoots(next)
    onDirtyChange?.(true)
  }
  async function apply() {
    setPending(true)
    setFailure('')
    try {
      onChange(await resolvePacks(selected))
      onDirtyChange?.(false)
    } catch (e) {
      setFailure(describeError(t, e))
    } finally {
      setPending(false)
    }
  }
  return (
    <Stack gap="xs">
      <Text fw={600}>{t('packs.selection')}</Text>
      <Text size="xs" c="dimmed">
        {t('packs.dependenciesHint')}
      </Text>
      {list.error && <Alert>{describeError(t, list.error)}</Alert>}
      {(list.data?.packs ?? [])
        .filter((p) => !p.archived && p.releases.length > 0)
        .map((p) => {
          const release = selected.find((r) => r.id === p.id)
          return (
            <Group key={p.id} wrap="nowrap">
              <Checkbox
                label={p.title}
                checked={!!release}
                disabled={disabled || pending}
                onChange={(e) =>
                  select(
                    e.currentTarget.checked
                      ? [...selected, p.releases[p.releases.length - 1]!]
                      : selected.filter((r) => r.id !== p.id),
                  )
                }
              />
              {release && (
                <Select
                  aria-label={t('packs.version')}
                  value={release.version}
                  data={p.releases.map((r) => r.version)}
                  disabled={disabled || pending}
                  onChange={(version) => {
                    const next = p.releases.find((r) => r.version === version)
                    if (next) select(selected.map((r) => (r.id === p.id ? next : r)))
                  }}
                />
              )}
            </Group>
          )
        })}
      {value && (
        <Text size="xs" c="dimmed">
          {t('packs.included')}: {value.packs.map((p) => `${p.id}@${p.version}`).join(', ')}
        </Text>
      )}
      <Button
        variant="light"
        disabled={disabled || selected.length === 0 || list.data === null}
        loading={pending}
        onClick={() => void apply()}
      >
        {t('packs.applySelection')}
      </Button>
      {failure && <Alert color="red">{failure}</Alert>}
    </Stack>
  )
}
