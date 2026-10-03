import { useState } from 'react'
import type { PackField, PackSchema, PackValue } from '@/lib/api/packs'
import { useT } from '@/lib/i18n'
import { Button, Checkbox, Group, NumberInput, Select, Stack, TextInput, Textarea } from '@/ui'
import { optionLabel } from './optionLabels'
import { fieldLabel } from './fieldLabels'

function initialValue(field: PackField, schema: PackSchema): PackValue {
  const spec = field.ref ? schema.definitions[field.ref]! : field
  if (spec.type === 'object')
    return Object.fromEntries(
      Object.entries(spec.properties ?? {})
        .filter(([, child]) => !child.optional)
        .map(([key, child]) => [key, initialValue(child, schema)]),
    )
  if (spec.type === 'map') return {}
  if (spec.type === 'array') return []
  if (spec.type === 'number') return 0
  if (spec.type === 'boolean') return false
  return ''
}

/** A recursive form over the codec's actual wire types, preserving omitted values. */
export function PackForm({
  field,
  schema,
  value,
  onChange,
  name,
  references,
  path = '',
  depth = 0,
}: {
  field: PackField
  schema: PackSchema
  value: PackValue | undefined
  onChange: (value: PackValue | undefined) => void
  name: string
  references: string[]
  path?: string
  depth?: number
}) {
  const t = useT()
  const [newKey, setNewKey] = useState('')
  const [expanded, setExpanded] = useState(depth < 2)
  const spec = field.ref ? schema.definitions[field.ref]! : field
  const label = fieldLabel(t, name)
  if (depth > 64) return null
  if (value === undefined || value === null)
    return (
      <Button variant="subtle" size="xs" onClick={() => onChange(initialValue(field, schema))}>
        {t('packs.addField', { field: label })}
      </Button>
    )
  const nested = (
    child: PackField,
    val: PackValue | undefined,
    key: string,
    change: (v: PackValue | undefined) => void,
    pathKey = key,
  ) => (
    <PackForm
      key={key}
      field={child}
      schema={schema}
      value={val}
      onChange={change}
      name={key}
      references={references}
      path={`${path}/${pathKey}`}
      depth={depth + 1}
    />
  )
  if (spec.type === 'object' || spec.type === 'map') {
    const obj = value as Record<string, PackValue>
    const properties =
      spec.properties ?? Object.fromEntries(Object.keys(obj).map((key) => [key, spec.items!]))
    const entries = Object.entries(properties).filter(
      ([key]) => !(path === '/manifest' && (key === 'files' || key === 'id')),
    )
    function put(key: string, v: PackValue | undefined) {
      const next = { ...obj }
      if (v === undefined) delete next[key]
      else next[key] = v
      onChange(next)
    }
    return (
      <details
        id={`pack-field${path}`}
        open={expanded}
        onToggle={(e) => setExpanded(e.currentTarget.open)}
      >
        <summary>
          {label}
          {typeof obj.slug === 'string' && obj.slug
            ? ` · ${obj.slug}`
            : typeof obj.id === 'string' && obj.id
              ? ` · ${obj.id}`
              : ''}
        </summary>
        {expanded && (
          <Stack gap="xs" pl="md">
            {entries.map(([key, child]) => (
              <Group key={key} align="start" wrap="nowrap">
                <div style={{ flex: 1, minWidth: 0 }}>
                  {nested(child, obj[key], key, (v) => put(key, v))}
                </div>
                {obj[key] !== undefined && (
                  <Button
                    variant="subtle"
                    size="xs"
                    aria-label={t('packs.removeField', { field: fieldLabel(t, key) })}
                    onClick={() => put(key, undefined)}
                  >
                    {t('packs.remove')}
                  </Button>
                )}
              </Group>
            ))}
            {spec.type === 'map' && (
              <Group>
                <TextInput
                  label={t('packs.key')}
                  value={newKey}
                  onChange={(e) => setNewKey(e.currentTarget.value)}
                />
                <Button
                  disabled={
                    !newKey ||
                    Object.hasOwn(obj, newKey) ||
                    ['__proto__', 'constructor', 'prototype'].includes(newKey)
                  }
                  onClick={() => {
                    put(newKey, initialValue(spec.items!, schema))
                    setNewKey('')
                  }}
                >
                  {t('packs.add')}
                </Button>
              </Group>
            )}
          </Stack>
        )}
      </details>
    )
  }
  if (spec.type === 'array') {
    const items = value as PackValue[]
    if (spec.items?.ref === 'ResourceRow') {
      const fields = Object.entries(schema.definitions.ResourceRow!.properties!)
      return (
        <details
          id={`pack-field${path}`}
          open={expanded}
          onToggle={(e) => setExpanded(e.currentTarget.open)}
        >
          <summary>
            {label} ({items.length})
          </summary>
          {expanded && (
            <Stack>
              <div style={{ overflowX: 'auto' }}>
                <table>
                  <thead>
                    <tr>
                      {fields.map(([key]) => (
                        <th key={key}>{fieldLabel(t, key)}</th>
                      ))}
                      <th>{t('packs.remove')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((item, i) => (
                      <tr key={i}>
                        {fields.map(([key, child]) => (
                          <td key={key} style={{ minWidth: 120 }}>
                            {nested(
                              child,
                              (item as Record<string, PackValue>)[key],
                              key,
                              (v) => {
                                const row = { ...(item as Record<string, PackValue>) }
                                if (v === undefined) delete row[key]
                                else row[key] = v
                                onChange(items.map((old, at) => (at === i ? row : old)))
                              },
                              `${i}/${key}`,
                            )}
                          </td>
                        ))}
                        <td>
                          <Button
                            size="xs"
                            onClick={() => onChange(items.filter((_, at) => at !== i))}
                          >
                            {t('packs.remove')}
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <Button onClick={() => onChange([...items, initialValue(spec.items!, schema)])}>
                {t('packs.add')}
              </Button>
            </Stack>
          )}
        </details>
      )
    }
    return (
      <details
        id={`pack-field${path}`}
        open={expanded}
        onToggle={(e) => setExpanded(e.currentTarget.open)}
      >
        <summary>
          {label} ({items.length})
        </summary>
        {expanded && (
          <Stack gap="xs" pl="md">
            {items.map((item, i) => (
              <Stack key={i} gap="xs">
                {nested(
                  spec.items!,
                  item,
                  String(i + 1),
                  (v) => onChange(items.map((old, at) => (at === i ? (v ?? null) : old))),
                  String(i),
                )}
                <Group>
                  <Button
                    size="xs"
                    variant="subtle"
                    onClick={() => onChange(items.filter((_, at) => at !== i))}
                  >
                    {t('packs.remove')}
                  </Button>
                  <Button
                    size="xs"
                    variant="subtle"
                    disabled={i === 0}
                    onClick={() => {
                      const next = [...items]
                      ;[next[i - 1], next[i]] = [next[i]!, next[i - 1]!]
                      onChange(next)
                    }}
                  >
                    {t('packs.moveUp')}
                  </Button>
                </Group>
              </Stack>
            ))}
            <Button
              size="xs"
              variant="light"
              onClick={() => onChange([...items, initialValue(spec.items!, schema)])}
            >
              {t('packs.add')}
            </Button>
          </Stack>
        )}
      </details>
    )
  }
  if (spec.type === 'boolean')
    return (
      <Checkbox
        label={label}
        checked={value === true}
        onChange={(e) => onChange(e.currentTarget.checked)}
      />
    )
  if (spec.type === 'number')
    return (
      <NumberInput
        id={`pack-field${path}`}
        label={label}
        value={typeof value === 'number' ? value : ''}
        allowDecimal={!spec.integer}
        onChange={(v) => onChange(typeof v === 'number' ? v : 0)}
      />
    )
  if (spec.type === 'reference')
    return (
      <Stack gap={2}>
        <Select
          label={label}
          searchable
          clearable
          data={[...new Set([...references, String(value)])].filter(Boolean)}
          value={String(value)}
          onChange={(v) => onChange(v ?? '')}
        />
        <TextInput
          id={`pack-field${path}`}
          aria-label={t('packs.reference')}
          value={String(value)}
          onChange={(e) => onChange(e.currentTarget.value)}
        />
      </Stack>
    )
  const options =
    name === 'op'
      ? path.includes('/effects/')
        ? ['grant', 'add', 'max', 'set']
        : [
            'constant',
            'read',
            'add',
            'subtract',
            'multiply',
            'min',
            'max',
            'floor-div',
            'ceil-div',
            'eq',
            'gte',
            'lte',
            'and',
            'or',
            'not',
          ]
      : name === 'rounding'
        ? ['floor', 'ceil']
        : name === 'combine'
          ? ['max', 'sum']
          : name === 'operation'
            ? ['all', 'amount', 'budget']
            : name === 'selection'
              ? ['known', 'prepared', 'spellbook']
              : null
  if (options)
    return (
      <Select
        id={`pack-field${path}`}
        label={label}
        value={String(value)}
        data={[...new Set([...options, String(value)])]
          .filter(Boolean)
          .map((option) => ({ value: option, label: optionLabel(t, option) }))}
        onChange={(v) => onChange(v ?? '')}
      />
    )
  return (
    <Textarea
      id={`pack-field${path}`}
      label={label}
      minRows={1}
      value={String(value)}
      onChange={(e) => onChange(e.currentTarget.value)}
    />
  )
}
