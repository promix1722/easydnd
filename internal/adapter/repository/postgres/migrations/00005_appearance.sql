-- +goose Up
ALTER TABLE users
 ADD COLUMN palette text NOT NULL DEFAULT 'dragon' CHECK (palette IN ('dragon', 'parchment', 'midnight', 'moss')),
 ADD COLUMN color_scheme text NOT NULL DEFAULT 'auto' CHECK (color_scheme IN ('light', 'dark', 'auto'));

-- +goose Down
ALTER TABLE users DROP COLUMN color_scheme, DROP COLUMN palette;
