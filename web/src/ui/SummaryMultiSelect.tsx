import { useRef, useState } from 'react'
import { CheckIcon, CloseButton, Combobox, Group, InputBase, ScrollArea, Text, useCombobox } from '@mantine/core'
import type { InputBaseProps } from '@mantine/core'

interface SummaryMultiSelectProps extends Pick<InputBaseProps, 'w' | 'miw' | 'maw' | 'disabled'> {
  'aria-label': string
  placeholder?: string | undefined
  data: { value: string; label: string }[]
  value: string[]
  onChange: (value: string[]) => void
  searchable?: boolean
  clearable?: boolean
}

/** A fixed-height multiselect with a comma-separated summary instead of pills. */
export function SummaryMultiSelect({ data, value, onChange, placeholder, searchable, clearable, 'aria-label': label, ...props }: SummaryMultiSelectProps) {
  const [search, setSearch] = useState('')
  const targetRef = useRef<HTMLButtonElement>(null)
  const [targetWidth, setTargetWidth] = useState(0)
  const combobox = useCombobox({
    onDropdownOpen: () => {
      setTargetWidth(targetRef.current?.getBoundingClientRect().width ?? 0)
      if (searchable) combobox.focusSearchInput()
    },
    onDropdownClose: () => { setSearch(''); combobox.resetSelectedOption() },
  })
  const summary = value.map((key) => data.find((option) => option.value === key)?.label ?? key).join(', ')
  const options = data.filter((option) => option.label.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()))
  return <Combobox store={combobox} width="max-content" position="bottom-start"
    styles={{ dropdown: { minWidth: `min(${targetWidth}px, calc(100vw - 16px))`, maxWidth: 'calc(100vw - 16px)' } }}
    onOptionSubmit={(key) => onChange(value.includes(key) ? value.filter((v) => v !== key) : [...value, key])}>
    <Combobox.Target targetType="button" withExpandedAttribute>
      <InputBase {...props} ref={targetRef} component="button" type="button" pointer role="combobox" aria-label={label}
        onClick={() => combobox.toggleDropdown()}
        rightSection={clearable && value.length > 0
          ? <CloseButton size="sm" aria-label={label} disabled={!!props.disabled} onClick={() => onChange([])} />
          : <Combobox.Chevron />}
        rightSectionPointerEvents={clearable && value.length > 0 ? 'auto' : 'none'}
        styles={{ input: { display: 'flex', alignItems: 'center', textAlign: 'left', height: 'var(--input-height)', overflow: 'hidden' } }}>
        <Text component="span" size="sm" truncate title={summary || placeholder} c={value.length ? 'inherit' : 'var(--mantine-color-placeholder)'} style={{ minWidth: 0, width: '100%' }}>
          {summary || placeholder}
        </Text>
      </InputBase>
    </Combobox.Target>
    <Combobox.Dropdown>
      {searchable && <Combobox.Search value={search} aria-label={label} onChange={(event) => {
        setSearch(event.currentTarget.value)
        combobox.resetSelectedOption()
      }} />}
      <ScrollArea.Autosize mah={240} type="auto" scrollbars="y">
        <Combobox.Options aria-multiselectable="true">
          {options.map((option) => <Combobox.Option key={option.value} value={option.value} active={value.includes(option.value)} aria-selected={value.includes(option.value)}>
            <Group gap="xs" wrap="nowrap">
              <CheckIcon size={12} aria-hidden style={{ flexShrink: 0, visibility: value.includes(option.value) ? 'visible' : 'hidden' }} />
              <Text component="span" size="sm" title={option.label} style={{ minWidth: 0, whiteSpace: 'normal', overflowWrap: 'anywhere' }}>{option.label}</Text>
            </Group>
          </Combobox.Option>)}
        </Combobox.Options>
      </ScrollArea.Autosize>
    </Combobox.Dropdown>
  </Combobox>
}
