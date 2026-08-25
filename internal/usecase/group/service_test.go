package group_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	groupuc "github.com/promix1722/easydnd/internal/usecase/group"
)

// fakeInviter stands in for the token adapter.
//
// It renders an invite as its own JSON, which makes a token readable in a
// failure message and lets a test forge one. What it keeps faithful is the
// part the usecase depends on: every rejection is an UnauthenticatedError,
// exactly as the real signer's is, because the translation of that into a 400
// is the subtlest thing in this package.
type fakeInviter struct{ broken bool }

func (f *fakeInviter) SignInvite(i domain.Invite) (string, error) {
	raw, err := json.Marshal(i)
	return string(raw), err
}

func (f *fakeInviter) VerifyInvite(token string, now time.Time) (domain.Invite, error) {
	var i domain.Invite
	if f.broken || json.Unmarshal([]byte(token), &i) != nil {
		return domain.Invite{}, types.NewUnauthenticatedError("invite token is not valid")
	}
	if now.After(i.ExpiresAt) {
		return domain.Invite{}, types.NewUnauthenticatedError("invite token has expired")
	}
	return i, nil
}

type fixture struct {
	svc     *groupuc.Service
	users   *memory.UserRepository
	inviter *fakeInviter
}

// newFixture seeds four accounts and an empty group store.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	users := memory.NewUserRepository()
	for _, id := range []string{"alice", "bob", "carol", "dave"} {
		err := users.Create(context.Background(), user.User{
			ID: user.ID(id), DisplayName: id, CreatedAt: time.Unix(0, 0).UTC(),
		})
		if err != nil {
			t.Fatalf("seed %q: %v", id, err)
		}
	}
	inviter := &fakeInviter{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &fixture{
		svc:     groupuc.NewService(memory.NewGroupRepository(users), users, inviter, log),
		users:   users,
		inviter: inviter,
	}
}

func account(id string) user.User {
	return user.User{ID: user.ID(id), DisplayName: id, CreatedAt: time.Unix(0, 0).UTC()}
}

func guest(id string) user.User {
	return user.User{
		ID:          user.ID(user.AnonymousIDPrefix + id),
		DisplayName: "Wintry " + id,
		CreatedAt:   time.Unix(0, 0).UTC(),
		Anonymous:   true,
	}
}

// table builds a group owned by alice with bob as a DM and carol as a player.
// dave is deliberately left out of it, and is the non-member every enumeration
// assertion below uses.
func (f *fixture) table(t *testing.T) domain.ID {
	t.Helper()
	ctx := context.Background()
	g, err := f.svc.Create(ctx, account("alice"), "Wednesday Night")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	f.seat(t, g.ID, "alice", account("bob"), domain.RoleDM)
	f.seat(t, g.ID, "alice", account("carol"), domain.RolePlayer)
	return g.ID
}

// seat invites somebody and has them accept, which is the only way into a
// group and therefore the only way a fixture should build one.
func (f *fixture) seat(t *testing.T, id domain.ID, host string, who user.User, role domain.Role) {
	t.Helper()
	ctx := context.Background()
	invitation, err := f.svc.Invite(ctx, user.ID(host), id, role)
	if err != nil {
		t.Fatalf("Invite(%s as %s): %v", who.ID, role, err)
	}
	if _, err := f.svc.Accept(ctx, who, invitation.Token); err != nil {
		t.Fatalf("Accept(%s): %v", who.ID, err)
	}
}

func (f *fixture) roleOf(t *testing.T, id domain.ID, who string) domain.Role {
	t.Helper()
	_, role, _, err := f.svc.Get(context.Background(), user.ID(who), id)
	if err != nil {
		t.Fatalf("Get(%s): %v", who, err)
	}
	return role
}

// assertNotFound is the enumeration guarantee: somebody outside a group must
// not be able to tell it apart from one that never existed.
func assertNotFound(t *testing.T, err error, what string) {
	t.Helper()
	if !types.IsNotFound(err) {
		t.Errorf("%s = %v, want a not-found error", what, err)
	}
}

// assertDenied is the other half: somebody inside a group who lacks a right
// gets told so, because they can already see the group and pretending
// otherwise would only confuse them.
func assertDenied(t *testing.T, err error, what string) {
	t.Helper()
	var denied *types.AccessDeniedError
	if !errors.As(err, &denied) {
		t.Errorf("%s = %v, want an access-denied error", what, err)
	}
}

