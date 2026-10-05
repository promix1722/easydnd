# Long polling

How the AI Wizard page learns that a running import has moved. One request is
held by the server until the session changes or a second has passed, and the
browser sends the next one the moment it is answered. There is no stream, no
timer in the browser and nothing to configure in a proxy.

For what the wizard does, see [agent.md](agent.md). This page is only the
transport.

## Why not a stream

The page used to have three feeds for one session: a first `GET`, a
Server-Sent Events stream, and a snapshot re-read every three seconds in case
the stream had been lost. The stream was itself a loop on the server that
re-read the session four times a second, and it only worked if every proxy
between the browser and the API agreed not to buffer it -- hence a
no-buffering header, heartbeats, and a per-request override of the write
deadline. Behind a proxy that buffered anyway, the page was left with the
three-second re-read, and the chat arrived in three-second steps.

A request that is answered within a second is an ordinary request to every
proxy there is. That is the whole argument.

## The request

`GET /v1/agent-sessions/:id?revision=R&after=N`

`R` is the session `revision` the page holds and `N` the id of its last event.
Event ids are `1..n` and events are only ever appended, so `N` is also how
many the page has. Without the query the route is what it always was -- the
whole session, at once -- and that is what the page opens with.

| Answer | When | Body |
| --- | --- | --- |
| `200` | the revision is not `R`, or `N` is not a cursor into this session | `{"session": {...}}`, the whole session |
| `200` | same revision, events past `N` | `{"events": [...]}`, only those |
| `204` | nothing differed for one second | none |
| `404` `agent.notFound` | the chat was discarded, or is another account's | the usual error envelope |

All of them carry `Cache-Control: no-store`.

**Why two kinds of `200`.** A revision moves on a change of status -- a turn
starting, ending, failing, a control being accepted -- and a reader that is
behind on one of those is behind on more than events, so it is sent everything.
Between those, a running turn appends an event per streamed chunk, and
answering each with only the tail is what keeps the requests small while the
assistant is typing.

**Why one second.** It is far inside every timeout on the path: the server's
15 s write timeout and nginx's 60 s `proxy_read_timeout`. A held poll therefore
needs no deadline override and no heartbeat, and `deploy/nginx/easydnd.conf`
does not mention it. The wait is the constant `agentPollWait` in
`internal/api/http/v1/character/agent.go`, not a configuration key: nothing
about a deployment changes what it should be. Its cost is one request a second
from each open, visible, unfinished chat.

## The server side

`Agent.Wait` in `internal/usecase/agent/agent.go` is `Get` for a reader
that says what it already has. It compares the published view with the cursor
under the read lock, copying nothing unless there is something to return, and
otherwise sleeps on `changed` -- a channel that `publish` closes and replaces
every time any session's view changes, and that discarding a chat closes too.
Closing a channel wakes every sleeper at once; each rechecks its own session
and goes back to sleep if the change was somebody else's.

Readers never take the coordinator's lock, for the reason given in
[agent.md](agent.md#session-lifetime).

`changed` lives in the memory of one process. That is one of the reasons the
wizard [does not scale horizontally](known-caveats.md).

**The access log.** `RequestLogger` writes one line per request, which for an
idle poll is one line a second per tab saying nothing. A `GET` answered `204`
is logged at Debug instead; every other answer is logged as before.

## The browser side

`AgentImportScreen` runs one loop per open chat:

1. Read the whole session once.
2. Send what it holds; wait for the answer.
3. Apply it with `merge` -- a whole session replaces the one held unless it is
   older, a tail is appended past the events already there -- and go to 2.

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
  retried after a second, so a server that is down is asked once a second and
  not in a tight loop.
- **Leaving the page** -- the request in flight is aborted.

## Tests

`TestImportHTTPUploadResumeOwnershipAndLongPoll` drives every row of the table
above over real HTTP, including a poll woken early by a change and one ended
by a discard. `ImportCharacterScreen.test.tsx` holds the poll in its `fetch`
stub and answers it from the test.
