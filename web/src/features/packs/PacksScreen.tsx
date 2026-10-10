import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { createPack, importPack, listPacks } from '@/lib/api/packs'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Alert, Anchor, Badge, Button, FileInput, Group, Page, Panel, Stack, TextInput } from '@/ui'

export function PacksScreen() {
  const t = useT()
  const navigate = useNavigate()
  const list = useResource('packs', listPacks)
  const [title, setTitle] = useState('')
  const create = useAction(async (file?: File) => {
    const p = file ? await importPack(file) : await createPack(title)
    await navigate(`/homebrew/${p.id}`)
  })
  const error = create.error ?? list.error
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
            <Button disabled={!title.trim()} loading={create.pending} onClick={() => void create.run()}>
              {t('packs.create')}
            </Button>
            <FileInput
              label={t('packs.import')}
              accept="application/json,application/zip,.json,.zip"
              disabled={create.pending}
              onChange={(file) => {
                if (file) void create.run(file)
              }}
            />
          </Group>
        </Panel>
        {error && <Alert color="red">{error}</Alert>}
        {(list.data?.packs ?? []).map((p) => (
          <Panel key={p.id}>
            <Group justify="space-between">
              <Anchor component={Link} to={`/homebrew/${p.id}`}>
                {p.title}
              </Anchor>
              <Badge>
                {p.restricted ? t('packs.restricted') : p.builtin ? t('packs.builtin') : p.owned ? t('packs.personal') : t('packs.shared')}
              </Badge>
              {p.archived && <Badge>{t('packs.archived')}</Badge>}
            </Group>
          </Panel>
        ))}
      </Stack>
    </Page>
  )
}
