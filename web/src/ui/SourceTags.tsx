import { Badge, Group } from '@mantine/core'
import { sourceAbbreviation } from './sourceAbbreviation'

interface Provenance {
  packId: string
  packTitle: string
  version: string
  sources: { id: string; name: string }[]
}

/** Publication metadata, separate from rules that grant an ability or spell. */
export function SourceTags({ provenance, rightAligned = false }: { provenance?: Provenance | undefined; rightAligned?: boolean }) {
  if (!provenance) return null
  return <Group gap={4} component="span" wrap="wrap" justify={rightAligned ? 'flex-end' : 'flex-start'} style={rightAligned ? { maxWidth: '55%', flexShrink: 0 } : undefined}>
    <Badge size="xs" color="blue" variant="light" title={`${provenance.packTitle} · ${provenance.version}`} style={{ maxWidth: 180 }}>{provenance.packTitle}</Badge>
    {provenance.sources.map((source) => <Badge key={source.id} size="xs" color="blue" variant="light" title={source.name}>{sourceAbbreviation(source.id)}</Badge>)}
  </Group>
}
