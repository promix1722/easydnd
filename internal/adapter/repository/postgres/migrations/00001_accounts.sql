-- +goose Up
-- +goose StatementBegin

-- An account id is base64url of 16 random bytes and doubles as the WebAuthn
-- user handle, so it is opaque text rather than a uuid: the value stored on the
-- authenticator is that exact string, and re-encoding it is how a passkey gets
-- orphaned.
CREATE TABLE users (
    id           text        NOT NULL,
    display_name text        NOT NULL,
    created_at   timestamptz NOT NULL,

    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_display_name_len CHECK (char_length(display_name) BETWEEN 1 AND 64)
);

-- One registered passkey.
--
-- The credential id is the PRIMARY KEY rather than a surrogate. That is what
-- gives ByCredentialID -- the lookup every usernameless sign-in makes -- its
-- index, and it enforces "a credential belongs to exactly one account" in the
-- database rather than in a Go map that only one process can see.
CREATE TABLE user_credentials (
    id               bytea       NOT NULL,
    user_id          text        NOT NULL,
    public_key       bytea       NOT NULL,
    attestation_type text        NOT NULL DEFAULT '',
    transports       text[]      NOT NULL DEFAULT '{}',
    aaguid           bytea       NOT NULL DEFAULT '\x'::bytea,

    -- SignCount is a uint32 in the domain and Postgres has no unsigned types.
    -- bigint, not integer: 4294967295 overflows a signed 32-bit column, so an
    -- authenticator reporting a high counter would fail to store rather than
    -- fail to verify. The CHECK is what keeps the uint32() cast on the read
    -- path honest -- without it a hand-written UPDATE could wrap silently.
    sign_count       bigint      NOT NULL DEFAULT 0
                     CONSTRAINT user_credentials_sign_count_uint32
                     CHECK (sign_count >= 0 AND sign_count <= 4294967295),

    backup_eligible  boolean     NOT NULL DEFAULT false,
    backup_state     boolean     NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL,
    -- NULL means "never asserted", which the adapter maps to the zero time.
    last_used_at     timestamptz,

    CONSTRAINT user_credentials_pkey PRIMARY KEY (id),
    CONSTRAINT user_credentials_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- ByID loads an account and then its credentials. Without this index that
-- second query is a sequential scan of every passkey in the system.
CREATE INDEX user_credentials_user_id_idx ON user_credentials (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_credentials;
DROP TABLE users;
-- +goose StatementEnd
