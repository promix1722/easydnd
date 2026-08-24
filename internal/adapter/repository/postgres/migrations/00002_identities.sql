-- +goose Up
-- +goose StatementBegin

-- One external account linked to one of ours.
--
-- The PRIMARY KEY is (provider, subject) rather than a surrogate, for the same
-- reason user_credentials keys on the credential id: it gives ByIdentity -- the
-- lookup every federated sign-in makes -- its index, and it enforces "a
-- provider's subject belongs to exactly one account" in the database rather
-- than in a Go map only one process can see. It is composite because a subject
-- is only unique within its issuer; keyed on the subject alone, one provider's
-- subject could resolve to an account linked through another, which is a
-- sign-in as the wrong person.
--
-- email is stored but is deliberately NOT unique and never a lookup key. An
-- address can be released by one person and reassigned to another, so matching
-- on one would eventually hand somebody else's party to a stranger. It is here
-- to be shown on the account screen and to answer support questions.
CREATE TABLE user_identities (
    provider       text        NOT NULL,
    subject        text        NOT NULL,
    user_id        text        NOT NULL,
    email          text        NOT NULL DEFAULT '',
    email_verified boolean     NOT NULL DEFAULT false,
    -- The provider's name for the person at link time. The account keeps its
    -- own display_name; this is informational, so that a rename upstream
    -- cannot silently rewrite what we show.
    display_name   text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL,
    -- NULL means "never used", which the adapter maps to the zero time.
    last_used_at   timestamptz,

    CONSTRAINT user_identities_pkey PRIMARY KEY (provider, subject),
    CONSTRAINT user_identities_provider_len CHECK (char_length(provider) > 0),
    CONSTRAINT user_identities_subject_len CHECK (char_length(subject) > 0),
    CONSTRAINT user_identities_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Loading an account reads its identities by user_id. Without this index that
-- query is a sequential scan of every linked account in the system.
CREATE INDEX user_identities_user_id_idx ON user_identities (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_identities;
-- +goose StatementEnd
