import { useState } from 'react'
import { getGroupPacks, listPacks, sharePack, unsharePack } from '@/lib/api/packs'
import { describeError } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Alert, Button, Group, Panel, Select, Stack, Text } from '@/ui'
/**
 * A superadmin's control for granting a private disk pack to a table. Members
 * of the group then find the pack in their own pack picker; nobody else does.
 */
export function GroupPacks({ group, canManage }: { group: string; canManage: boolean }) {
  const t = useT()
  const { user } = useAuth()
  const loaded = useResource(`group-packs:${group}`, async () => {
    const [shares, available] = await Promise.all([getGroupPacks(group), listPacks()])
    return { shares, available }
  })
  const [selection, setSelection] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  async function act(work: () => Promise<unknown>) {
    setPending(true)
    setError('')
    try {
      await work()
      loaded.refresh()
    } catch (e) {
      setError(describeError(t, e))
    } finally {
      setPending(false)
    }
  }
  return (
    <Stack>
      {(error || loaded.error) && (
        <Alert color="red">{error || describeError(t, loaded.error)}</Alert>
      )}
      <Group align="end">
        <Select
          label={t('packs.share')}
          value={selection}
          searchable
          onChange={setSelection}
          data={(loaded.data?.available.packs ?? [])
            .filter((p) => p.shareable)
            .flatMap((p) =>
              p.releases.map((r) => ({
                value: `${p.id}@${r.version}`,
                label: `${p.title} v${r.version}`,
              })),
            )}
        />
        <Button
          disabled={!selection || pending}
          onClick={() => {
            const [id, version] = selection!.split('@')
            void act(() => sharePack(group, id!, version!))
          }}
        >
          {t('packs.share')}
        </Button>
      </Group>
      {(loaded.data?.shares ?? []).map((s) => (
        <Panel key={s.pack}>
          <Group justify="space-between">
            <Text>
              {loaded.data?.available.packs.find((p) => p.id === s.pack)?.title ?? s.pack} v
              {s.rules.packs.find((p) => p.id === s.pack)?.version}
            </Text>
            {(canManage || s.contributor === user?.id) && (
              <Button
                variant="subtle"
                disabled={pending}
                onClick={() => void act(() => unsharePack(group, s.pack))}
              >
                {t('packs.unshare')}
              </Button>
            )}
          </Group>
        </Panel>
      ))}
    </Stack>
  )
}
