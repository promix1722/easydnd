// Package migrations carries the account schema's history as embedded SQL.
//
// Shipping the schema inside the binary is what makes a release self-contained:
// there is no migration step for a deploy to forget and no chance of a binary
// meeting a schema it was not built for, because they travel together.
package migrations

import "embed"

// FS holds the goose migrations.
//
// The .sql files sit at the ROOT of this FS on purpose. goose globs "*.sql" at
// the root of whatever fs.FS it is given and nowhere else, so moving them into
// a subdirectory does not produce a warning -- it produces ErrNoMigrations, and
// a server that starts happily against an empty database.
//
//go:embed *.sql
var FS embed.FS