func TestCreatorOwnsTheGroup(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)

	if got := f.roleOf(t, id, "alice"); got != domain.RoleOwner {
		t.Errorf("creator's role = %q, want %q", got, domain.RoleOwner)
	}
	_, _, members, err := f.svc.Get(context.Background(), "alice", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("roster has %d members, want 3", len(members))
	}
	if members[0].UserID != "alice" || members[0].Role != domain.RoleOwner {
		t.Errorf("roster does not lead with the owner: %+v", members[0])
	}
}

// A non-member gets 404 everywhere, and never 403. A 403 on somebody else's id
// confirms the id exists, which turns a guessable identifier into an
// enumeration oracle.
func TestANonMemberSeesNothing(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	_, _, _, err := f.svc.Get(ctx, "dave", id)
	assertNotFound(t, err, "Get")
	_, err = f.svc.Rename(ctx, "dave", id, "Theirs")
	assertNotFound(t, err, "Rename")
	assertNotFound(t, f.svc.Delete(ctx, "dave", id), "Delete")
	_, err = f.svc.Invite(ctx, "dave", id, domain.RolePlayer)
	assertNotFound(t, err, "Invite")
	assertNotFound(t, f.svc.SetRole(ctx, "dave", id, "carol", domain.RoleDM), "SetRole")
	assertNotFound(t, f.svc.RemoveMember(ctx, "dave", id, "carol"), "RemoveMember")

	// And the group is not in their list.
	mine, err := f.svc.List(ctx, "dave")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(mine) != 0 {
		t.Errorf("a non-member lists %d groups, want 0", len(mine))
	}
}

func TestAPlayerMayOnlyLook(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	// Reading is every member's right.
	if _, _, _, err := f.svc.Get(ctx, "carol", id); err != nil {
		t.Fatalf("Get: %v", err)
	}

	_, err := f.svc.Rename(ctx, "carol", id, "Mine now")
	assertDenied(t, err, "Rename")
	assertDenied(t, f.svc.Delete(ctx, "carol", id), "Delete")
	_, err = f.svc.Invite(ctx, "carol", id, domain.RolePlayer)
	assertDenied(t, err, "Invite")
	assertDenied(t, f.svc.SetRole(ctx, "carol", id, "bob", domain.RolePlayer), "SetRole")
	assertDenied(t, f.svc.RemoveMember(ctx, "carol", id, "bob"), "RemoveMember")
}

func TestADMRunsTheTableButDoesNotOwnIt(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	if _, err := f.svc.Rename(ctx, "bob", id, "Thursday Night"); err != nil {
		t.Errorf("a DM could not rename: %v", err)
	}
	if _, err := f.svc.Invite(ctx, "bob", id, domain.RoleDM); err != nil {
		t.Errorf("a DM could not invite: %v", err)
	}
	if err := f.svc.RemoveMember(ctx, "bob", id, "carol"); err != nil {
		t.Errorf("a DM could not remove a player: %v", err)
	}

	// What a DM may not do.
	assertDenied(t, f.svc.Delete(ctx, "bob", id), "Delete")
	assertDenied(t, f.svc.SetRole(ctx, "bob", id, "dave", domain.RoleDM), "SetRole")
	assertDenied(t, f.svc.RemoveMember(ctx, "bob", id, "alice"), "RemoveMember(owner)")
}

// The user asked for this one by name: DMs manage DMs too.
func TestADMMayRemoveAnotherDM(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	f.seat(t, id, "alice", account("dave"), domain.RoleDM)

	if err := f.svc.RemoveMember(context.Background(), "bob", id, "dave"); err != nil {
		t.Errorf("a DM could not remove another DM: %v", err)
	}
}

// Nobody may remove or demote the owner -- not a DM, and not the owner
// themselves by aiming the removal at their own id.
func TestTheOwnerCannotBeUnseated(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	assertDenied(t, f.svc.RemoveMember(ctx, "bob", id, "alice"), "a DM removing the owner")
	assertDenied(t, f.svc.RemoveMember(ctx, "alice", id, "alice"), "the owner leaving")
	assertDenied(t, f.svc.SetRole(ctx, "alice", id, "alice", domain.RolePlayer),
		"the owner demoting themselves")

	if got := f.roleOf(t, id, "alice"); got != domain.RoleOwner {
		t.Errorf("owner's role = %q, want %q", got, domain.RoleOwner)
	}
}

