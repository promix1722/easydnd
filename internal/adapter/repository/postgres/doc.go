// Package postgres is the durable account store.
//
// It implements internal/domain/user.Repository against PostgreSQL, which in
// production means an AWS RDS instance. Accounts and their passkeys are the
// only aggregate kept here: characters and the folders they are filed in still
// live in internal/adapter/repository/memory, and a restart still costs a
// player every character they made.
//
// This is an outbound adapter, so it depends inward and never sideways. It
// imports the domain, internal/types and internal/config, and it does not
// import gin, net/http or internal/api.
//
// database/sql appears exactly once, in migrate.go, because goose operates on
// a *sql.DB. The query path never touches it: pgx.ErrNoRows already wraps
// sql.ErrNoRows, so errors.Is reaches it without the import.
//
// The behaviour this package must exhibit is not written here. It lives in
// internal/adapter/repository/repotest, which the in-memory adapter runs too:
// two implementations of one port that disagree about which error a bad call
// produces are two different ports wearing the same name.
package postgres
