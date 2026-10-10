-- +goose Up
-- Who may look at a character is not a fact about how it was built, so it sits
-- beside folder_id rather than in the log.
ALTER TABLE characters ADD COLUMN public boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE characters DROP COLUMN public;
