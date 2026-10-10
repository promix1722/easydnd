import { useState } from 'react'
import { useRulesEdition } from '@/lib/api/catalogScope'

import type { Change } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, Group, Stack, Text } from '@/ui'

export interface RulesetFormProps {
  pending: boolean
  selected?: boolean
  onSubmit: (changes: Change[]) => void
}

/** The default SRD rules edition. */
const RULESET_2014 = '2014'

/**
 * Which rules the character is built under.
 *
 * The selected core pack determines the rules edition. The player confirms
 * it before moving on, like any other choice.
 * The settled block is then locked because the server treats it as final.
 */
export function RulesetForm({ pending, selected = false, onSubmit }: RulesetFormProps) {
  const t = useT()
  const ruleset = useRulesEdition()
  const [picked, setPicked] = useState(true)

  return (
    <Stack gap="md">
      <Button
        variant={picked ? 'light' : 'default'}
        aria-pressed={picked}
        justify="space-between"
        h="auto"
        py="xs"
        disabled={pending || selected}
        onClick={() => setPicked(true)}
      >
        <Text size="sm" style={{ whiteSpace: 'normal', textAlign: 'left' }}>
          {ruleset === RULESET_2014 ? t('ruleset.2014') : ruleset}
        </Text>
      </Button>
      {!selected && (
        <Group>
          <Button
            loading={pending}
            disabled={!picked || pending}
            onClick={() => onSubmit([{ path: 'identity.ruleset', op: 'set', value: { kind: 'slug', slug: ruleset } }])}
          >
            {picked ? t('answer.confirm') : t('prompt.chooseMore', { count: 1 })}
          </Button>
          {picked && <Button variant="subtle" disabled={pending} onClick={() => setPicked(false)}>
            {t('prompt.clear')}
          </Button>}
        </Group>
      )}
      <Text size="xs" c="dimmed">
        {t('ruleset.final')}
      </Text>
    </Stack>
  )
}
