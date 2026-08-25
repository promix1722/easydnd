package group

import (
	"context"
	"time"

	"github.com/promix1722/easydnd/internal/domain/user"
)

// Repository is the persistence port for groups and their rosters.
//
// The method set is shaped by one rule: no caller should ever have to read a
// group, decide something, and write it back. Every method below is a single
// statement or a single transaction, so two people editing the same roster at
// once cannot interleave into a state neither of them asked for. That is why
// TransferOwnership exists as its own method instead of being two SetRole
// calls, and why there is no general Save.
//
// Implementations live under internal/adapter/repository; internal/app picks
// the concrete one, and that assignment is what proves conformance.
type Repository interface {
	// Create stores g and seats owner in it as RoleOwner, both or neither.
	// A group with no owner must never be observable, so this is one
	// transaction rather than a create followed by an add. It reports a
	// *types.ValidationError if the id is already taken.
	Create(ctx context.Context, g Group, owner user.ID) error

	// ByID returns the group, or a *types.NotFoundError. It says nothing about
	// who may see it -- that is the usecase's decision, and this port is
	// deliberately unable to make it.
	ByID(ctx context.Context, id ID) (Group, error)

	// ListFor returns every group u belongs to, with u's own rank in each,
	// most recently created first.
	ListFor(ctx context.Context, u user.ID) ([]Membership, error)

	// Members returns the whole roster, owner first and then by join time, so
	// that the order on screen does not depend on the storage engine.
	Members(ctx context.Context, id ID) ([]Member, error)

	// MemberRole returns u's rank in the group, or a *types.NotFoundError if
	// they are not in it.
	//
	// This is the hot path: it is called on the way into every single group
	// operation, which is why it is its own query rather than a scan of what
	// Members returns.
	MemberRole(ctx context.Context, id ID, u user.ID) (Role, error)

	// AddMember seats u at role. It reports a *types.ValidationError if u is
	// already seated -- deciding that a repeat join is harmless is the
	// usecase's call to make, not storage's to hide.
	AddMember(ctx context.Context, id ID, u user.ID, role Role, at time.Time) error

	// SetRole moves an existing member between RoleDM and RolePlayer. It
	// reports a *types.NotFoundError if u is not a member.
	//
	// It must not be used to hand out RoleOwner: a second owner violates the
	// invariant the whole role model rests on, and the unique index behind
	// this port will refuse it. Ownership moves through TransferOwnership.
	SetRole(ctx context.Context, id ID, u user.ID, role Role) error

	// RemoveMember unseats u, reporting a *types.NotFoundError if they were
	// not seated. It enforces no rule about who is left behind; the usecase
	// owns "a group always has an owner".
	RemoveMember(ctx context.Context, id ID, u user.ID) error

	// TransferOwnership demotes from to RoleDM and promotes to to RoleOwner,
	// in one transaction.
	//
	// One method rather than two SetRole calls because there is no ordering of
	// those two writes that is safe: promote first and the group briefly has
	// two owners, demote first and it briefly has none. Both must be a single
	// atomic step. Both users must already be members.
	TransferOwnership(ctx context.Context, id ID, from, to user.ID) error

	// Rename changes the group's name, reporting a *types.NotFoundError if it
	// does not exist.
	Rename(ctx context.Context, id ID, name string) error

	// Delete removes the group and, with it, every membership.
	Delete(ctx context.Context, id ID) error
}
