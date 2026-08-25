package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// UserRepository is the durable account store.
//
// Every error string below is copied verbatim from the in-memory adapter, and
// that is not tidiness. internal/api/http/helpers puts a ValidationError's
// message straight into the 400 body and a NotFoundError's into the 404 body,
// so a reworded message here is a changed API response -- from a package the
// HTTP layer is not supposed to know exists.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository returns a store over the given pool.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const credentialColumns = `
	id, public_key, attestation_type, transports, aaguid,
	sign_count, backup_eligible, backup_state, created_at, last_used_at`

const identityColumns = `
	provider, subject, email, email_verified, display_name, created_at, last_used_at`

// EnsureGuest stores a guest's row if it is not already there.
//
// ON CONFLICT DO NOTHING rather than a SELECT and then an INSERT: two of a
// guest's requests can reach this at once -- accepting an invite in one tab
// while creating a group in another -- and the read-then-write version loses
// that race and returns a primary key violation to one of them.
//
// The name is written once and never refreshed, matching the in-memory
// adapter: the first join is what named them.
func (r *UserRepository) EnsureGuest(ctx context.Context, u domain.User) error {
	if u.ID == "" {
		return types.NewValidationError("account id must not be empty")
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, display_name, created_at) VALUES ($1, $2, $3)
		 ON CONFLICT ON CONSTRAINT users_pkey DO NOTHING`,
		string(u.ID), u.DisplayName, u.CreatedAt)
	if err != nil {
		return types.WrapServerError(err, "ensure guest account")
	}
	return nil
}

// Create stores u together with its initial credentials.
//
// One transaction, all or nothing. The in-memory adapter emulates this by
// checking every credential before writing any of them, for the reason its
// comment gives: a partial write leaves index entries resolving to an account
// that was never stored. Here the database enforces it.
func (r *UserRepository) Create(ctx context.Context, u domain.User) error {
	if u.ID == "" {
		return types.NewValidationError("account id must not be empty")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin create account")
	}
	// Rollback after a successful Commit is a no-op, so this needs no flag.
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		`INSERT INTO users (id, display_name, created_at) VALUES ($1, $2, $3)`,
		string(u.ID), u.DisplayName, u.CreatedAt)
	switch {
	case isUniqueViolation(err, constraintUsersPK):
		return types.NewValidationError("account %q already exists", u.ID)
	case err != nil:
		return types.WrapServerError(err, "insert account")
	}

	for _, c := range u.Credentials {
		if err := insertCredential(ctx, tx, u.ID, c); err != nil {
			return err
		}
	}
	for _, i := range u.Identities {
		if err := insertIdentity(ctx, tx, u.ID, i); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit create account")
	}
	return nil
}

// ByID returns the account with the given id.
func (r *UserRepository) ByID(ctx context.Context, id domain.ID) (domain.User, error) {
	return r.load(ctx,
		`SELECT id, display_name, created_at FROM users WHERE id = $1`,
		"account not found", string(id))
}

// ByCredentialID returns the account owning the given raw credential id.
//
// This is the lookup a usernameless sign-in depends on, and it is a single
// indexed join rather than a scan because the credential id is the primary key
// of user_credentials.
func (r *UserRepository) ByCredentialID(ctx context.Context, credentialID []byte) (domain.User, error) {
	return r.load(ctx, `
		SELECT u.id, u.display_name, u.created_at
		  FROM users u
		  JOIN user_credentials c ON c.user_id = u.id
		 WHERE c.id = $1`,
		"credential not found", credentialID)
}

// load runs a header query and then the account's credentials and identities.
//
// Both run inside one read-only transaction so that a concurrent AddCredential
// cannot produce an account whose credential list is from a different instant
// than its header.
func (r *UserRepository) load(ctx context.Context, headerSQL, missMessage string, args ...any) (domain.User, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.User{}, types.WrapServerError(err, "begin read account")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var u domain.User
	var id string
	err = tx.QueryRow(ctx, headerSQL, args...).Scan(&id, &u.DisplayName, &u.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.User{}, types.NewNotFoundError("%s", missMessage)
	case err != nil:
		return domain.User{}, types.WrapServerError(err, "read account")
	}
	u.ID = domain.ID(id)

	// ORDER BY is not decoration: without it the row order is whatever the heap
	// returns, and a caller indexing Credentials[0] gets a coin flip.
	rows, err := tx.Query(ctx,
		`SELECT`+credentialColumns+` FROM user_credentials WHERE user_id = $1 ORDER BY created_at, id`,
		id)
	if err != nil {
		return domain.User{}, types.WrapServerError(err, "read credentials")
	}
	defer rows.Close()

	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return domain.User{}, types.WrapServerError(err, "scan credential")
		}
		u.Credentials = append(u.Credentials, c)
	}
	if err := rows.Err(); err != nil {
		return domain.User{}, types.WrapServerError(err, "read credentials")
	}
	rows.Close()

	// Inside the same read-only transaction as the header and the credentials,
	// so an account cannot come back with its two lists read at different
	// instants -- the whole reason load opens a transaction at all.
	identityRows, err := tx.Query(ctx,
		`SELECT`+identityColumns+` FROM user_identities WHERE user_id = $1 ORDER BY created_at, provider, subject`,
		id)
	if err != nil {
		return domain.User{}, types.WrapServerError(err, "read identities")
	}
	defer identityRows.Close()

	for identityRows.Next() {
		i, err := scanIdentity(identityRows)
		if err != nil {
			return domain.User{}, types.WrapServerError(err, "scan identity")
		}
		u.Identities = append(u.Identities, i)
	}
	if err := identityRows.Err(); err != nil {
		return domain.User{}, types.WrapServerError(err, "read identities")
	}
	return u, nil
}

