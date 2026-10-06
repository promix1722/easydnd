-- +goose Up

-- An AI Wizard chat. It is here, and not in the memory of the process that
-- started it, so that any API process can answer for it and any can run its
-- next turn. See docs/agent.md#session-lifetime.
--
-- owner_id has no foreign key: a guest may own a chat, and a guest is an
-- account only once it asks to be named in a group.
--
-- status is the state machine's state, and the lease columns say who is
-- working on a running turn. The second CHECK is the half of that machine the
-- database holds: a row is running exactly when somebody has it.
--
-- document is everything else -- transcript, working notes, the character's
-- log as the assistant last saw it. It is json and not jsonb on purpose: it
-- is never queried, and jsonb refuses the \u0000 a transcribed source can
-- carry.
--
-- touched_at is when the chat was last used. Rows a day past it are deleted
-- by every API process's sweep.
CREATE TABLE agent_sessions (
    id          text        PRIMARY KEY,
    owner_id    text        NOT NULL,
    status      text        NOT NULL
        CONSTRAINT agent_sessions_status_check
        CHECK (status IN ('opening', 'queued', 'running', 'waiting', 'review', 'paused', 'failed')),
    revision    integer     NOT NULL,
    generation  integer     NOT NULL,
    finished    boolean     NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL,
    touched_at  timestamptz NOT NULL,
    lease_owner text,
    lease_until timestamptz,
    document    json        NOT NULL,
    CONSTRAINT agent_sessions_lease_check
        CHECK ((status = 'running') = (lease_owner IS NOT NULL AND lease_until IS NOT NULL))
);

CREATE INDEX agent_sessions_owner_idx ON agent_sessions (owner_id);
CREATE INDEX agent_sessions_claim_idx ON agent_sessions (status, lease_until);

-- What the page shows, one row per event, only ever appended: a poll reads
-- the rows past the id it holds.
CREATE TABLE agent_events (
    session_id text    NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    id         integer NOT NULL,
    body       json    NOT NULL,
    PRIMARY KEY (session_id, id)
);

-- The attached sources, read once per turn to be sent to the model.
CREATE TABLE agent_files (
    session_id text  NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    name       text  NOT NULL,
    mime       text  NOT NULL,
    data       bytea NOT NULL,
    PRIMARY KEY (session_id, name)
);

-- +goose Down
DROP TABLE agent_files;
DROP TABLE agent_events;
DROP TABLE agent_sessions;
