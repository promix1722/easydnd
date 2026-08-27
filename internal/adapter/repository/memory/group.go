package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// GroupRepository is the in-process group store.
//
// It holds a UserRepository rather than a copy of everybody's name, because
// the Postgres adapter reads names with a join and the two have to behave the
// same way. Storing the name alongside the membership would be faster and
// would mean a rename showed up in some rosters and not others -- which is
// exactly the bug the shared contract suite exists to make impossible to ship
// in one adapter and not the other.
type GroupRepository struct {
	mu      sync.RWMutex
	groups  map[domain.ID]domain.Group
	members map[domain.ID]map[user.ID]seat

	users *UserRepository
}

// seat is a membership without the display name, which is looked up on read.
type seat struct {
	role     domain.Role
	joinedAt time.Time
}

// NewGroupRepository returns an empty store that reads names out of users.
func NewGroupRepository(users *UserRepository) *GroupRepository {
	return &GroupRepository{
		groups:  make(map[domain.ID]domain.Group),
		members: make(map[domain.ID]map[user.ID]seat),
		users:   users,
	}
}

// Create stores g and seats owner in it, both or neither.
func (r *GroupRepository) Create(_ context.Context, g domain.Group, owner user.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case g.ID == "":
		return types.NewValidationError("group id must not be empty")
	case g.Name == "":
		return types.NewValidationError("group name must not be empty")
	case owner == "":
		return types.NewValidationError("a group must have an owner")
	}
	if _, exists := r.groups[g.ID]; exists {
		return types.NewValidationError("group %q already exists", g.ID)
	}

	r.groups[g.ID] = g
	r.members[g.ID] = map[user.ID]seat{owner: {role: domain.RoleOwner, joinedAt: g.CreatedAt}}
	return nil
}

// ByID returns the group.
func (r *GroupRepository) ByID(_ context.Context, id domain.ID) (domain.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.groups[id]
	if !ok {
		return domain.Group{}, types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	return g, nil
}

// ListFor returns every group u belongs to, most recently created first.
func (r *GroupRepository) ListFor(ctx context.Context, u user.ID) ([]domain.Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Membership, 0)
	for id, seats := range r.members {
		s, ok := seats[u]
		if !ok {
			continue
		}
		out = append(out, domain.Membership{Group: r.groups[id], Role: s.role})
	}
	// Newest first, with the id breaking a tie so the order is total: two
	// groups made in the same clock tick must not swap places between calls.
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Group.CreatedAt.Equal(out[j].Group.CreatedAt) {
			return out[i].Group.CreatedAt.After(out[j].Group.CreatedAt)
		}
		return out[i].Group.ID < out[j].Group.ID
	})
	_ = ctx
	return out, nil
}

