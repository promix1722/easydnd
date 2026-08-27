import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { importCharacter } from '@/lib/api'
import type { ImportEntry, ImportReport } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import {
  Alert,
  Button,
  Card,
  Divider,
  FileInput,
  Group,
  Page,
  Stack,
  Text,
} from '@/ui'

/**
 * Imports a character exported from another tool.
 *
 * The screen is two steps rather than one, and the second step is the point.
 * An import is lossy by construction -- SRD 5.1 publishes one background and
 * one feat, so a sheet from a tool with the full rules always leaves something
 * behind -- so the report is shown and acknowledged before the player moves
 * on. Navigating straight to the new character would make the import look
 * lossless and let the player discover otherwise from a wrong number later.
 *
 * The onward link goes to the build screen, not the sheet: an import answers
 * no prompts, so there is always something left to decide.
 */
export function ImportCharacterScreen() {
  const t = useT()
  const navigate = useNavigate()
  // The folder the character list was filtered to when Import was pressed.
  const [params] = useSearchParams()
  const folder = params.get('folder') ?? undefined
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<{ id: string; report: ImportReport } | null>(null)

  const submit = useAction(importCharacter)

  async function onImport() {
    if (file === null) return
    const imported = await submit.run(file, folder)
    if (imported !== null) {
      setResult({ id: imported.id, report: imported.report })
    }
  }

  if (result !== null) {
    return <ImportedReport id={result.id} report={result.report} navigate={navigate} />
  }

  return (
    <Page
      trail={[{ label: t('import.title') }]}
      subtitle={
        <>
          {t('import.lead')}
        </>
      }
    >
      <Stack gap="md">

        {submit.error !== null && (
          <Alert color="red" title={t('import.failed')}>
            <Text size="sm">{submit.error}</Text>
          </Alert>
        )}

        <FileInput
          label={t('import.fileLabel')}
          description={t('import.fileHint')}
          placeholder={t('import.filePlaceholder')}
          accept="application/json,.json"
          value={file}
          onChange={setFile}
          clearable
        />

        <Group>
          <Button onClick={() => void onImport()} disabled={file === null} loading={submit.pending}>
            {t('characters.import')}
          </Button>
          <Button variant="subtle" onClick={() => void navigate('/characters')}>
            {t('common.cancel')}
          </Button>
        </Group>
      </Stack>
    </Page>
  )
}

/** The report, shown once the character exists. */
function ImportedReport({
  id,
  report,
  navigate,
}: {
  id: string
  report: ImportReport
  navigate: ReturnType<typeof useNavigate>
}) {
  const t = useT()
  const nothingLost = report.unresolved.length === 0 && report.skipped.length === 0

  return (
    <Page
      trail={[{ label: t('import.done') }]}
      subtitle={
        nothingLost ? t('import.nothingLost') : t('import.somethingLost')
      }
    >
      <Stack gap="md">

        <EntryList
          title={t('import.notInSrd')}
          hint={t('import.notInSrdHint')}
          entries={report.unresolved}
        />
        <EntryList
          title={t('import.notImported')}
          hint={t('import.notImportedHint')}
          entries={report.skipped}
        />

        {report.open.length > 0 && (
          <Card withBorder padding="md">
            <Text fw={500}>{t('import.stillToDecide')}</Text>
            <Text size="sm" c="dimmed">
              An import records what your character is, not what you chose, so these {report.open.length}{' '}
              choices are still open. The build screen asks for them.
            </Text>
          </Card>
        )}

        <Divider />

        <Group>
          <Button onClick={() => void navigate(`/characters/${id}/build`)}>
            {t('import.finish')}
          </Button>
          <Button variant="subtle" onClick={() => void navigate(`/characters/${id}`)}>
            {t('import.viewSheet')}
          </Button>
        </Group>
      </Stack>
    </Page>
  )
}

function EntryList({
  title,
  hint,
  entries,
}: {
  title: string
  hint: string
  entries: ImportEntry[]
}) {
  if (entries.length === 0) return null
  return (
    <Card withBorder padding="md">
      <Text fw={500}>{title}</Text>
      <Text size="sm" c="dimmed">
        {hint}
      </Text>
      <Stack gap={4} mt="xs">
        {entries.map((entry, i) => (
          <Text key={`${entry.field}-${i}`} size="sm">
            {entry.detail}
          </Text>
        ))}
      </Stack>
    </Card>
  )
}
