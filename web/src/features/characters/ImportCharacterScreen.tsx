import { useState } from 'react'
import { useNavigate } from 'react-router'

import { importCharacter } from '@/lib/api'
import type { ImportEntry, ImportReport } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import {
  Alert,
  Button,
  Card,
  Divider,
  FileInput,
  Group,
  Stack,
  Text,
  Title,
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
  const navigate = useNavigate()
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<{ id: string; report: ImportReport } | null>(null)

  const submit = useAction(importCharacter)

  async function onImport() {
    if (file === null) return
    const imported = await submit.run(file)
    if (imported !== null) {
      setResult({ id: imported.id, report: imported.report })
    }
  }

  if (result !== null) {
    return <ImportedReport id={result.id} report={result.report} navigate={navigate} />
  }

  return (
    <Stack gap="md">
      <div>
        <Title order={2}>Import a character</Title>
        <Text c="dimmed" size="sm">
          Reads a sheet exported from HexSheet. Your character&apos;s numbers come across as
          they are; the choices behind them stay open for you to answer.
        </Text>
      </div>

      {submit.error !== null && (
        <Alert color="red" title="That file could not be imported">
          <Text size="sm">{submit.error}</Text>
        </Alert>
      )}

      <FileInput
        label="Exported sheet"
        description="A .json file exported from HexSheet"
        placeholder="Choose a file"
        accept="application/json,.json"
        value={file}
        onChange={setFile}
        clearable
      />

      <Group>
        <Button onClick={() => void onImport()} disabled={file === null} loading={submit.pending}>
          Import
        </Button>
        <Button variant="subtle" onClick={() => void navigate('/characters')}>
          Cancel
        </Button>
      </Group>
    </Stack>
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
  const nothingLost = report.unresolved.length === 0 && report.skipped.length === 0

  return (
    <Stack gap="md">
      <div>
        <Title order={2}>Imported</Title>
        <Text c="dimmed" size="sm">
          {nothingLost
            ? 'Everything in the export came across.'
            : 'Everything below did not come across. Nothing else was changed.'}
        </Text>
      </div>

      <EntryList
        title="Not in SRD 5.1"
        hint="easydnd only knows what the SRD publishes, which is one background and one feat."
        entries={report.unresolved}
      />
      <EntryList
        title="Not imported"
        hint="Real data easydnd has nowhere to put yet."
        entries={report.skipped}
      />

      {report.open.length > 0 && (
        <Card withBorder padding="md">
          <Text fw={500}>Still to decide</Text>
          <Text size="sm" c="dimmed">
            An import records what your character is, not what you chose, so these {report.open.length}{' '}
            choices are still open. The build screen asks for them.
          </Text>
        </Card>
      )}

      <Divider />

      <Group>
        <Button onClick={() => void navigate(`/characters/${id}/build`)}>
          Finish this character
        </Button>
        <Button variant="subtle" onClick={() => void navigate(`/characters/${id}`)}>
          View the sheet
        </Button>
      </Group>
    </Stack>
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
