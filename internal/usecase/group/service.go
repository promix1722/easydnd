// Package group implements the play-group application usecases.
//
// This is where the codebase's authorization model stops being one predicate.
// A character has an owner and the rule is "is this yours"; a group has a
// roster and three ranks, and every rule is "does your rank in this group
// outrank what you are trying to do". All of those rules live in this package
// and nowhere else -- in particular not in the handlers, because "the handler
// remembered to check" is a habit rather than an invariant.
//
// It depends on the group domain, the account domain and internal/types, and
// on nothing else. It never sees a *gin.Context and it never sees a JWT.
package group

import (
	"context"
	"log/slog"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// groupIDBytes is the entropy behind a group id. A group id travels in invite
// links, so it has to be unguessable for the same reason a character id has to
// be: possession of the identifier is one half of reaching the thing.
const groupIDBytes = 16

// Service holds the group usecases. Every dependency arrives through the
// constructor; there are no package-level singletons.
type Service struct {
	repo    domain.Repository
	users   user.Repository
	invites domain.Inviter
	tables  domain.Tables
	log     *slog.Logger

	// clock is injected so a test can predict the timestamps an invite and a
	// membership are stamped with. Nil means the real clock; see now.
	clock func() time.Time
}

// NewService wires a Service over the group store, the account store and the
// invite signer.
//
// It takes the account store because a group roster shows names, and there is
// one table in this schema that says what a person is called. It is also what
// materialises the row a guest needs before they can be named in one -- see
// ensureStored.
//
// The tables port may be nil, on the same terms character.Sharing may be: a
// build with no games in it has nothing to tear down when a group goes.
func NewService(
	repo domain.Repository, users user.Repository, invites domain.Inviter,
	tables domain.Tables, log *slog.Logger,
) *Service {
	return &Service{repo: repo, users: users, invites: invites, tables: tables, log: log}
}

// now reads the clock, defaulting to the real one.
func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now().UTC()
}

// member resolves the actor's standing in a group, and is the only way any
// read or write in this package reaches one.
//
// A non-member gets a NotFoundError rather than an AccessDeniedError, for
// exactly the reason character.owned does: a 403 on somebody else's id
// confirms that the id exists, which turns a guessable identifier into an
// enumeration oracle. A member who lacks a right gets AccessDenied instead,
// and the difference is deliberate -- they already know the group exists,
// they are looking at it, and pretending otherwise would be theatre that only
// confused them.
func (s *Service) member(
	ctx context.Context, actor user.ID, id domain.ID,
) (domain.Group, domain.Role, error) {
	role, err := s.repo.MemberRole(ctx, id, actor)
	if err != nil {
		if types.IsNotFound(err) {
			return domain.Group{}, "", types.NewNotFoundError("group %q", id)
		}
		return domain.Group{}, "", err
	}
	g, err := s.repo.ByID(ctx, id)
	if err != nil {
		return domain.Group{}, "", err
	}
	return g, role, nil
}

// ensureStored materialises the row a guest needs in order to be named in
// somebody else's roster.
//
// An account already has one. A guest is minted inside a session token and
// stored nowhere, which was fine while a guest could only see their own
// things -- but a roster is read by other people, and a foreign key with
// nothing behind it is not a roster entry. This runs on the two paths that put
// somebody into a group, and only those.
func (s *Service) ensureStored(ctx context.Context, actor user.User) error {
	if !actor.Anonymous {
		return nil
	}
	return s.users.EnsureGuest(ctx, actor)
}

// Create opens a group with actor seated in it as its owner.
//
// Anyone with a session may do this, guests included. A guest's group is real
// and durable; it is the guest who is not, and every surface that shows one
// says so.
func (s *Service) Create(ctx context.Context, actor user.User, name string) (domain.Group, error) {
	clean, err := validateName(name)
	if err != nil {
		return domain.Group{}, err
	}
	if err := s.ensureStored(ctx, actor); err != nil {
		return domain.Group{}, err
	}
	id, err := newGroupID()
	if err != nil {
		return domain.Group{}, err
	}

	g := domain.Group{ID: id, Name: clean, CreatedBy: actor.ID, CreatedAt: s.now()}
	if err := s.repo.Create(ctx, g, actor.ID); err != nil {
		return domain.Group{}, err
	}
	s.log.Info("group created", "group_id", string(id), "owner_id", string(actor.ID))
	return g, nil
}

// List returns the groups actor belongs to, with actor's rank in each.
func (s *Service) List(ctx context.Context, actor user.ID) ([]domain.Membership, error) {
	return s.repo.ListFor(ctx, actor)
}

// Get returns a group and its whole roster. Every member may read it: seeing
// who else is at the table is most of what a group is for.
func (s *Service) Get(
	ctx context.Context, actor user.ID, id domain.ID,
) (domain.Group, domain.Role, []domain.Member, error) {
	g, role, err := s.member(ctx, actor, id)
	if err != nil {
		return domain.Group{}, "", nil, err
	}
	members, err := s.repo.Members(ctx, id)
	if err != nil {
		return domain.Group{}, "", nil, err
	}
	return g, role, members, nil
}

// Rename changes what the table is called. Owner and DMs may; players may not.
func (s *Service) Rename(
	ctx context.Context, actor user.ID, id domain.ID, name string,
) (domain.Group, error) {
	g, role, err := s.member(ctx, actor, id)
	if err != nil {
		return domain.Group{}, err
	}
	if !role.AtLeast(domain.RoleDM) {
		return domain.Group{}, types.NewAccessDeniedError("only the owner or a DM may rename a group")
	}
	clean, err := validateName(name)
	if err != nil {
		return domain.Group{}, err
	}
	if err := s.repo.Rename(ctx, id, clean); err != nil {
		return domain.Group{}, err
	}
	g.Name = clean
	return g, nil
}

// Delete removes the group and every membership in it.
//
// Owner only. It is also the escape hatch for an owner who wants out of a
// group nobody else should inherit: they may not leave, but they may close the
// table.
func (s *Service) Delete(ctx context.Context, actor user.ID, id domain.ID) error {
	_, role, err := s.member(ctx, actor, id)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner {
		return types.NewAccessDeniedError("only the owner may delete a group")
	}
	// The games and the shared table go first, for the reason deleting a
	// character unshares it first: stopping in the middle leaves a group with
	// nothing hung off it, which is a group, whereas the other order leaves
	// rows naming a table that is gone.
	if s.tables != nil {
		if err := s.tables.DeleteForGroup(ctx, id); err != nil {
			return err
		}
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.log.Info("group deleted", "group_id", string(id), "actor_id", string(actor))
	return nil
}