// Members returns the whole roster, owner first and then by join time.
func (r *GroupRepository) Members(ctx context.Context, id domain.ID) ([]domain.Member, error) {
	r.mu.RLock()
	seats, ok := r.members[id]
	if !ok {
		r.mu.RUnlock()
		return nil, types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	snapshot := make(map[user.ID]seat, len(seats))
	for k, v := range seats {
		snapshot[k] = v
	}
	r.mu.RUnlock()

	out := make([]domain.Member, 0, len(snapshot))
	for uid, s := range snapshot {
		member := domain.Member{UserID: uid, Role: s.role, JoinedAt: s.joinedAt}
		// A member whose account has gone keeps their seat and loses their
		// name. Dropping the row instead would silently shrink a roster.
		if u, err := r.users.ByID(ctx, uid); err == nil {
			member.DisplayName = u.DisplayName
		} else if !types.IsNotFound(err) {
			return nil, err
		}
		out = append(out, member)
	}
	sortMembers(out)
	return out, nil
}

// sortMembers orders a roster: owner, then DMs, then players, each by join
// time. The order is decided here rather than left to the storage engine so
// that both adapters -- and therefore the screen -- agree.
func sortMembers(members []domain.Member) {
	sort.Slice(members, func(i, j int) bool {
		if members[i].Role.Rank() != members[j].Role.Rank() {
			return members[i].Role.Rank() > members[j].Role.Rank()
		}
		if !members[i].JoinedAt.Equal(members[j].JoinedAt) {
			return members[i].JoinedAt.Before(members[j].JoinedAt)
		}
		return members[i].UserID < members[j].UserID
	})
}

// MemberRole returns u's rank in the group.
func (r *GroupRepository) MemberRole(
	_ context.Context, id domain.ID, u user.ID,
) (domain.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.members[id][u]
	if !ok {
		return "", types.NewNotFoundError("group %q has no member %q", id, u)
	}
	return s.role, nil
}

// AddMember seats u at role.
func (r *GroupRepository) AddMember(
	_ context.Context, id domain.ID, u user.ID, role domain.Role, at time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.groups[id]; !ok {
		return types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	if !role.Valid() || role == domain.RoleOwner {
		// An owner is made by Create and moved by TransferOwnership. Admitting
		// one here is what the partial unique index refuses in Postgres.
		return types.NewValidationError("a member cannot be added as %q", role)
	}
	if _, exists := r.members[id][u]; exists {
		return types.NewValidationError("%q is already in group %q", u, id)
	}
	r.members[id][u] = seat{role: role, joinedAt: at}
	return nil
}

// SetRole moves an existing member between RoleDM and RolePlayer.
func (r *GroupRepository) SetRole(
	_ context.Context, id domain.ID, u user.ID, role domain.Role,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !role.Valid() || role == domain.RoleOwner {
		return types.NewValidationError("a member's role cannot be set to %q", role)
	}
	s, ok := r.members[id][u]
	if !ok {
		return types.NewNotFoundError("group %q has no member %q", id, u)
	}
	if s.role == domain.RoleOwner {
		return types.NewValidationError("the owner's role cannot be changed")
	}
	s.role = role
	r.members[id][u] = s
	return nil
}

// RemoveMember unseats u.
func (r *GroupRepository) RemoveMember(_ context.Context, id domain.ID, u user.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.members[id][u]
	if !ok {
		return types.NewNotFoundError("group %q has no member %q", id, u)
	}
	// Refused in storage as well as in the usecase, so that no ordering of
	// concurrent calls can leave a group with nobody who can run it.
	if s.role == domain.RoleOwner {
		return types.NewValidationError("the owner cannot be removed from group %q", id)
	}
	delete(r.members[id], u)
	return nil
}

// TransferOwnership demotes from to RoleDM and promotes to to RoleOwner.
func (r *GroupRepository) TransferOwnership(
	_ context.Context, id domain.ID, from, to user.ID,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	fromSeat, ok := r.members[id][from]
	if !ok {
		return types.NewNotFoundError("group %q has no member %q", id, from)
	}
	if fromSeat.role != domain.RoleOwner {
		return types.NewValidationError("%q does not own group %q", from, id)
	}
	toSeat, ok := r.members[id][to]
	if !ok {
		return types.NewNotFoundError("group %q has no member %q", id, to)
	}

	// Demote first, then promote -- the same order the Postgres adapter is
	// forced into by group_members_one_owner_idx. It makes no difference here,
	// and writing it the other way round would let this adapter pass a test
	// the real one fails.
	fromSeat.role = domain.RoleDM
	r.members[id][from] = fromSeat
	toSeat.role = domain.RoleOwner
	r.members[id][to] = toSeat
	return nil
}

// Rename changes the group's name.
func (r *GroupRepository) Rename(_ context.Context, id domain.ID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	g, ok := r.groups[id]
	if !ok {
		return types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	if name == "" {
		return types.NewValidationError("group name must not be empty")
	}
	g.Name = name
	r.groups[id] = g
	return nil
}

// Delete removes the group and every membership in it.
func (r *GroupRepository) Delete(_ context.Context, id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.groups[id]; !ok {
		return types.NewNotFoundError("group %q", id).Because("group.notFound")
	}
	delete(r.groups, id)
	delete(r.members, id)
	return nil
}
