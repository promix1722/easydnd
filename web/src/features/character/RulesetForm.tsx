import { useState } from 'react'

import type { Change } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, Group, Select, Stack, Text } from '@/ui'

export interface RulesetFormProps {
  pending: boolean
  submitLabel: string
  onSubmit: (changes: Change[]) => void
}

/** The one ruleset this application serves. */
const RULESET_2014 = '2014'

/**
 * Which rules the character is built under.
 *
 * One option today -- the 2014 rules are the only compendium served -- so the
 * select is a statement wearing a control's clothes. It exists because the
 * answer is recorded on the character and is final: the server refuses any
 * later change to a different value, and the settled block is drawn locked.
 */
export function RulesetForm({ pending, submitLabel, onSubmit }: RulesetFormProps) {
  const t = useT()
  const [ruleset, setRuleset] = useState<string>(RULESET_2014)

  return (
    <Stack gap="md">
      <Select
        aria-label={t('choice.ruleset')}
        data={[{ value: RULESET_2014, label: t('ruleset.2014') }]}
        value={ruleset}
        onChange={(value) => {
          if (value !== null) setRuleset(value)
        }}
        allowDeselect={false}
      />
      <Text size="xs" c="dimmed">
        {t('ruleset.final')}
      </Text>
      <Group>
        <Button
          onClick={() =>
            onSubmit([{ path: 'identity.ruleset', op: 'set', value: { kind: 'slug', slug: ruleset } }])
          }
          loading={pending}
        >
          {submitLabel}
        </Button>
      </Group>
    </Stack>
  )
}
