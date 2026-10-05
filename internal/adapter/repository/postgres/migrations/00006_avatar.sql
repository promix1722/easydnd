-- +goose Up
ALTER TABLE users ADD COLUMN image text NOT NULL DEFAULT '' CHECK (octet_length(image) <= 262144);

-- +goose Down
ALTER TABLE users DROP COLUMN image;
