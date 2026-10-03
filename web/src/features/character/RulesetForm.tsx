import type { Change } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Button, Group, Stack, Text } from '@/ui'

export interface RulesetFormProps {
  pending: boolean
  selected?: boolean
  onSubmit: (changes: Change[]) => void
}

/** The one ruleset this application serves. */
const RULESET_2014 = '2014'

/**
 * Which rules the character is built under.
 *
 * One option today -- the 2014 rules are the only compendium served. It starts
 * selected, but the player confirms it before moving on, like any other choice.
 * The settled block is then locked because the server treats it as final.
 */
export function RulesetForm({ pending, selected = false, onSubmit }: RulesetFormProps) {
  const t = useT()

  return (
    <Stack gap="md">
      <Button
        variant="filled"
        aria-pressed="true"
        disabled={pending}
      >
        {t('ruleset.2014')}
      </Button>
      {!selected && (
        <Group>
          <Button
            loading={pending}
            onClick={() => onSubmit([{ path: 'identity.ruleset', op: 'set', value: { kind: 'slug', slug: RULESET_2014 } }])}
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
