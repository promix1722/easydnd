import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { createPack, importPack, listPacks } from '@/lib/api/packs'
import { describeError } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Alert, Anchor, Badge, Button, FileInput, Group, Page, Panel, Stack, TextInput } from '@/ui'

export function PacksScreen() {
  const t = useT()
  const navigate = useNavigate()
  const list = useResource('packs', listPacks)
  const [title, setTitle] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  async function create(file?: File) {
    setPending(true)
    setError('')
    try {
      const p = file ? await importPack(file) : await createPack(title)
      await navigate(`/homebrew/${p.id}`)
    } catch (e) {
      setError(describeError(t, e))
    } finally {
      setPending(false)
    }
  }
  return (
    <Page trail={[]}>
      <Stack>
        <Panel>
          <Group align="end">
            <TextInput
              label={t('packs.title')}
              value={title}
              onChange={(e) => setTitle(e.currentTarget.value)}
            />
            <Button disabled={!title.trim()} loading={pending} onClick={() => void create()}>
              {t('packs.create')}
            </Button>
            <FileInput
              label={t('packs.import')}
              accept="application/json,application/zip,.json,.zip"
              disabled={pending}
              onChange={(file) => {
                if (file) void create(file)
              }}
            />
          </Group>
        </Panel>
        {(error || list.error) && (
          <Alert color="red">{error || describeError(t, list.error)}</Alert>
        )}
        {(list.data?.packs ?? []).map((p) => (
          <Panel key={p.id}>
            <Group justify="space-between">
              <Anchor component={Link} to={`/homebrew/${p.id}`}>
                {p.title}
              </Anchor>
              <Badge>
                {p.builtin ? t('packs.builtin') : p.owned ? t('packs.personal') : t('packs.shared')}
              </Badge>
              {p.archived && <Badge>{t('packs.archived')}</Badge>}
            </Group>
          </Panel>
        ))}
      </Stack>
    </Page>
  )
}
