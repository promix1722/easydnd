// Package postgres is the durable store.
//
// It implements every repository port -- accounts, groups, characters,
// folders, shared pools, games, rule packs and AI Wizard chats -- against
// PostgreSQL, which in production means an AWS RDS instance. Its in-memory
// sibling, internal/adapter/repository/memory, is the development fallback
// and nothing else.
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
