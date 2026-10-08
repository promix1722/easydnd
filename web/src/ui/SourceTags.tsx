import { Badge, Group } from '@mantine/core'
import { bookSources } from '@/lib/bookSources'

interface Provenance {
  packId: string
  packTitle: string
  version: string
  sources: { id: string; name: string }[]
}

/** Publication metadata, separate from rules that grant an ability or spell. */
export function SourceTags({ provenance, rightAligned = false, oneLine = false }: { provenance?: Provenance | undefined; rightAligned?: boolean; oneLine?: boolean }) {
  if (!provenance) return null
  return <Group gap={4} component="span" wrap={oneLine ? 'nowrap' : 'wrap'} justify={rightAligned ? 'flex-end' : 'flex-start'} style={rightAligned ? { maxWidth: '55%', flexShrink: 0 } : undefined}>
    <Badge size="xs" color="blue" variant="light" title={`${provenance.packTitle} v${provenance.version}`} style={{ maxWidth: 180 }}>{provenance.packTitle} v{provenance.version}</Badge>
    {bookSources(provenance).map((source) => <Badge key={source.id} size="xs" color="blue" variant="light" title={source.name} style={{ height: 'auto', textAlign: 'left' }} styles={{ label: { whiteSpace: 'normal' } }}>{source.name}</Badge>)}
  </Group>
}
