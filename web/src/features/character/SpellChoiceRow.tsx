import type { Entry, Spell } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { ActionIcon, Badge, Button, ChoiceDetails, SourceTags, Group, IconPlus, IconTrash, Paper, Stack, Text, useIsDesktop } from '@/ui'
import { SpellIcon } from '@/features/spells/spellIcon'
import { SpellTags } from '@/features/spells/SpellTags'
import { SpellDetails } from '@/features/spells/SpellDetails'
import { castingTimeText, componentsAbbrev, levelText } from '@/features/spells/spellText'
import type { Choosable } from './options'

/** Separate sibling buttons inside one bordered box keep the row keyboard accessible. */
export function SpellChoiceRow({ option, spell, entries, isSelected, pending, disabled, opened, onOpen, onToggle, custom = false }: {
  custom?: boolean
  option: Choosable
  slug: string
  spell: Spell | undefined
  entries: ReadonlyMap<string, Entry>
  isSelected: boolean
  pending: boolean
  disabled: boolean
  opened: boolean
  onOpen: (open: boolean) => void
  onToggle: () => void
}) {
  const t = useT()
  const isDesktop = useIsDesktop()
  const facts = spell === undefined ? [] : [
    entries.get(spell.school ?? '')?.name,
    castingTimeText(t, spell.castingTime),
    componentsAbbrev(t, spell.components),
  ].filter(Boolean)
  const changeSelection = () => {
    onToggle()
    onOpen(false)
  }
  const removeLabel = t('list.rowAction', { label: t('common.remove'), name: option.label })
  const addLabel = t('list.rowAction', { label: t('common.add'), name: option.label })
  return (
    <Stack key={option.key} gap="xs">
      <Paper component="article" aria-label={option.label} withBorder radius="sm" px="xs" py={4} bg={isSelected ? "var(--mantine-color-blue-light)" : "var(--mantine-color-body)"}
        onFocus={(event) => event.currentTarget.scrollIntoView?.({ block: 'nearest' })}>
        <Group gap="xs" wrap="nowrap" align="center">
          <Button
            aria-label={option.label}
            aria-expanded={opened}
            variant="transparent" color="var(--mantine-color-text)"
            h="auto"
            py={4}
            px={isDesktop ? 'xs' : 0}
            justify="flex-start"
            disabled={pending}
            onClick={() => onOpen(!opened)}
            style={{ flex: 1, minWidth: 0 }}
            styles={{ label: { width: '100%', whiteSpace: 'normal' } }}
          >
            <Group gap="xs" wrap="nowrap" align="flex-start" w="100%">
              <SpellIcon icon={spell?.icon} />
              <Stack gap={2} style={{ textAlign: 'left', minWidth: 0, flex: 1 }}>
                <Group gap="xs" wrap="nowrap" justify="space-between">
                  <Group gap="xs" style={{ minWidth: 0, flex: 1 }}>
                    <Text size="sm" fw={600}>{option.label}</Text>
                    {spell?.level !== undefined && <Badge size="sm" variant="default">{levelText(t, spell.level)}</Badge>}
                    {spell !== undefined && <SpellTags spell={spell} />}
                    {custom && <Badge size="xs" color="orange" variant="light">{t('spellRules.custom')}</Badge>}
                  </Group>
                  <SourceTags provenance={spell?.provenance} rightAligned />
                </Group>
                {facts.length > 0 && <Text size="xs" c="dimmed">{facts.join(' · ')}</Text>}
                {option.reason !== undefined && <Text size="xs" c="dimmed">{option.reason}</Text>}
              </Stack>
            </Group>
          </Button>
          {isDesktop ? <Button
            size="xs" {...(isSelected ? { color: 'blue' } : {})}
            variant={isSelected ? 'subtle' : 'light'}
            aria-label={isSelected ? removeLabel : addLabel}
            disabled={pending || disabled}
            onClick={changeSelection}
          >{isSelected ? t('common.remove') : t('common.add')}</Button> : <ActionIcon
            size={44} {...(isSelected ? { color: 'blue' } : {})}
            variant={isSelected ? 'subtle' : 'light'}
            aria-label={isSelected ? removeLabel : addLabel}
            disabled={pending || disabled} onClick={changeSelection} style={{ flexShrink: 0 }}
          >{isSelected ? <IconTrash size={20} aria-hidden /> : <IconPlus size={20} aria-hidden />}</ActionIcon>}
        </Group>
      {opened && (
        <ChoiceDetails
          title={option.label}
          onBack={() => onOpen(false)}
          actions={isSelected
            ? <Button variant="default" aria-label={removeLabel} disabled={pending || disabled} onClick={changeSelection}>{t('common.remove')}</Button>
            : <Button disabled={pending || disabled} onClick={changeSelection}>{t('common.add')}</Button>}
          mobileSummary={<Group gap="xs">
            <SpellIcon icon={spell?.icon} />
            {spell?.level !== undefined && <Badge variant="default">{levelText(t, spell.level)}</Badge>}
            {spell !== undefined && <SpellTags spell={spell} />}
          </Group>}
        >
          {spell !== undefined && <SpellDetails spell={spell} entries={entries} />}
        </ChoiceDetails>
      )}
      </Paper>
    </Stack>
  )
}
