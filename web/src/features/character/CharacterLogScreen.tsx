import { Link, useParams } from 'react-router'

import { getEvents } from '@/lib/api'
import type { Answer, CharacterEvent } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import {
  Alert,
  Anchor,
  Badge,
  Button,
  Code,
  DataList,
  Group,
  Loader,
  Stack,
  Text,
  Title,
} from '@/ui'

import { resolveRefNames } from './refNames'

import {
  describeChange,
  eventLabel,
  formatAt,
  kindOf,
  pickLabel,
  promptLabel,
  slugOf,
  titleCase,
} from '@/domain'

interface LogView {
  events: CharacterEvent[]
  /** A reference to the compendium's name for it, where the compendium has one. */
  names: Map<string, string>
}

/**
 * The character's event log: the record, in the order it was written.
 *
 * This is the page the event sourcing was for. docs/dnd.md justifies the log
 * by saying it makes "why do I have this proficiency?" answerable, and until
 * something drew it that claim could not be checked from a browser.
 *
 * It deliberately does *not* fetch the sheet -- not even for the character's
 * name in the header. The sheet is a projection of this log, and a broken
 * projection is the exact circumstance in which somebody opens the log. A page
 * that fails whenever the thing it exists to diagnose fails is no use.
 */
export function CharacterLogScreen() {
  const { id = '' } = useParams()
  const log = useResource<LogView>(`log:${id}`, async (signal) => {
    const { events } = await getEvents(id, signal)
    return { events, names: await resolveRefNames(events) }
  })

  if (log.loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Reading the log...
        </Text>
      </Group>
    )
  }
  if (log.error !== null || log.data === null) {
    return (
      <Alert color="red" title="Could not load this log">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{log.error ?? 'Unknown error'}</Text>
          <Button variant="light" onClick={log.reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const { events, names } = log.data

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="flex-start">
        <div>
          <Title order={2}>Event log</Title>
          <Text c="dimmed" size="sm">
            {events.length === 1 ? '1 event' : `${events.length} events`} · the record the sheet is
            projected from
          </Text>
        </div>
        <Anchor component={Link} to={`/characters/${id}`}>
          <Button variant="subtle">Back to sheet</Button>
        </Anchor>
      </Group>

      <DataList
        items={events}
        getKey={(event) => String(event.seq ?? 0)}
        empty="Nothing recorded yet."
        columns={[
          {
            key: 'seq',
            header: '#',
            render: (event) => (
              <Text size="sm" c="dimmed">
                {event.seq ?? '--'}
              </Text>
            ),
          },
          {
            key: 'event',
            header: 'Event',
            primary: true,
            render: (event) => (
              <Group gap={6}>
                <Text size="sm" fw={500}>
                  {eventLabel(event.type)}
                </Text>
                {event.level !== undefined && event.level > 0 && (
                  <Badge size="xs" variant="light">
                    Level {event.level}
                  </Badge>
                )}
              </Group>
            ),
          },
          {
            key: 'at',
            header: 'When',
            render: (event) => (
              <Text size="sm" c="dimmed">
                {formatAt(event.at)}
              </Text>
            ),
          },
          {
            key: 'detail',
            header: 'Detail',
            render: (event) => <EventDetail event={event} names={names} />,
          },
        ]}
      />
    </Stack>
  )
}

/**
 * Drops the answer that only says which branch a nested prompt took.
 *
 * A nested option is named by the prompt it opens, and that prompt's own
 * answer is in the same event -- so keeping both prints the choice twice, once
 * saying "Expertise" and once saying which two skills.
 */
function answersWorthShowing(choices: readonly Answer[]): Answer[] {
  const answered = new Set(choices.map((answer) => answer.prompt))
  return choices.filter(
    (answer) => answer.picks.length === 0 || !answer.picks.every((pick) => answered.has(pick)),
  )
}

function EventDetail({ event, names }: { event: CharacterEvent; names: Map<string, string> }) {
  const changes = event.changes ?? []
  const choices = answersWorthShowing(event.choices ?? [])
  const note = event.note ?? ''
  if (event.ref === undefined && choices.length === 0 && changes.length === 0 && note === '') {
    return (
      <Text size="sm" c="dimmed">
        --
      </Text>
    )
  }

  return (
    <Stack gap={4} align="flex-start">
      {event.ref !== undefined && (
        <Group gap={6}>
          <Badge size="xs" variant="light">
            {kindOf(event.ref)}
          </Badge>
          <Text size="sm" fw={500}>
            {names.get(event.ref) ?? titleCase(slugOf(event.ref))}
          </Text>
        </Group>
      )}
      {choices.map((answer) => (
        <div key={answer.prompt}>
          <Text size="xs" c="dimmed" tt="uppercase">
            {promptLabel(answer.prompt)}
          </Text>
          <Text size="sm">{answer.picks.map(pickLabel).join(', ')}</Text>
        </div>
      ))}
      {changes.map((change, index) => (
        <Code key={`${change.path}:${index}`}>{describeChange(change)}</Code>
      ))}
      {note !== '' && <Text size="sm">{note}</Text>}
    </Stack>
  )
}
