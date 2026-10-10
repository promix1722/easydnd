import type { Sheet } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Stack, Text } from '@/ui'
import { fieldLabel } from './fieldLabels'

function changes(
  before: unknown,
  after: unknown,
  path: string[] = [],
): { path: string[]; before: unknown; after: unknown }[] {
  if (JSON.stringify(before) === JSON.stringify(after)) return []
  if (
    before &&
    after &&
    typeof before === 'object' &&
    typeof after === 'object' &&
    !Array.isArray(before) &&
    !Array.isArray(after)
  ) {
    const a = before as Record<string, unknown>
    const b = after as Record<string, unknown>
    return [...new Set([...Object.keys(a), ...Object.keys(b)])].flatMap((key) =>
      changes(a[key], b[key], [...path, key]),
    )
  }
  return [{ path, before, after }]
}
function display(value: unknown): string {
  if (value === undefined || value === null) return '—'
  if (Array.isArray(value)) return value.map(display).join(', ') || '—'
  if (typeof value === 'object') return Object.values(value).map(display).join(' · ')
  return String(value)
}
export function PackMigrationPreview({ before, after }: { before: Sheet; after: Sheet }) {
  const t = useT()
  const rows = changes(before, after)
  return (
    <Stack gap="xs">
      <Text>{t('packs.migrationPreview')}</Text>
      <div style={{ overflowX: 'auto' }}>
        <table>
          <thead>
            <tr>
              <th>{t('packs.changedField')}</th>
              <th>{t('packs.before')}</th>
              <th>{t('packs.after')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.path.join('.')}>
                <td>{row.path.map((p) => fieldLabel(t, p)).join(' / ')}</td>
                <td>{display(row.before)}</td>
                <td>{display(row.after)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Stack>
  )
}