// The sequence the whole role model is built around: an owner who wants out
// hands the group on, becomes a DM, and can then leave. Nobody is trapped and
// the group is never ownerless.
func TestTransferThenLeave(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	if err := f.svc.SetRole(ctx, "alice", id, "bob", domain.RoleOwner); err != nil {
		t.Fatalf("SetRole(owner): %v", err)
	}
	if got := f.roleOf(t, id, "bob"); got != domain.RoleOwner {
		t.Errorf("new owner's role = %q, want %q", got, domain.RoleOwner)
	}
	if got := f.roleOf(t, id, "alice"); got != domain.RoleDM {
		t.Errorf("outgoing owner's role = %q, want %q", got, domain.RoleDM)
	}

	// Now she may leave, which a moment ago she could not.
	if err := f.svc.RemoveMember(ctx, "alice", id, "alice"); err != nil {
		t.Fatalf("the ex-owner could not leave: %v", err)
	}
	_, _, members, err := f.svc.Get(ctx, "bob", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	owners := 0
	for _, m := range members {
		if m.Role == domain.RoleOwner {
			owners++
		}
		if m.UserID == "alice" {
			t.Error("the ex-owner is still in the roster")
		}
	}
	if owners != 1 {
		t.Errorf("group has %d owners, want exactly 1", owners)
	}
}

// Deleting is the other way out, and the reason a sole owner is never stuck.
func TestOnlyTheOwnerMayDelete(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	assertDenied(t, f.svc.Delete(ctx, "bob", id), "a DM deleting")
	if err := f.svc.Delete(ctx, "alice", id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, _, _, err := f.svc.Get(ctx, "alice", id)
	assertNotFound(t, err, "Get after delete")
}

func TestOnlyTheOwnerMayChangeRoles(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	if err := f.svc.SetRole(ctx, "alice", id, "carol", domain.RoleDM); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if got := f.roleOf(t, id, "carol"); got != domain.RoleDM {
		t.Errorf("role = %q, want %q", got, domain.RoleDM)
	}
	// Setting the role somebody already has is a no-op, not an error.
	if err := f.svc.SetRole(ctx, "alice", id, "carol", domain.RoleDM); err != nil {
		t.Errorf("SetRole(unchanged) = %v, want no error", err)
	}
	// A person who is not in the group is a 404, not a 403: the actor can
	// already read the roster, so this reveals nothing they did not know.
	assertNotFound(t, f.svc.SetRole(ctx, "alice", id, "dave", domain.RoleDM), "SetRole(stranger)")
}

func TestAnInviteCannotOfferOwnership(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)

	_, err := f.svc.Invite(context.Background(), "alice", id, domain.RoleOwner)
	var fields *types.FieldValidationError
	if !errors.As(err, &fields) {
		t.Fatalf("Invite(owner) = %v, want a field validation error", err)
	}
}

// The link is reusable, so a second click must succeed and change nothing --
// and in particular must not re-rank somebody who is already seated.
func TestAcceptingTwiceIsANoOp(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	invitation, err := f.svc.Invite(ctx, "alice", id, domain.RolePlayer)
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	// bob is already a DM. Redeeming a player link must not demote him.
	if _, err := f.svc.Accept(ctx, account("bob"), invitation.Token); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if got := f.roleOf(t, id, "bob"); got != domain.RoleDM {
		t.Errorf("role after re-accepting = %q, want %q", got, domain.RoleDM)
	}

	// And the same link seats a second, different person.
	if _, err := f.svc.Accept(ctx, account("dave"), invitation.Token); err != nil {
		t.Fatalf("Accept(dave): %v", err)
	}
	if got := f.roleOf(t, id, "dave"); got != domain.RolePlayer {
		t.Errorf("dave's role = %q, want %q", got, domain.RolePlayer)
	}
}

// The subtlest correctness point in the feature.
//
// Every token port reports failure as an UnauthenticatedError, which the HTTP
// layer renders as 401 -- and a 401 is what tells this application's client
// that the session is gone and to show the landing page. A stale invite link
// must therefore NOT surface as one, or clicking yesterday's link would sign
// out the perfectly-signed-in person who clicked it.
func TestAStaleLinkDoesNotSignAnybodyOut(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	invitation, err := f.svc.Invite(ctx, "alice", id, domain.RolePlayer)
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
		setup func()
	}{
		{name: "forged", token: "not-a-token"},
		{name: "rejected by the signer", token: invitation.Token, setup: func() { f.inviter.broken = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
				defer func() { f.inviter.broken = false }()
			}
			_, err := f.svc.Accept(ctx, account("dave"), tc.token)
			if types.IsUnauthenticated(err) {
				t.Fatalf("Accept = %v, which would sign the caller out", err)
			}
			var validation *types.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("Accept = %v, want a validation error", err)
			}

			_, err = f.svc.PreviewInvite(ctx, "dave", tc.token)
			if types.IsUnauthenticated(err) {
				t.Fatalf("PreviewInvite = %v, which would sign the caller out", err)
			}
		})
	}
}

