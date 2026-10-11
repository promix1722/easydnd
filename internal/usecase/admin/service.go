// Package admin holds the superadmin's listings -- every account and every
// character, whoever owns them -- and the deletion of any character or
// account.
//
// Nothing here asks who is calling. The routes sit behind
// middleware.RequireSuperadmin, and that is the whole of the authorization.
package admin

import (
	"context"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// ownerMatches bounds how many accounts an owner filter may resolve to.
const ownerMatches = 200

// Service lists accounts and characters.
type Service struct {
	users      user.Repository
	characters character.Repository
	remove     Remove
	groups     Groups
	packs      Packs
}

// Groups is what deleting an account needs of the group usecase, called in
// the name of the account being deleted so that every rule a person leaving
// or closing a table is held to applies here too.
type Groups interface {
	List(ctx context.Context, actor user.ID) ([]group.Membership, error)
	Get(ctx context.Context, actor user.ID, id group.ID) (group.Group, group.Role, []group.Member, error)
	Delete(ctx context.Context, actor user.ID, id group.ID) error
	RemoveMember(ctx context.Context, actor user.ID, id group.ID, target user.ID) error
}

// Packs reports the homebrew packs an account authored.
type Packs interface {
	ListFor(ctx context.Context, owner user.ID, ids []string) ([]pack.Record, error)
}

// WithAccounts installs what DeletePlayer settles before the account goes.
func (s *Service) WithAccounts(groups Groups, packs Packs) *Service {
	s.groups, s.packs = groups, packs
	return s
}

// DeletePlayer deletes an account and what is only theirs: their characters,
// their seats at other people's tables, and the groups they own alone.
//
// It refuses, before touching anything, where the delete would take something
// from somebody else: a group they own that other people sit at, or a
// homebrew pack they authored, which other people's characters may be pinned
// to and which nothing in this application can delete. And it refuses the
// caller's own account, so a superadmin cannot lock the door behind them.
//
// ponytail: not one transaction -- the steps run through four stores. Every
// refusal comes first, and a failure after that leaves an account with fewer
// things, which a second call finishes. Folders and AI Wizard chats are left
// as a guest's are: owned by an id with no account row.
func (s *Service) DeletePlayer(ctx context.Context, actor, id user.ID) error {
	if actor == id {
		return types.NewValidationError("a superadmin cannot delete their own account").Because("admin.player.self")
	}
	if _, err := s.users.ByID(ctx, id); err != nil {
		return err
	}
	seats, err := s.groups.List(ctx, id)
	if err != nil {
		return err
	}
	for _, seat := range seats {
		if seat.Role != group.RoleOwner {
			continue
		}
		_, _, members, err := s.groups.Get(ctx, id, seat.Group.ID)
		if err != nil {
			return err
		}
		if len(members) > 1 {
			return types.NewValidationError("the account owns group %q, which has other members", seat.Group.Name).Because("admin.player.ownsGroup")
		}
	}
	authored, err := s.packs.ListFor(ctx, id, nil)
	if err != nil {
		return err
	}
	if len(authored) > 0 {
		return types.NewValidationError("the account authored %d homebrew packs", len(authored)).Because("admin.player.ownsPacks")
	}

	// Characters first: deleting one unshares it, which needs the seat.
	characters, err := s.characters.List(ctx, character.OwnerID(id))
	if err != nil {
		return err
	}
	for _, c := range characters {
		if err := s.remove(ctx, c.Owner, c.ID); err != nil {
			return err
		}
	}
	for _, seat := range seats {
		if seat.Role == group.RoleOwner {
			err = s.groups.Delete(ctx, id, seat.Group.ID)
		} else {
			err = s.groups.RemoveMember(ctx, id, seat.Group.ID, id)
		}
		if err != nil {
			return err
		}
	}
	return s.users.Delete(ctx, id)
}

// Remove deletes a character as its owner would: the character usecase's
// Delete, which takes it off every group and game first.
type Remove func(ctx context.Context, owner character.OwnerID, id character.ID) error

// NewService wires a Service over the two stores it reads and the one delete.
func NewService(users user.Repository, characters character.Repository, remove Remove) *Service {
	return &Service{users: users, characters: characters, remove: remove}
}

// DeleteCharacter deletes anybody's character, in its owner's name.
func (s *Service) DeleteCharacter(ctx context.Context, id character.ID) error {
	c, err := s.characters.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.remove(ctx, c.Owner, id)
}

// Players lists stored accounts. A guest who never joined a group has no row
// and so is not among them.
func (s *Service) Players(ctx context.Context, q user.Query) ([]user.Listed, int, error) {
	return s.users.Search(ctx, q)
}

// CharacterQuery is character.Query with the owner given as text a person
// would type -- a name, an email or an id -- rather than as resolved ids.
type CharacterQuery struct {
	Owner  string
	ID     string
	Public *bool
	Limit  int
	Offset int
}

// Character is one row of the listing.
type Character struct {
	character.Summary
	// OwnerName is empty for a guest who never joined a group: they own
	// characters and have no account row to be named in.
	OwnerName string
	Public    bool
	Revision  int
}

// Characters lists characters across every owner.
//
// Name, level and class are folded from the log for the returned page only,
// which is why none of them is a filter: they are not stored.
func (s *Service) Characters(ctx context.Context, q CharacterQuery) ([]Character, int, error) {
	query := character.Query{ID: q.ID, Public: q.Public, Limit: q.Limit, Offset: q.Offset}
	if q.Owner != "" {
		// The text itself stands in as an id, because a guest who never joined
		// a group owns characters without an account row to match by name.
		query.Owners = []character.OwnerID{character.OwnerID(q.Owner)}
		// ponytail: an owner filter sees the first 200 matching accounts; join
		// users in SQL if a name ever matches more.
		matches, _, err := s.users.Search(ctx, user.Query{Text: q.Owner, Limit: ownerMatches})
		if err != nil {
			return nil, 0, err
		}
		for _, m := range matches {
			query.Owners = append(query.Owners, character.OwnerID(m.ID))
		}
	}
	found, total, err := s.characters.Search(ctx, query)
	if err != nil || len(found) == 0 {
		return nil, total, err
	}

	ids := make([]user.ID, 0, len(found))
	for _, c := range found {
		ids = append(ids, user.ID(c.Owner))
	}
	owners, _, err := s.users.Search(ctx, user.Query{IDs: ids, Limit: len(ids)})
	if err != nil {
		return nil, 0, err
	}
	names := make(map[user.ID]string, len(owners))
	for _, o := range owners {
		names[o.ID] = o.DisplayName
	}

	out := make([]Character, 0, len(found))
	for _, c := range found {
		// No catalogue: it would only add the subclass to the class line, at
		// the price of resolving a pinned compendium per row.
		summary := character.Summarize(c.ID, c.Owner, c.Folder, c.Log, nil)
		summary.Image = ""
		out = append(out, Character{
			Summary: summary, OwnerName: names[user.ID(c.Owner)],
			Public: c.Public, Revision: c.Revision,
		})
	}
	return out, total, nil
}
