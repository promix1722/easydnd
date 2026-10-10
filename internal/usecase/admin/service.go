// Package admin holds the superadmin's read-only listings: every account and
// every character, whoever owns them.
//
// Nothing here asks who is calling. The routes sit behind
// middleware.RequireSuperadmin, and that is the whole of the authorization.
package admin

import (
	"context"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/user"
)

// ownerMatches bounds how many accounts an owner filter may resolve to.
const ownerMatches = 200

// Service lists accounts and characters.
type Service struct {
	users      user.Repository
	characters character.Repository
}

// NewService wires a Service over the two stores it reads.
func NewService(users user.Repository, characters character.Repository) *Service {
	return &Service{users: users, characters: characters}
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
