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

  return (
    <Stack gap="md">
      <Button
        variant="filled"
        aria-pressed="true"
        disabled={pending}
      >
        {ruleset === RULESET_2014 ? t('ruleset.2014') : ruleset}
      </Button>
      {!selected && (
        <Group>
          <Button
            loading={pending}
            onClick={() => onSubmit([{ path: 'identity.ruleset', op: 'set', value: { kind: 'slug', slug: ruleset } }])}
          >
            {t('answer.confirm')}
          </Button>
        </Group>
      )}
      <Text size="xs" c="dimmed">
        {t('ruleset.final')}
      </Text>
    </Stack>
  )
}
