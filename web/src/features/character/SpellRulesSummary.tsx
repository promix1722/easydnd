import type { SpellRule } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, Group, Paper, Stack, Text } from '@/ui'
import { levelText } from '@/features/spells/spellText'
import { refName } from './refNames'
import { spellChoiceName } from './promptNames'

/** One compact explanation per source, using the current level's total. */
export function SpellRulesSummary({ rules, cantripsOnly, names, count, total, extra = 0, onChangeLimit, pending }: {
  rules: readonly SpellRule[]
  cantripsOnly: boolean
  names: ReadonlyMap<string, string>
  count: number
  total: number
  extra?: number
  onChangeLimit?: ((delta: number) => void) | undefined
  pending?: boolean
}) {
  const t = useT()
  const relevant = rules.filter((rule) => rule.purpose !== 'custom-limit' && rule.purpose !== 'replace' && rule.purpose !== 'forget' && (cantripsOnly ? rule.minLevel === 0 : rule.maxLevel > 0))
  const groups = new Map<string, SpellRule[]>()
  for (const rule of relevant) {
    const group = groups.get(rule.source) ?? []
    const same = group.find((item) => item.purpose === rule.purpose && !item.automatic?.length && !rule.automatic?.length)
    if (same === undefined) group.push({ ...rule })
    else {
      same.count += rule.count
      same.minLevel = Math.min(same.minLevel, rule.minLevel)
      same.maxLevel = Math.max(same.maxLevel, rule.maxLevel)
      if (rule.maxLevelCount !== undefined) same.maxLevelCount = rule.maxLevelCount
      same.classLevel = Math.max(same.classLevel ?? 0, rule.classLevel ?? 0)
    }
    groups.set(rule.source, group)
  }
  return <Paper component="section" withBorder p="sm" radius="sm" aria-label={t('spellRules.title')}>
    <Stack gap="xs">
      <Text fw={600}>{t('spellRules.title')}</Text>
      {[...groups].map(([source, group]) => {
        const classLevel = Math.max(...group.map((rule) => rule.classLevel ?? 0))
        const hasClass = group.some((rule) => rule.class !== undefined)
        const details = group.map((rule) => {
          if (rule.automatic?.length) return t('spellRules.automatic', { spells: rule.automatic.map((slug) => refName(`spell:${slug}`, names)).join(', ') })
          let allowance = t('spellRules.totalAllowance', {
            choice: spellChoiceName(t, rule.purpose || (rule.maxLevel === 0 ? 'cantrip' : 'known'), rule.count),
            levels: rule.minLevel === rule.maxLevel ? levelText(t, rule.minLevel) : t('spellRules.levelRange', { min: rule.minLevel, max: rule.maxLevel }),
          })
          if (rule.maxLevelCount !== undefined) allowance += t('spellRules.maxLevelCount', { level: rule.maxLevel, count: rule.maxLevelCount })
          const lists = rule.listClasses?.filter((slug) => `class:${slug}` !== source) ?? []
          return lists.length === 0 ? allowance : t('spellRules.sourceList', { allowance, classes: lists.map((slug) => refName(`class:${slug}`, names)).join(', ') })
        })
        return <Stack key={source} gap={2}>
          <Text size="sm" fw={600}>{hasClass && classLevel > 0
            ? t('spellRules.sourceLevel', { source: refName(source, names), level: classLevel }) : refName(source, names)}</Text>
          <Text size="sm" c="dimmed">
            {details.join('; ')}
            {!hasClass && group.some((rule) => !rule.automatic?.length) && <> {t('spellRules.oneTime')}</>}
          </Text>
        </Stack>
      })}
      {extra > 0 && <Stack gap={2}>
        <Text size="sm" fw={600}>{t('spellRules.extraTitle')}</Text>
        <Text size="sm" c="dimmed">{t('spellRules.extra', { count: extra })}</Text>
      </Stack>}
      <Group justify="space-between" align="center" mt="xs">
        <Text size="sm" aria-live="polite">{t('prompt.spellsSelected', { count, total })}</Text>
        {onChangeLimit !== undefined && <Group gap={4} style={{ marginLeft: 'auto' }}>
          <Text size="xs" c="dimmed" mr={4}>{t('spellRules.changeLimit')}</Text>
          <Button size="xs" variant="light" color="gray" px="xs" aria-label={t('spellRules.decrease')}
            disabled={pending === true || extra <= 0} onClick={() => onChangeLimit(-1)}>−</Button>
          <Button size="xs" variant="light" color="gray" px="xs" aria-label={t('spellRules.increase')}
            disabled={pending === true || extra >= 1000} onClick={() => onChangeLimit(1)}>+</Button>
        </Group>}
      </Group>
    </Stack>
  </Paper>
}
