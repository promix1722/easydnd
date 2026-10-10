// Package pack owns authoring and access to private rule releases.
package pack

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/group"
	domain "github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

type Service struct {
	repo   domain.Repository
	engine domain.Engine
	groups group.Repository
	users  user.Repository
	// superadmins is auth.superadmins: who reads restricted packs outright.
	superadmins []string
	limits      types.Limits
}

// SetLimits replaces the limits this service enforces; it starts with
// types.DefaultLimits.
func (s *Service) SetLimits(l types.Limits) { s.limits = l }

func (s *Service) SetSuperadmins(list []string) { s.superadmins = list }

// Superadmin reports whether u is one. A guest has no account row and so no
// verified email; it can only match by id, which nobody would configure.
func (s *Service) Superadmin(ctx context.Context, u user.ID) bool {
	if len(s.superadmins) == 0 {
		return false
	}
	account, err := s.users.ByID(ctx, u)
	if err != nil {
		account = user.User{ID: u}
	}
	return account.Superadmin(s.superadmins)
}

func NewService(r domain.Repository, e domain.Engine, g group.Repository, u user.Repository) *Service {
	return &Service{repo: r, engine: e, groups: g, users: u, limits: types.DefaultLimits}
}
func (s *Service) Schema() []byte       { return s.engine.Schema() }
func (s *Service) Default() domain.Lock { return s.engine.Default() }
func denied() error                     { return types.NewNotFoundError("pack unavailable").Because("pack.unavailable") }
func invalid(err error) error {
	d := Diagnose(err)
	return types.NewValidationError("invalid pack: %v", err).Because(d.Reason, d.Args)
}
func (s *Service) available(ctx context.Context, u user.ID) ([]domain.Record, map[domain.Release]bool, error) {
	records, err := s.repo.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	records = append(records, s.engine.Builtins()...)
	granted, err := s.repo.Grants(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	allowed := map[domain.Release]bool{}
	for _, r := range records {
		// A restricted disk pack is unowned too, but only a superadmin has it
		// outright; everybody else reaches it by a superadmin's grant, or
		// through a group share below.
		// ponytail: one account lookup per restricted pack per call; cache
		// per request if a deployment ever installs more than a handful.
		if !r.Archived && (r.Owner == u || r.Owner == "" && (!r.Restricted || slices.Contains(granted, r.ID) || s.Superadmin(ctx, u))) {
			for _, d := range r.Releases {
				allowed[d.Release] = true
			}
		}
	}
	groups, err := s.groups.ListFor(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	for _, g := range groups {
		shares, err := s.repo.Shares(ctx, string(g.Group.ID))
		if err != nil {
			return nil, nil, err
		}
		for _, share := range shares {
			for _, r := range share.Lock.Packs {
				allowed[r] = true
			}
		}
	}
	for _, r := range records {
		if r.Archived {
			for _, d := range r.Releases {
				delete(allowed, d.Release)
			}
		}
	}
	return records, allowed, nil
}

// Restricted lists the private disk packs installed here: what a superadmin
// has to hand out.
func (s *Service) Restricted() []domain.Record {
	out := []domain.Record{}
	for _, r := range s.engine.Builtins() {
		if r.Restricted {
			out = append(out, r)
		}
	}
	return out
}

// Granted lists the restricted packs handed to one account.
func (s *Service) Granted(ctx context.Context, u user.ID) ([]string, error) {
	return s.repo.Grants(ctx, u)
}

// Grant replaces the restricted packs one account has been handed. It does
// not ask who is calling: the route is the superadmin's, as with every admin
// listing. The account must be a stored one -- a guest who never joined a
// group has no row to hang a grant on -- and every id an installed restricted
// pack, so a grant can never open a homebrew pack or name one that is not
// there.
//
// Taking a pack away stops new characters being built on it. One already
// built keeps loading: a read follows the character's lock, not this list.
func (s *Service) Grant(ctx context.Context, u user.ID, packs []string) error {
	if _, err := s.users.ByID(ctx, u); err != nil {
		return err
	}
	restricted := s.Restricted()
	for _, id := range packs {
		if !slices.ContainsFunc(restricted, func(r domain.Record) bool { return r.ID == id }) {
			return denied()
		}
	}
	return s.repo.SetGrants(ctx, u, packs)
}
func (s *Service) List(ctx context.Context, u user.ID) ([]domain.Record, error) {
	records, allowed, err := s.available(ctx, u)
	if err != nil {
		return nil, err
	}
	out := []domain.Record{}
	for _, r := range records {
		releases := []domain.Document{}
		for _, d := range r.Releases {
			if allowed[d.Release] || r.Owner == u {
				releases = append(releases, d)
			}
		}
		if r.Owner != u {
			r.Draft = nil
		}
		if r.Owner == u || len(releases) > 0 {
			r.Releases = releases
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *Service) Get(ctx context.Context, u user.ID, id string) (domain.Record, error) {
	rows, err := s.List(ctx, u)
	if err != nil {
		return domain.Record{}, err
	}
	for _, r := range rows {
		if r.ID == id {
			return r, nil
		}
	}
	return domain.Record{}, denied()
}
func (s *Service) owned(ctx context.Context, u user.ID, id string) (domain.Record, error) {
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return r, err
	}
	if r.Owner != u {
		return domain.Record{}, denied()
	}
	return r, nil
}
func (s *Service) Create(ctx context.Context, u user.User, title string, data []byte, mappings map[string]string) (domain.Record, error) {
	if strings.TrimSpace(title) == "" || len(title) > 160 {
		return domain.Record{}, types.NewValidationError("invalid pack title").Because("pack.title")
	}
	if u.ID == "" {
		return domain.Record{}, denied()
	}
	// Archived packs do not count: a pack cannot be deleted, so counting them
	// would leave an owner at the limit with no way back under it.
	// ponytail: reads every pack to count one owner's, and counts before it
	// inserts. A CountByOwner port over rule_packs_owner fixes the first, a
	// guarded INSERT the second.
	all, err := s.repo.List(ctx)
	if err != nil {
		return domain.Record{}, err
	}
	held := 0
	for _, r := range all {
		if r.Owner == u.ID && !r.Archived {
			held++
		}
	}
	if held >= s.limits.Packs {
		return domain.Record{}, types.LimitReached("packs", s.limits.Packs)
	}
	if u.Anonymous {
		if err := s.users.EnsureGuest(ctx, u); err != nil {
			return domain.Record{}, err
		}
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return domain.Record{}, err
	}
	id := "home-" + hex.EncodeToString(b[:])
	draft := s.engine.NewDraft(id)
	if len(data) > 0 {
		var err error
		draft, err = s.engine.Fork(data, id, mappings)
		if err != nil {
			return domain.Record{}, invalid(err)
		}
	}
	if err := s.engine.CheckDraft(draft, id); err != nil {
		return domain.Record{}, invalid(err)
	}
	r := domain.Record{ID: id, Title: strings.TrimSpace(title), Owner: u.ID, Draft: draft}
	if err := s.repo.Save(ctx, r, 0); err != nil {
		return r, err
	}
	return s.repo.Get(ctx, id)
}
func (s *Service) Save(ctx context.Context, u user.ID, id, title string, revision int, data []byte, mappings map[string]string) (domain.Record, error) {
	r, err := s.owned(ctx, u, id)
	if err != nil {
		return r, err
	}
	if strings.TrimSpace(title) == "" || len(title) > 160 {
		return r, types.NewValidationError("invalid title").Because("pack.title")
	}
	if err = s.engine.CheckDraft(data, id); err != nil {
		return r, invalid(err)
	}
	if len(mappings) > 0 {
		data, err = s.engine.Fork(data, id, mappings)
		if err != nil {
			return r, invalid(err)
		}
	}
	r.Title = strings.TrimSpace(title)
	r.Draft = data
	if err = s.repo.Save(ctx, r, revision); err != nil {
		return r, err
	}
	return s.repo.Get(ctx, id)
}
func documents(records []domain.Record, allowed map[domain.Release]bool) []domain.Document {
	out := []domain.Document{}
	for _, r := range records {
		for _, d := range r.Releases {
			if allowed[d.Release] {
				out = append(out, d)
			}
		}
	}
	return out
}
func (s *Service) Resolve(ctx context.Context, u user.ID, roots []domain.Release) (domain.Lock, error) {
	records, allowed, err := s.available(ctx, u)
	if err != nil {
		return domain.Lock{}, err
	}
	for _, root := range roots {
		found := false
		for r := range allowed {
			if r.ID == root.ID && r.Version == root.Version && (root.Digest == "" || root.Digest == r.Digest) {
				found = true
			}
		}
		if !found {
			return domain.Lock{}, denied()
		}
	}
	l, err := s.engine.Resolve(ctx, documents(records, allowed), roots)
	if err != nil {
		return l, invalid(err)
	}
	return l, nil
}

// AuthorizeLock allows retained releases only through a character already owned
// by the caller. Every newly introduced release still needs current access.
func (s *Service) AuthorizeLock(ctx context.Context, u user.ID, target, retained domain.Lock) error {
	records, allowed, err := s.available(ctx, u)
	if err != nil {
		return err
	}
	for _, r := range retained.Packs {
		allowed[r] = true
	}
	for _, r := range target.Packs {
		if !allowed[r] {
			return denied()
		}
	}
	l, err := s.engine.Resolve(ctx, documents(records, allowed), target.Packs)
	if err != nil {
		return invalid(err)
	}
	if !l.Equal(target) {
		return types.NewValidationError("incomplete lock").Because("pack.invalid")
	}
	return nil
}
func (s *Service) Validate(ctx context.Context, u user.ID, id string) (domain.Document, domain.Lock, error) {
	r, err := s.owned(ctx, u, id)
	if err != nil {
		return domain.Document{}, domain.Lock{}, err
	}
	records, allowed, err := s.available(ctx, u)
	if err != nil {
		return domain.Document{}, domain.Lock{}, err
	}
	return s.engine.Validate(ctx, documents(records, allowed), r.Draft)
}
func (s *Service) Publish(ctx context.Context, u user.ID, id string, revision int) (domain.Record, error) {
	r, err := s.owned(ctx, u, id)
	if err != nil {
		return r, err
	}
	if r.Revision != revision {
		return r, types.NewValidationError("stale revision").Because("pack.stale")
	}
	doc, _, err := s.Validate(ctx, u, id)
	if err != nil {
		return r, invalid(err)
	}
	for _, old := range r.Releases {
		if old.Release.Version == doc.Release.Version {
			return r, types.NewValidationError("version already published").Because("pack.versionExists")
		}
	}
	if len(r.Releases) >= s.limits.PackReleases {
		return r, types.LimitReached("packReleases", s.limits.PackReleases)
	}
	r.Releases = append(r.Releases, doc)
	r.Archived = false
	if err = s.repo.Save(ctx, r, revision); err != nil {
		return r, err
	}
	return s.repo.Get(ctx, id)
}
func (s *Service) Archive(ctx context.Context, u user.ID, id string, revision int) (domain.Record, error) {
	r, err := s.owned(ctx, u, id)
	if err != nil {
		return r, err
	}
	r.Archived = true
	if err = s.repo.Save(ctx, r, revision); err != nil {
		return r, err
	}
	return s.repo.Get(ctx, id)
}
func (s *Service) Export(ctx context.Context, u user.ID, id, version string) ([]byte, error) {
	r, err := s.Get(ctx, u, id)
	if err != nil {
		return nil, err
	}
	// A grant is to play with the pack at one table, not to carry it away.
	if r.Restricted && !s.Superadmin(ctx, u) {
		return nil, denied()
	}
	if version == "" {
		doc, _, err := s.Validate(ctx, u, id)
		if err != nil {
			return nil, invalid(err)
		}
		return doc.Data, nil
	}
	for _, d := range r.Releases {
		if d.Release.Version == version {
			return d.Data, nil
		}
	}
	return nil, denied()
}
func (s *Service) GroupShares(ctx context.Context, u user.ID, g string) ([]domain.Share, error) {
	if _, err := s.groups.MemberRole(ctx, group.ID(g), u); err != nil {
		return nil, denied()
	}
	return s.repo.Shares(ctx, g)
}
func (s *Service) Share(ctx context.Context, u user.ID, g, id, version string) error {
	if _, err := s.groups.MemberRole(ctx, group.ID(g), u); err != nil {
		return denied()
	}
	super := s.Superadmin(ctx, u)
	r, err := s.owned(ctx, u, id)
	if err != nil {
		// A restricted disk pack has no owner to share it; a superadmin does.
		if r, err = s.Get(ctx, u, id); err != nil || !r.Restricted || !super {
			return denied()
		}
	}
	if r.Archived {
		return denied()
	}
	l, err := s.Resolve(ctx, u, []domain.Release{{ID: id, Version: version}})
	if err != nil {
		return err
	}
	shared, err := s.repo.Shares(ctx, g)
	if err != nil {
		return err
	}
	groupAllowed := map[domain.Release]bool{}
	// Sharing another version of a pack the group has replaces it.
	// ponytail: count, then insert -- racing shares can overshoot by a few.
	if len(shared) >= s.limits.GroupPacks && !slices.ContainsFunc(shared, func(sh domain.Share) bool { return sh.Pack == id }) {
		return types.LimitReached("groupPacks", s.limits.GroupPacks)
	}
	for _, sh := range shared {
		for _, p := range sh.Lock.Packs {
			groupAllowed[p] = true
		}
	}
	for _, p := range l.Packs {
		dep, err := s.Get(ctx, u, p.ID)
		if err != nil {
			return err
		}
		// Somebody else's pack, or a restricted one the caller was only
		// granted: neither is theirs to pass on to another table.
		if (dep.Owner != "" && dep.Owner != u || dep.Restricted && !super) && !groupAllowed[p] {
			return types.NewAccessDeniedError("dependency cannot be shared").Because("pack.dependencyPrivate")
		}
	}
	return s.repo.PutShare(ctx, domain.Share{Group: g, Pack: id, Contributor: u, Lock: l})
}
func (s *Service) Unshare(ctx context.Context, u user.ID, g, id string) error {
	role, err := s.groups.MemberRole(ctx, group.ID(g), u)
	if err != nil {
		return denied()
	}
	shares, err := s.repo.Shares(ctx, g)
	if err != nil {
		return err
	}
	for _, sh := range shares {
		if sh.Pack == id {
			if sh.Contributor != u && !role.AtLeast(group.RoleDM) {
				return denied()
			}
			return s.repo.DeleteShare(ctx, g, id)
		}
	}
	return denied()
}

// Import converts a directory archive before creating the same independent draft as JSON.
func (s *Service) Import(ctx context.Context, u user.User, title string, data []byte, zip bool) (domain.Record, error) {
	if zip {
		var err error
		data, err = s.engine.ImportZIP(data)
		if err != nil {
			return domain.Record{}, invalid(err)
		}
	}
	return s.Create(ctx, u, title, data, nil)
}