func TestAnInviteToADeletedGroupSaysSo(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	invitation, err := f.svc.Invite(ctx, "alice", id, domain.RolePlayer)
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if err := f.svc.Delete(ctx, "alice", id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = f.svc.Accept(ctx, account("dave"), invitation.Token)
	assertNotFound(t, err, "Accept into a deleted group")
}

func TestPreviewNamesTheGroupAndTheInviter(t *testing.T) {
	f := newFixture(t)
	id := f.table(t)
	ctx := context.Background()

	invitation, err := f.svc.Invite(ctx, "bob", id, domain.RoleDM)
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}

	preview, err := f.svc.PreviewInvite(ctx, "dave", invitation.Token)
	if err != nil {
		t.Fatalf("PreviewInvite: %v", err)
	}
	if preview.Group.Name != "Wednesday Night" {
		t.Errorf("group name = %q, want %q", preview.Group.Name, "Wednesday Night")
	}
	if preview.Role != domain.RoleDM {
		t.Errorf("role = %q, want %q", preview.Role, domain.RoleDM)
	}
	if preview.InvitedBy != "bob" {
		t.Errorf("invited by = %q, want %q", preview.InvitedBy, "bob")
	}
	if preview.AlreadyMember {
		t.Error("a stranger is reported as already a member")
	}

	// Somebody already seated is told so, rather than being offered a button
	// whose effect they cannot see.
	seated, err := f.svc.PreviewInvite(ctx, "carol", invitation.Token)
	if err != nil {
		t.Fatalf("PreviewInvite(member): %v", err)
	}
	if !seated.AlreadyMember {
		t.Error("an existing member is not reported as one")
	}
}

// A guest is a full participant, and the row that makes them nameable in
// somebody else's roster is written the moment they join and not before.
func TestAGuestCanCreateAndJoin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wanderer := guest("wanderer")

	// Nothing is stored until they do something that needs it.
	if _, err := f.users.ByID(ctx, wanderer.ID); !types.IsNotFound(err) {
		t.Fatalf("a guest was stored before joining anything: %v", err)
	}

	own, err := f.svc.Create(ctx, wanderer, "Guest's table")
	if err != nil {
		t.Fatalf("a guest could not create a group: %v", err)
	}
	stored, err := f.users.ByID(ctx, wanderer.ID)
	if err != nil {
		t.Fatalf("the guest's row was not written: %v", err)
	}
	if stored.DisplayName != wanderer.DisplayName {
		t.Errorf("stored name = %q, want %q", stored.DisplayName, wanderer.DisplayName)
	}
	if got := f.roleOf(t, own.ID, string(wanderer.ID)); got != domain.RoleOwner {
		t.Errorf("guest's role in their own group = %q, want %q", got, domain.RoleOwner)
	}

	// And they can be invited into somebody else's, where the roster names
	// them and marks them as a guest.
	id := f.table(t)
	f.seat(t, id, "alice", wanderer, domain.RolePlayer)
	_, _, members, err := f.svc.Get(ctx, "alice", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	var found bool
	for _, m := range members {
		if m.UserID != wanderer.ID {
			continue
		}
		found = true
		if !m.Anonymous() {
			t.Error("the guest is not marked anonymous in the roster")
		}
		if m.DisplayName != wanderer.DisplayName {
			t.Errorf("guest name = %q, want %q", m.DisplayName, wanderer.DisplayName)
		}
	}
	if !found {
		t.Error("the guest is not in the roster")
	}
}

func TestCreateValidatesTheName(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	for _, name := range []string{"", "   "} {
		_, err := f.svc.Create(ctx, account("alice"), name)
		var fields *types.FieldValidationError
		if !errors.As(err, &fields) {
			t.Errorf("Create(%q) = %v, want a field validation error", name, err)
		}
	}

	long := ""
	for range 65 {
		long += "x"
	}
	if _, err := f.svc.Create(ctx, account("alice"), long); err == nil {
		t.Error("Create accepted a 65-character name")
	}

	// Names are trimmed rather than rejected for stray whitespace.
	g, err := f.svc.Create(ctx, account("alice"), "  Padded  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if g.Name != "Padded" {
		t.Errorf("name = %q, want %q", g.Name, "Padded")
	}
}
