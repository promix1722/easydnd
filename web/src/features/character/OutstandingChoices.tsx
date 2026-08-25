import type { Prompt } from '@/lib/api'
import { Badge, Group, Stack, Text } from '@/ui'

import { choiceName } from './promptNames'
import { refName } from './refNames'

export interface OutstandingChoicesProps {
  prompts: readonly Prompt[]
  /** Compendium names for the entries the prompts hang off. */
  names?: ReadonlyMap<string, string>
}

/**
 * What a character still has to decide, said rather than offered.
 *
 * The sheet's statement of what is left, beside the link that goes and
 * answers it -- and the level-up page's, when there is one. The build screen
 * draws the same choices as blocks you can open, because there they are ways
 * in; here they are a list, and a list of things you cannot press should not
 * look like a list of things you can.
 *
 * What the two surfaces share is everything that matters: the server answers
 * `/prompts` with exactly the questions the character is being asked, and both
 * name them through `choiceName`. There is deliberately no second notion of
 * "outstanding" anywhere in this client, and no second vocabulary for it, so
 * there is no way for the sheet and the build screen to disagree.
 *
 * Nothing here can be answered before it is asked, because there is nothing
 * here that was not asked.
 */
export function OutstandingChoices({ prompts, names }: OutstandingChoicesProps) {
  return (
    <Stack gap={4} align="stretch">
      {prompts.map((prompt) => (
        <Group key={prompt.choice.prompt} gap={8} wrap="nowrap" justify="space-between" w="100%">
          <Text size="sm" style={{ whiteSpace: 'normal', textAlign: 'left' }}>
            {choiceName(prompt)}
            {prompt.source !== undefined && (
              <Text span size="xs" c="dimmed">
                {' '}
                · from {refName(prompt.source, names ?? new Map())}
              </Text>
            )}
          </Text>
          {prompt.optional && (
            <Badge size="xs" variant="light" color="gray">
              optional
            </Badge>
          )}
        </Group>
      ))}
    </Stack>
  )
}