// ByIdentity returns the account holding the given external identity.
//
// The lookup a federated sign-in depends on, and a single indexed join rather
// than a scan because (provider, subject) is the primary key of
// user_identities.
func (r *UserRepository) ByIdentity(ctx context.Context, provider domain.Provider, subject string) (domain.User, error) {
	return r.load(ctx, `
		SELECT u.id, u.display_name, u.created_at
		  FROM users u
		  JOIN user_identities i ON i.user_id = u.id
		 WHERE i.provider = $1 AND i.subject = $2`,
		"identity not found", string(provider), subject)
}

// AddIdentity links an external identity to an existing account.
func (r *UserRepository) AddIdentity(ctx context.Context, id domain.ID, i domain.Identity) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin add identity")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := requireAccount(ctx, tx, id); err != nil {
		return err
	}
	if err := insertIdentity(ctx, tx, id, i); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit add identity")
	}
	return nil
}

// TouchIdentity records a successful federated sign-in.
func (r *UserRepository) TouchIdentity(ctx context.Context, id domain.ID, i domain.Identity) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin touch identity")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := requireAccount(ctx, tx, id); err != nil {
		return err
	}

	// Four columns, and only these four. provider and subject are the link
	// itself, so rewriting the whole row would let a replayed exchange move an
	// identity between accounts; created_at is history. The user_id in the
	// WHERE clause is what stops one account touching another's identity.
	tag, err := tx.Exec(ctx, `
		UPDATE user_identities
		   SET email = $1, email_verified = $2, display_name = $3, last_used_at = $4
		 WHERE provider = $5 AND subject = $6 AND user_id = $7`,
		i.Email, i.EmailVerified, i.DisplayName, nullTime(i.LastUsedAt),
		string(i.Provider), i.Subject, string(id))
	if err != nil {
		return types.WrapServerError(err, "update identity")
	}
	if tag.RowsAffected() == 0 {
		return types.NewNotFoundError("identity not found")
	}

	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit touch identity")
	}
	return nil
}

// RemoveIdentity unlinks an external identity.
//
// It does not enforce that the account keeps a way in. That rule spans both
// kinds of proof and lives in the usecase, which is the only layer that can
// see a credential and an identity at the same time.
func (r *UserRepository) RemoveIdentity(
	ctx context.Context,
	id domain.ID,
	provider domain.Provider,
	subject string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin remove identity")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := requireAccount(ctx, tx, id); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
		DELETE FROM user_identities
		 WHERE provider = $1 AND subject = $2 AND user_id = $3`,
		string(provider), subject, string(id))
	if err != nil {
		return types.WrapServerError(err, "delete identity")
	}
	if tag.RowsAffected() == 0 {
		return types.NewNotFoundError("identity not found")
	}

	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit remove identity")
	}
	return nil
}

// TouchCredential records a successful assertion against a stored credential.
func (r *UserRepository) TouchCredential(ctx context.Context, id domain.ID, c domain.Credential) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin touch credential")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := requireAccount(ctx, tx, id); err != nil {
		return err
	}

	// Three columns, and only these three. Rewriting the whole row would let a
	// replayed ceremony overwrite the public key -- which is the one field that
	// decides whether a future assertion verifies at all.
	//
	// The user_id in the WHERE clause is what stops one account touching
	// another account's credential.
	tag, err := tx.Exec(ctx, `
		UPDATE user_credentials
		   SET sign_count = $1, backup_state = $2, last_used_at = $3
		 WHERE id = $4 AND user_id = $5`,
		int64(c.SignCount), c.BackupState, nullTime(c.LastUsedAt), c.ID, string(id))
	if err != nil {
		return types.WrapServerError(err, "update credential")
	}
	if tag.RowsAffected() == 0 {
		return types.NewNotFoundError("credential not found")
	}

	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit touch credential")
	}
	return nil
}

// requireAccount reports a *types.NotFoundError if the account does not exist.
//
// This probe looks redundant next to the foreign key, and it is not. Postgres
// evaluates a unique constraint before it fires a foreign key trigger, so a
// call that is wrong in both ways -- an account that does not exist and a
// credential id already claimed -- reports the unique violation. Without this
// probe the Postgres adapter would answer "credential is already registered"
// where the in-memory adapter answers "account not found": one port, two
// behaviours, and a conformance suite that fails for a reason nobody expects.
func requireAccount(ctx context.Context, tx pgx.Tx, id domain.ID) error {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT true FROM users WHERE id = $1`, string(id)).Scan(&exists)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return types.NewNotFoundError("account not found")
	case err != nil:
		return types.WrapServerError(err, "look up account")
	}
	return nil
}

