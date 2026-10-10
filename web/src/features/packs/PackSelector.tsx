import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { listPacks, resolvePacks, type RulesLock, type PackRelease } from '@/lib/api/packs'
import { describeError } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useT } from '@/lib/i18n'
import { Alert, BlockList, Button, Group, Stack, Text } from '@/ui'

export function PackSelector({
  value,
  onChange,
  disabled = false,
  finalized = false,
  onDirtyChange,
  disclosure,
  children,
}: {
  value?: RulesLock | undefined
  onChange: (lock: RulesLock) => void
  disabled?: boolean
  finalized?: boolean
  onDirtyChange?: (dirty: boolean) => void
  disclosure?: { open: boolean; onOpen: (open: boolean) => void }
  children?: ReactNode
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
  const surfaceRef = useRef<HTMLDivElement>(null)
  const open = disclosure?.open ?? true
  const focusOnOpen = disclosure?.open ?? false
  useEffect(() => {
    if (!focusOnOpen || list.data === null) return
    const frame = requestAnimationFrame(() => {
      const first = surfaceRef.current?.querySelector<HTMLButtonElement>('button:not(:disabled)')
      first?.focus({ preventScroll: true })
    })
    return () => cancelAnimationFrame(frame)
  }, [focusOnOpen, list.data])
  const selected = roots ?? value?.packs ?? list.data?.defaultRules?.packs ?? []
  function select(next: PackRelease[]) {
    if (finalized) return
    setRoots(next)
    setFailure('')
    onDirtyChange?.(true)
  }
  async function apply() {
    if (finalized) return
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
  // Finalized is the same list with everything locked, so editing a
  // character shows the form it was created on.
  const form = (
    <Stack gap="md" ref={surfaceRef}>
      {list.error && <Alert>{describeError(t, list.error)}</Alert>}
      <Stack gap="xs">
        {(list.data?.packs ?? [])
          .filter((p) => (!p.archived || selected.some((r) => r.id === p.id)) && p.releases.length > 0)
          .flatMap((p) => [...p.releases].reverse().map((release) => {
            const chosen = selected.some((r) => r.id === p.id && r.version === release.version)
            return (
              <Button
                key={`${p.id}@${release.version}`}
                variant={chosen ? 'light' : 'default'}
                aria-pressed={chosen}
                justify="flex-start"
                h="auto"
                py="xs"
                disabled={disabled || pending || finalized}
                onClick={() => {
                  const remaining = selected.filter((r) => r.id !== p.id)
                  select(chosen ? remaining : [...remaining, release])
                }}
              >
                <Text size="sm" style={{ whiteSpace: 'normal', textAlign: 'left' }}>{p.title} v{release.version}</Text>
              </Button>
            )
          }))}
      </Stack>
      {!finalized && <Group>
        <Button
          disabled={disabled || selected.length === 0 || list.data === null}
          loading={pending}
          onClick={() => void apply()}
        >
          {selected.length > 0 ? t('answer.confirm') : t('prompt.chooseMore', { count: 1 })}
        </Button>
        {selected.length > 0 && <Button variant="subtle" disabled={disabled || pending} onClick={() => select([])}>
          {t('prompt.clear')}
        </Button>}
      </Group>}
      <Text size="xs" c="dimmed">{t(finalized ? 'ruleset.final' : 'packs.dependenciesHint')}</Text>
      {value && <Text size="xs" c="dimmed">
        {t('packs.included')}: {value.packs.map((p) => `${p.id}@${p.version}`).join(', ')}
      </Text>}
      {failure && <Alert color="red">{failure}</Alert>}
      {children}
    </Stack>
  )
  if (disclosure) return <BlockList
    open={open ? 'packs' : null}
    onOpen={(key) => disclosure.onOpen(key !== null)}
    items={[{
      key: 'packs',
      header: <Text size="sm" fw={600}>{t('packs.selection')}</Text>,
      highlighted: !finalized && (roots !== null || value === undefined),
      body: form,
    }]}
  />
  return <Stack gap="sm"><Text fw={600}>{t('packs.selection')}</Text>{form}</Stack>
}
