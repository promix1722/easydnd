package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// GroupRepository is the durable group store.
//
// As in UserRepository, every error string below is copied verbatim from the
// in-memory adapter: the HTTP layer renders a ValidationError's message into
// the 400 body and a NotFoundError's into the 404 body, so a reworded message
// here is a changed API response.
type GroupRepository struct {
	pool *pgxpool.Pool
}

// NewGroupRepository returns a store over the given pool.
func NewGroupRepository(pool *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{pool: pool}
}

const groupColumns = `id, name, created_by, created_at`

// memberOrder puts the owner first, then DMs, then players, each by join time.
//
// The order lives in SQL rather than in Go so that it costs nothing, and it is
// stated at all so that both adapters -- and therefore the roster on screen --
// agree about it.
const memberOrder = `
	ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'dm' THEN 1 ELSE 2 END,
	         m.joined_at, m.user_id`

// Create stores g and seats owner in it, in one transaction.
//
// A group with no owner must never be observable, so this is not a create
// followed by an add: an interrupted pair would leave exactly that.
func (r *GroupRepository) Create(ctx context.Context, g domain.Group, owner user.ID) error {
	switch {
	case g.ID == "":
		return types.NewValidationError("group id must not be empty")
	case g.Name == "":
		return types.NewValidationError("group name must not be empty")
	case owner == "":
		return types.NewValidationError("a group must have an owner")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin create group")
	}
	// Rollback after a successful Commit is a no-op, so this needs no flag.
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		`INSERT INTO groups (id, name, created_by, created_at) VALUES ($1, $2, $3, $4)`,
		string(g.ID), g.Name, string(g.CreatedBy), g.CreatedAt)
	switch {
	case isUniqueViolation(err, constraintGroupsPK):
		return types.NewValidationError("group %q already exists", g.ID)
	case err != nil:
		return types.WrapServerError(err, "insert group")
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, role, joined_at) VALUES ($1, $2, $3, $4)`,
		string(g.ID), string(owner), string(domain.RoleOwner), g.CreatedAt)
	if err != nil {
		return types.WrapServerError(err, "insert group owner")
	}

	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit create group")
	}
	return nil
}

// ByID returns the group.
func (r *GroupRepository) ByID(ctx context.Context, id domain.ID) (domain.Group, error) {
	var g domain.Group
	err := r.pool.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE id = $1`, string(id),
	).Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Group{}, types.NewNotFoundError("group %q", id).Because("group.notFound")
	case err != nil:
		return domain.Group{}, types.WrapServerError(err, "load group")
	}
	return g, nil
}

// ListFor returns every group u belongs to, most recently created first.
func (r *GroupRepository) ListFor(ctx context.Context, u user.ID) ([]domain.Membership, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT g.id, g.name, g.created_by, g.created_at, m.role
		   FROM groups g
		   JOIN group_members m ON m.group_id = g.id
		  WHERE m.user_id = $1
		  ORDER BY g.created_at DESC, g.id`, string(u))
	if err != nil {
		return nil, types.WrapServerError(err, "list groups")
	}
	defer rows.Close()

	out := make([]domain.Membership, 0)
	for rows.Next() {
		var m domain.Membership
		if err := rows.Scan(
			&m.Group.ID, &m.Group.Name, &m.Group.CreatedBy, &m.Group.CreatedAt, &m.Role,
		); err != nil {
			return nil, types.WrapServerError(err, "scan group")
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, types.WrapServerError(err, "list groups")
	}
	return out, nil
}

// Members returns the whole roster, with each member's name read from users.
//
// The join is what makes "one table says what a person is called" true: a name
// is never copied into group_members, so a rename is visible in every roster
// at once or in none.
func (r *GroupRepository) Members(ctx context.Context, id domain.ID) ([]domain.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT m.user_id, u.display_name, m.role, m.joined_at
		   FROM group_members m
		   JOIN users u ON u.id = m.user_id
		  WHERE m.group_id = $1`+memberOrder, string(id))
	if err != nil {
		return nil, types.WrapServerError(err, "load roster")
	}
	defer rows.Close()

	out := make([]domain.Member, 0)
	for rows.Next() {
		var m domain.Member
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.Role, &m.JoinedAt); err != nil {
			return nil, types.WrapServerError(err, "scan member")
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, types.WrapServerError(err, "load roster")
	}
	// Every group has an owner from the moment it is created, so an empty
	// roster cannot mean "a group with nobody in it" -- it means there is no
	// such group. That saves a second query on the common path.
	if len(out) == 0 {
		return nil, types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	return out, nil
}

// MemberRole returns u's rank in the group.
func (r *GroupRepository) MemberRole(
	ctx context.Context, id domain.ID, u user.ID,
) (domain.Role, error) {
	var role domain.Role
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2`,
		string(id), string(u)).Scan(&role)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", types.NewNotFoundError("group %q has no member %q", id, u)
	case err != nil:
		return "", types.WrapServerError(err, "load membership")
	}
	return role, nil
}

// AddMember seats u at role.
func (r *GroupRepository) AddMember(
	ctx context.Context, id domain.ID, u user.ID, role domain.Role, at time.Time,
) error {
	if !role.Valid() || role == domain.RoleOwner {
		return types.NewValidationError("a member cannot be added as %q", role)
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, role, joined_at) VALUES ($1, $2, $3, $4)`,
		string(id), string(u), string(role), at)
	switch {
	case isUniqueViolation(err, constraintGroupMembersPK):
		return types.NewValidationError("%q is already in group %q", u, id)
	case isForeignKeyViolation(err, constraintGroupMembersGroupFK):
		return types.NewNotFoundError("group %q", id).Because("group.notFound")
	case err != nil:
		// A violation of the users foreign key lands here on purpose. It means
		// the usecase did not materialise the row a guest needs, which is a
		// bug of ours and not something to explain to the caller.
		return types.WrapServerError(err, "add group member")
	}
	return nil
}

