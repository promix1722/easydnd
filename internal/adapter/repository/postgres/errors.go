package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// SQLSTATE codes this adapter reacts to. Everything else is a server error.
const (
	sqlstateUniqueViolation = "23505"
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
)

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
