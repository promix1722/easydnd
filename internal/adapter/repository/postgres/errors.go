package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// SQLSTATE codes this adapter reacts to. Everything else is a server error.
const (
	sqlstateUniqueViolation     = "23505"
	sqlstateForeignKeyViolation = "23503"
)

// Constraint names the error mapping depends on.
//
// They are declared explicitly in 00001_accounts.sql rather than left to
// Postgres' defaults, because these strings are the only thing that tells
// "that account already exists" apart from "that passkey is already
// registered" -- two different messages the client is shown verbatim. A future
// ALTER TABLE ... RENAME CONSTRAINT would silently turn a 400 into a 500.
const (
	constraintUsersPK           = "users_pkey"
	constraintUserCredentialsPK = "user_credentials_pkey"
	constraintUserIdentitiesPK  = "user_identities_pkey"
	constraintGroupsPK          = "groups_pkey"
	constraintGroupMembersPK    = "group_members_pkey"

	// The membership -> group foreign key. Mapped, unlike its sibling on
	// user_id: adding somebody to a group that is not there is a 404 the
	// caller can act on, whereas adding somebody with no users row means the
	// usecase failed to materialise one, which is a bug and belongs in the
	// 500 log rather than in a polite 400.
	constraintGroupMembersGroupFK = "group_members_group_id_fkey"
)

// group_members_one_owner_idx deliberately has no constant here.
//
// A unique violation on it means two owners were written, which is not
// something a client did wrong -- it is this package or the usecase above it
// having a bug. Mapping it to a 400 would hand the caller a polite message
// about a broken invariant; leaving it unmapped puts it in the 500 log, which
// is where somebody will actually see it.

// isUniqueViolation reports whether err is a unique-constraint violation on the
// named constraint.
//
// errors.As rather than a type assertion because pgx wraps: the error arriving
// from a Query inside a transaction is several layers deep, and errorlint
// rejects the comparison forms that would miss it.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == sqlstateUniqueViolation && pgErr.ConstraintName == constraint
}

// isForeignKeyViolation reports whether err is a foreign-key violation on the
// named constraint. Same wrapping caveat as isUniqueViolation.
func isForeignKeyViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == sqlstateForeignKeyViolation &&
		pgErr.ConstraintName == constraint
}