// SetRole moves an existing member between RoleDM and RolePlayer.
func (r *GroupRepository) SetRole(
	ctx context.Context, id domain.ID, u user.ID, role domain.Role,
) error {
	if !role.Valid() || role == domain.RoleOwner {
		return types.NewValidationError("a member's role cannot be set to %q", role)
	}
	// `AND role <> 'owner'` puts the rule in the statement rather than in a
	// read followed by a write, so a concurrent transfer cannot slip between
	// the check and the update.
	tag, err := r.pool.Exec(ctx,
		`UPDATE group_members SET role = $3
		  WHERE group_id = $1 AND user_id = $2 AND role <> 'owner'`,
		string(id), string(u), string(role))
	if err != nil {
		return types.WrapServerError(err, "set member role")
	}
	if tag.RowsAffected() == 0 {
		return r.explainMissedWrite(ctx, id, u, "the owner's role cannot be changed")
	}
	return nil
}

// RemoveMember unseats u.
func (r *GroupRepository) RemoveMember(ctx context.Context, id domain.ID, u user.ID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM group_members
		  WHERE group_id = $1 AND user_id = $2 AND role <> 'owner'`,
		string(id), string(u))
	if err != nil {
		return types.WrapServerError(err, "remove group member")
	}
	if tag.RowsAffected() == 0 {
		return r.explainMissedWrite(ctx, id, u,
			"the owner cannot be removed from group "+quoted(id))
	}
	return nil
}

// explainMissedWrite turns "no rows matched" into the reason there were none.
//
// Both writes above exclude the owner in their WHERE clause, which makes them
// safe but indistinguishable: zero rows means either that the person is not a
// member or that they are the owner. Only once that has happened is it worth a
// second query to find out which, and the answer decides between a 404 and a
// 400.
func (r *GroupRepository) explainMissedWrite(
	ctx context.Context, id domain.ID, u user.ID, ownerMessage string,
) error {
	role, err := r.MemberRole(ctx, id, u)
	if err != nil {
		return err
	}
	if role == domain.RoleOwner {
		return types.NewValidationError("%s", ownerMessage)
	}
	// The row is there and is not the owner's, so the write should have
	// matched. Something changed underneath us.
	return types.NewValidationError("group %q membership changed; try again", id)
}

// quoted renders an id the way the in-memory adapter's %q verb does, so the
// two adapters produce byte-identical messages.
func quoted(id domain.ID) string { return `"` + string(id) + `"` }

// TransferOwnership demotes from to RoleDM and promotes to to RoleOwner.
func (r *GroupRepository) TransferOwnership(
	ctx context.Context, id domain.ID, from, to user.ID,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin transfer ownership")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// DEMOTE FIRST, PROMOTE SECOND. group_members_one_owner_idx is a unique
	// index, which is checked as each statement runs and cannot be deferred to
	// commit: promoting first means two owners exist for one statement and the
	// index rejects it. Demoting first passes through zero owners, which it
	// permits. The in-memory adapter writes them in this order too, so that it
	// cannot pass a test this one fails.
	tag, err := tx.Exec(ctx,
		`UPDATE group_members SET role = 'dm'
		  WHERE group_id = $1 AND user_id = $2 AND role = 'owner'`,
		string(id), string(from))
	if err != nil {
		return types.WrapServerError(err, "demote outgoing owner")
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.MemberRole(ctx, id, from); err != nil {
			return err
		}
		return types.NewValidationError("%q does not own group %q", from, id)
	}

	tag, err = tx.Exec(ctx,
		`UPDATE group_members SET role = 'owner'
		  WHERE group_id = $1 AND user_id = $2`,
		string(id), string(to))
	if err != nil {
		return types.WrapServerError(err, "promote incoming owner")
	}
	if tag.RowsAffected() == 0 {
		return types.NewNotFoundError("group %q has no member %q", id, to)
	}

	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit transfer ownership")
	}
	return nil
}

// Rename changes the group's name.
func (r *GroupRepository) Rename(ctx context.Context, id domain.ID, name string) error {
	if name == "" {
		return types.NewValidationError("group name must not be empty")
	}
	tag, err := r.pool.Exec(ctx, `UPDATE groups SET name = $2 WHERE id = $1`, string(id), name)
	if err != nil {
		return types.WrapServerError(err, "rename group")
	}
	if tag.RowsAffected() == 0 {
		return types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	return nil
}

// Delete removes the group; group_members follows by cascade.
func (r *GroupRepository) Delete(ctx context.Context, id domain.ID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM groups WHERE id = $1`, string(id))
	if err != nil {
		return types.WrapServerError(err, "delete group")
	}
	if tag.RowsAffected() == 0 {
		return types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	return nil
}

// Compile-time proof that this adapter satisfies the port.
var _ domain.Repository = (*GroupRepository)(nil)