// insertCredential writes one passkey, mapping a duplicate to the same message
// the in-memory adapter produces.
func insertCredential(ctx context.Context, tx pgx.Tx, id domain.ID, c domain.Credential) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_credentials (
			id, user_id, public_key, attestation_type, transports, aaguid,
			sign_count, backup_eligible, backup_state, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		c.ID, string(id), c.PublicKey, c.AttestationType,
		nilToEmpty(c.Transports), nilToEmpty(c.AAGUID),
		int64(c.SignCount), c.BackupEligible, c.BackupState, c.CreatedAt, nullTime(c.LastUsedAt))
	switch {
	case isUniqueViolation(err, constraintUserCredentialsPK):
		return types.NewValidationError("credential is already registered")
	case err != nil:
		return types.WrapServerError(err, "insert credential")
	}
	return nil
}

// insertIdentity writes one external identity, mapping a duplicate to the same
// message the in-memory adapter produces.
//
// The duplicate case is any account's, this one included: the in-memory
// adapter refuses a re-link because it would duplicate the entry in the slice
// while the index kept pointing at one of them, and the composite primary key
// refuses it here for the same reason. Moving a subject between accounts must
// be an explicit unlink and relink.
func insertIdentity(ctx context.Context, tx pgx.Tx, id domain.ID, i domain.Identity) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_identities (
			provider, subject, user_id, email, email_verified,
			display_name, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		string(i.Provider), i.Subject, string(id), i.Email, i.EmailVerified,
		i.DisplayName, i.CreatedAt, nullTime(i.LastUsedAt))
	switch {
	case isUniqueViolation(err, constraintUserIdentitiesPK):
		return types.NewValidationError("that %s account is already linked", i.Provider)
	case err != nil:
		return types.WrapServerError(err, "insert identity")
	}
	return nil
}

// scanIdentity reads one row of identityColumns.
func scanIdentity(rows pgx.Rows) (domain.Identity, error) {
	var (
		i          domain.Identity
		provider   string
		lastUsedAt *time.Time
	)
	err := rows.Scan(
		&provider, &i.Subject, &i.Email, &i.EmailVerified,
		&i.DisplayName, &i.CreatedAt, &lastUsedAt)
	if err != nil {
		return domain.Identity{}, err
	}
	i.Provider = domain.Provider(provider)
	if lastUsedAt != nil {
		i.LastUsedAt = *lastUsedAt
	}
	return i, nil
}

// scanCredential reads one row of credentialColumns.
func scanCredential(rows pgx.Rows) (domain.Credential, error) {
	var (
		c          domain.Credential
		signCount  int64
		lastUsedAt *time.Time
	)
	err := rows.Scan(
		&c.ID, &c.PublicKey, &c.AttestationType, &c.Transports, &c.AAGUID,
		&signCount, &c.BackupEligible, &c.BackupState, &c.CreatedAt, &lastUsedAt)
	if err != nil {
		return domain.Credential{}, err
	}

	// The SQL CHECK constraint on sign_count is what makes this narrowing cast
	// safe; without it a hand-written UPDATE could wrap the value silently.
	c.SignCount = uint32(signCount)
	if lastUsedAt != nil {
		c.LastUsedAt = *lastUsedAt
	}

	// The in-memory adapter clones with slices.Clone, which returns nil for a
	// nil input, so an absent Transports or AAGUID comes back nil there and
	// empty-but-non-nil here. Two adapters behind one port must not be
	// distinguishable by that.
	c.Transports = emptyToNil(c.Transports)
	c.AAGUID = emptyToNil(c.AAGUID)
	return c, nil
}

// nullTime maps the domain's zero time to SQL NULL, so "never asserted" is
// recorded as an absence rather than as the year 1.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// emptyToNil and nilToEmpty are the two halves of one problem.
//
// The domain uses a nil slice for "no transports" and "no AAGUID", and the
// in-memory adapter preserves that because slices.Clone(nil) is nil. Postgres
// has no such spelling: a nil slice encodes as SQL NULL, which these NOT NULL
// columns reject outright -- a column DEFAULT does not apply when NULL is
// passed explicitly -- and a stored empty array decodes back as empty-but-not-
// nil. Normalising at both ends is what keeps the two adapters indistinguishable
// through the port.
func emptyToNil[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return s
}

func nilToEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// Compile-time proof that this adapter satisfies the port.
var _ domain.Repository = (*UserRepository)(nil)
