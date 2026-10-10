# Polling

How the AI Wizard page learns that a running import has moved. Once a second
it asks the server what it lacks, and is answered at once from the database.
There is no stream, no held request and nothing to configure in a proxy.

For what the wizard does, see [agent.md](agent.md). This page is only the
transport.

## Why nothing cleverer

The page has had three transports before this one.

First, three feeds for one session: a first `GET`, a Server-Sent Events
stream, and a snapshot re-read every three seconds in case the stream had
been lost. The stream only worked if every proxy between the browser and the
API agreed not to buffer it -- hence a no-buffering header, heartbeats and a
per-request override of the write deadline -- and behind one that buffered
anyway the chat arrived in three-second steps.

Then a long poll: one request held by the server for up to a second and
answered early when the session changed. The early answer came from a channel
in the memory of the process running the turn, which was the whole of the
session's state at the time.

The session is now [in PostgreSQL](agent.md#session-lifetime), so that any
API process can answer for it. A held request on a process that is not
running the turn has nothing to be woken by, and would have to ask the
database on a timer of its own -- a poll inside a poll. The page asking once
a second is the same thing with the pretence removed.

Its cost is latency: a streamed reply appears in one-second steps, and the
end of a turn is seen up to a second late. The page already types the
assistant's words out a message at a time (`useReveal`), so the steps are not
what the reader sees.

## The request

`GET /v1/agent-sessions/:id?revision=R&after=N`

`R` is the session `revision` the page holds and `N` the id of its last event.
Event ids are `1..n` and events are only ever appended, so `N` is also how
many the page has. Without the query the route is the whole session, which is
what the page opens with.

| Answer | When | Body |
| --- | --- | --- |
| `200` | the revision is not `R`, or `N` is not a cursor into this session | `{"session": {...}}`, the whole session |
| `200` | same revision, events past `N` | `{"events": [...]}`, only those |
| `204` | the page has everything | none |
| `404` `agent.notFound` | the chat was deleted, or is another account's | the usual error envelope |

All of them are immediate and carry `Cache-Control: no-store`.

**Why two kinds of `200`.** A revision moves on a change of status -- a turn
starting, ending, failing, a control being accepted -- and a reader that is
behind on one of those is behind on more than events, so it is sent everything.
Between those, a running turn appends an event per streamed chunk, and
answering each poll with only the tail is what keeps the answers small while
the assistant is typing.

## The server side

`Agent.Poll` in `internal/usecase/agent/agent.go` asks the store for the
session's revision and event count (`Store.Tail` -- two indexed reads, and
never the session's document) and returns the tail, the whole session, or
nothing. It takes no lock and waits for nothing.

**The access log.** `RequestLogger` writes one line per request, which for an
idle poll is one line a second per tab saying nothing. A `GET` answered `204`
is logged at Debug instead; every other answer is logged as before.

## The browser side

`AgentImportScreen` runs one loop per open chat:

1. Read the whole session once.
2. Send what it holds; apply the answer with `merge` -- a whole session
   replaces the one held unless it is older, a tail is appended past the
   events already there.
3. Wait a second (`polling.every` in `lib/api/agent.ts`) and go to 2.

The same `merge` takes the answers to the page's own writes, so a slow poll
cannot undo a newer control. The cursor is kept in a ref that moves with the
answer rather than with the next render; without that the following request
would ask for what had just arrived.

The loop stops, or holds off:

- **Hidden tab** -- it waits for the tab to be shown, then catches up in one
  request.
- **Finished chat** -- it ends. A finished chat is a record.
- **`404`** -- it ends. The chat is gone.
- **A failed request** -- the reconnecting notice appears and the request is
  retried after a second.
- **Leaving the page** -- the request in flight is aborted.

## Tests

`TestImportHTTPUploadResumeOwnershipAndPoll` drives every row of the table
above over real HTTP, including an idle poll answered at once and one that
follows a discard. `RunAgentStore` checks `Tail` on both stores.
`AgentImportScreen.test.tsx` answers the poll from its `fetch` stub, and
sets `polling.every` to zero so that no test waits for it.
