-- +goose Up
CREATE TABLE spell_icons (
 slug text PRIMARY KEY,
 revision text NOT NULL,
 content_type text NOT NULL,
 data bytea NOT NULL
);
-- +goose Down
DROP TABLE spell_icons;
