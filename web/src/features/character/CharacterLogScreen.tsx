import { Link, useParams } from 'react-router'

import { getEvents } from '@/lib/api'
import type { CharacterEvent } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useLocale, useT } from '@/lib/i18n'

import { describeChange, eventLabel, formatAt } from './labels'
import {
  Anchor,
  Badge,
  Button,
  Code,
  DataList,
  Group,
  Page,
  pageState,
  Stack,
  Text,
} from '@/ui'

import { resolveRefNames } from './refNames'
import { leafAnswers } from './settled'

import {
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
  const t = useT()
  const locale = useLocale()
  const { id = '' } = useParams()
  const log = useResource<LogView>(`log:${id}`, async (signal) => {
    const { events } = await getEvents(id, signal)
    return { events, names: await resolveRefNames(events) }
  })

  const state = pageState(log, {
    title: t('log.loadFailed'),
    fallback: t('error.unknown'),
    onRetry: log.reload,
  })

  /*
   * Two crumbs, not three -- `Characters / Event log`, with no character name
   * between them.
   *
   * Every other detail page names the thing it is about. This one cannot, and
   * the reason is the same one the whole screen is built on: it never asks for
   * the sheet, because the sheet is a projection of this log and a broken
   * projection is the exact circumstance in which somebody opens the log. The
   * name lives on the sheet. Fetching it for a breadcrumb would reintroduce
   * the dependency this page exists without, so the trail says less instead.
   *
   * `/characters/:id/build` is the asymmetry worth noting: it holds the sheet
   * already, so it gets the full three-crumb trail.
   */
  const trail = [{ label: t('log.title') }]

  if (state.kind !== 'ready' || log.data === null) {
    return (
      <Page
        trail={trail}
        state={state.kind === 'loading' ? { ...state, what: t('log.loading') } : state}
      />
    )
  }

  const { events, names } = log.data

  return (
    <Page
      trail={trail}
      subtitle={t('log.subtitle', { count: events.length })}
      actions={
        <Anchor component={Link} to={`/characters/${id}`}>
          <Button variant="subtle">{t('log.backToSheet')}</Button>
        </Anchor>
      }
    >
      <Stack gap="lg">
        <DataList
          items={events}
          getKey={(event) => String(event.seq ?? 0)}
          empty={t('log.empty')}
          // A mark on the row rather than part of its name, which is what lets
          // it ride beside the name at both widths without being built inside
          // the cell that draws it.
          badges={(event) =>
            event.level !== undefined && event.level > 0 ? (
              <Badge size="xs" variant="light">
                {t('block.level', { level: event.level })}
              </Badge>
            ) : null
          }
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
              header: t('log.event'),
              primary: true,
              // The log is a record rather than a way in: an entry opens onto
              // nothing, so there is no `to`.
              text: (event) => eventLabel(t, event.type),
              render: (event) => (
                <Text size="sm" fw={500}>
                  {eventLabel(t, event.type)}
                </Text>
              ),
            },
            {
              key: 'at',
              header: t('log.when'),
              render: (event) => (
                <Text size="sm" c="dimmed">
                  {formatAt(locale, event.at)}
                </Text>
              ),
            },
            {
              key: 'detail',
              header: t('log.detail'),
              // The one column in the app that is not a value. `EventDetail` is
              // a stack of `<Code>` lines and labelled answers, so it gets a
              // full-width line of its own on a phone rather than being joined
              // to the others with a middle dot.
              slot: 'block',
              render: (event) => <EventDetail event={event} names={names} />,
            },
          ]}
        />
      </Stack>
    </Page>
  )
}

function EventDetail({ event, names }: { event: CharacterEvent; names: Map<string, string> }) {
  const t = useT()
  const changes = event.changes ?? []
  const choices = leafAnswers(event.choices ?? [])
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
        <Code key={`${change.path}:${index}`}>{describeChange(t, change)}</Code>
      ))}
      {note !== '' && <Text size="sm">{note}</Text>}
    </Stack>
  )
}
