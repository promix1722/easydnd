import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import {
  archivePack,
  createPack,
  exportPack,
  getPack,
  getPackSchema,
  listPacks,
  publishPack,
  savePack,
  validatePack,
  type PackDocument,
  type PackRecord,
  type PackValidation,
  type PackValue,
} from '@/lib/api/packs'
import { describeError } from '@/lib/api'
import { useT, type MessageKey } from '@/lib/i18n'
import { useResource } from '@/lib/useResource'
import { Alert, Button, Group, Page, Panel, Select, Stack, Text, TextInput } from '@/ui'
import { PackForm } from './PackForm'

const kinds: Record<string, string> = {
  races: 'race',
  subraces: 'subrace',
  classes: 'class',
  subclasses: 'subclass',
  spells: 'spell',
  equipment: 'item',
  features: 'feature',
  feats: 'feat',
  traits: 'trait',
  backgrounds: 'background',
  abilities: 'ability',
  skills: 'skill',
  languages: 'language',
  proficiencies: 'proficiency',
  resources: 'resource',
  actions: 'action',
  rules: 'rule',
  'magic-items': 'magic-item',
  'damage-types': 'damage-type',
  'magic-schools': 'magic-school',
  'equipment-categories': 'equipment-category',
  'weapon-properties': 'weapon-property',
  alignments: 'alignment',
  conditions: 'condition',
}
function referencesIn(doc: PackDocument): string[] {
  const out: string[] = []
  for (const [collection, value] of Object.entries({ ...doc.entities, ...doc.mechanics })) {
    if (!Array.isArray(value) || !kinds[collection]) continue
    for (const entry of value)
      if (entry && typeof entry === 'object' && !Array.isArray(entry)) {
        const id = entry.id ?? entry.slug
        if (typeof id === 'string') out.push(`${doc.manifest.id}:${kinds[collection]}:${id}`)
      }
  }
  return out
}
export function PackEditorScreen() {
  const { id = '' } = useParams()
  const t = useT()
  const navigate = useNavigate()
  const loaded = useResource(`pack:${id}`, async () => {
    const [pack, schema, available] = await Promise.all([getPack(id), getPackSchema(), listPacks()])
    return { pack, schema, available }
  })
  const [pack, setPack] = useState<PackRecord | null>(null)
  const [doc, setDoc] = useState<PackDocument | null>(null)
  const [version, setVersion] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const [validation, setValidation] = useState<PackValidation | null>(null)
  const [dirty, setDirty] = useState(false)
  const [references, setReferences] = useState<string[]>([])
  const [mappings, setMappings] = useState<Record<string, string>>({})
  const [shown, setShown] = useState(loaded.data)
  if (loaded.data && shown !== loaded.data) {
    setShown(loaded.data)
    if (!dirty || pack?.id !== loaded.data.pack.id) {
      setPack(loaded.data.pack)
      setDoc(loaded.data.pack.draft ?? null)
      setVersion(loaded.data.pack.releases.at(-1)?.version ?? '')
      setDirty(false)
    }
  }
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (dirty) e.preventDefault()
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [dirty])
  useEffect(() => {
    let current = true
    const deps = doc?.manifest.dependencies
    if (!Array.isArray(deps)) return
    const ids = deps.flatMap((d) =>
      d && typeof d === 'object' && !Array.isArray(d) && typeof d.id === 'string' ? [d.id] : [],
    )
    const packs = loaded.data?.available.packs.filter((p) => ids.includes(p.id)) ?? []
    void Promise.all(
      packs.map((p) => exportPack(p.id, p.releases.at(-1)?.version ?? '').then(referencesIn)),
    )
      .then((all) => {
        if (current) setReferences(all.flat())
      })
      .catch(() => {
        if (current) setReferences([])
      })
    return () => {
      current = false
    }
  }, [doc?.manifest.dependencies, loaded.data?.available])
  async function act(work: () => Promise<void>) {
    setError('')
    setPending(true)
    try {
      await work()
    } catch (e) {
      setError(describeError(t, e))
    } finally {
      setPending(false)
    }
  }
  async function persist() {
    if (!pack || !doc) return null
    const saved = await savePack(pack, doc, mappings)
    setPack(saved)
    setDoc(saved.draft ?? null)
    setMappings({})
    setDirty(false)
    return saved
  }
  async function download() {
    if (!pack) return
    if (pack.owned && doc) await persist()
    const data = await exportPack(id, pack.owned ? '' : version)
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }),
    )
    const link = document.createElement('a')
    link.href = url
    link.download = `${id}.json`
    link.click()
    URL.revokeObjectURL(url)
  }
  const dependencies = Array.isArray(doc?.manifest.dependencies) ? doc.manifest.dependencies : []
  return (
    <Page trail={[{ label: pack?.title ?? null }]}>
      <Stack>
        {(error || loaded.error) && (
          <Alert color="red">{error || describeError(t, loaded.error)}</Alert>
        )}
        {pack && (
          <Panel>
            <Stack>
              <TextInput
                label={t('packs.title')}
                value={pack.title}
                readOnly={!pack.owned}
                onChange={(e) => {
                  setPack({ ...pack, title: e.currentTarget.value })
                  setDirty(true)
                }}
              />
              <Group>
                {pack.owned && (
                  <>
                    <Button
                      disabled={!doc || pending}
                      onClick={() =>
                        void act(async () => {
                          await persist()
                        })
                      }
                    >
                      {t('packs.save')}
                    </Button>
                    <Button
                      disabled={!doc || pending}
                      onClick={() =>
                        void act(async () => {
                          if (await persist()) setValidation(await validatePack(id))
                        })
                      }
                    >
                      {t('packs.validate')}
                    </Button>
                    <Button
                      disabled={!doc || pending}
                      onClick={() =>
                        void act(async () => {
                          const saved = await persist()
                          if (!saved) return
                          const result = await validatePack(id)
                          setValidation(result)
                          if (result.valid) {
                            const released = await publishPack(saved)
                            setPack(released)
                            setVersion(released.releases.at(-1)?.version ?? '')
                          }
                        })
                      }
                    >
                      {t('packs.publish')}
                    </Button>
                    <Button
                      variant="subtle"
                      disabled={pending || pack.archived}
                      onClick={() =>
                        void act(async () => {
                          const saved = await persist()
                          if (saved) setPack(await archivePack(saved))
                        })
                      }
                    >
                      {t('packs.archive')}
                    </Button>
                  </>
                )}
                <Button variant="light" disabled={pending} onClick={() => void act(download)}>
                  {t('packs.export')}
                </Button>
                <Button
                  variant="light"
                  disabled={pending || (!doc && !version)}
                  onClick={() =>
                    void act(async () => {
                      const copy = await createPack(
                        pack.title,
                        doc ?? (await exportPack(id, version)),
                      )
                      await navigate(`/homebrew/${copy.id}`)
                    })
                  }
                >
                  {t('packs.duplicate')}
                </Button>
              </Group>
              {dirty && <Text size="sm">{t('packs.unsaved')}</Text>}
              {pack.releases.length > 0 && (
                <Group align="end">
                  <Select
                    label={t('packs.release')}
                    value={version}
                    data={pack.releases.map((r) => r.version)}
                    onChange={(v) => setVersion(v ?? '')}
                  />
                  <Button
                    variant="light"
                    disabled={pending || dirty || !version}
                    onClick={() =>
                      void act(async () => {
                        setDoc(await exportPack(id, version))
                        setDirty(pack.owned)
                        setValidation(null)
                      })
                    }
                  >
                    {pack.owned ? t('packs.editRelease') : t('packs.viewRelease')}
                  </Button>
                </Group>
              )}
            </Stack>
          </Panel>
        )}
        {validation && (
          <Alert
            color={validation.valid ? 'green' : 'red'}
            title={validation.valid ? t('packs.valid') : t('packs.invalid')}
          >
            {validation.diagnostics.map((d, i) => (
              <Stack key={i} gap="xs">
                <Button
                  variant="subtle"
                  onClick={() => {
                    const node = document.getElementById(`pack-field${d.path ?? ''}`)
                    let parent: HTMLElement | null = node
                    while (parent) {
                      if (parent.tagName === 'DETAILS') (parent as HTMLDetailsElement).open = true
                      parent = parent.parentElement
                    }
                    node?.scrollIntoView({ block: 'center' })
                  }}
                >
                  {t(`error.${d.reason}` as MessageKey, d.args)}
                </Button>
                <details>
                  <summary>{t('packs.compilerDetails')}</summary>
                  <Text size="sm">{d.detail}</Text>
                </details>
              </Stack>
            ))}
          </Alert>
        )}
        {pack?.owned && dependencies.length > 0 && (
          <Panel>
            <Stack>
              <Text>{t('packs.mapDependencies')}</Text>
              {dependencies.map((dep, i) => {
                if (
                  !dep ||
                  typeof dep !== 'object' ||
                  Array.isArray(dep) ||
                  typeof dep.id !== 'string'
                )
                  return null
                const source = dep.id
                return (
                  <Select
                    key={i}
                    label={source}
                    searchable
                    clearable
                    value={mappings[source] ?? null}
                    data={(loaded.data?.available.packs ?? [])
                      .filter((p) => p.id !== id && p.releases.length > 0)
                      .map((p) => ({ value: p.id, label: `${p.title} v${p.releases.at(-1)!.version}` }))}
                    onChange={(v) => {
                      const next = { ...mappings }
                      if (v) next[source] = v
                      else delete next[source]
                      setMappings(next)
                      setDirty(true)
                    }}
                  />
                )
              })}
            </Stack>
          </Panel>
        )}
        {doc && loaded.data && (
          <Panel>
            <fieldset
              disabled={!pack?.owned || pending}
              style={{ border: 0, padding: 0, minWidth: 0 }}
            >
              <PackForm
                name="document"
                schema={loaded.data.schema}
                field={loaded.data.schema.root}
                value={doc as unknown as PackValue}
                references={[...references, ...referencesIn(doc)]}
                onChange={(value) => {
                  if (value) {
                    setDoc(value as unknown as PackDocument)
                    setDirty(true)
                    setValidation(null)
                  }
                }}
              />
            </fieldset>
          </Panel>
        )}
      </Stack>
    </Page>
  )
}
