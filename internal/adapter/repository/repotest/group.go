package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	group "github.com/promix1722/easydnd/internal/domain/group"
	user "github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// NewGroupRepository builds an empty store for one subtest, together with the
// account store it reads display names out of.
//
// Both, and not just the group store, because a roster is a join: the Postgres
// adapter reads names from users and the in-memory one is handed the same
// repository so that it behaves identically. A contract that could only see
// the group half would let the two disagree about the one thing a roster is
// for.
type NewGroupRepository func(t *testing.T) (group.Repository, user.Repository)

// groupAt builds a group stamped at a whole second -- see Account for why
// whole seconds, and why every comparison below uses time.Time.Equal.
func groupAt(id, name string, owner user.ID, sec int64) group.Group {
	return group.Group{
		ID:        group.ID(id),
		Name:      name,
		CreatedBy: owner,
		CreatedAt: time.Unix(sec, 0).UTC(),
	}
}

// joined stamps a membership time, on a whole second for the reason Account
// gives.
func joined(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

// RunGroupRepository runs the whole port contract against one implementation.
func RunGroupRepository(t *testing.T, newRepos NewGroupRepository) {
	t.Helper()

	// setup seeds n accounts named a, b, c... and returns both stores. Every
	// membership needs a real account behind it: in Postgres that is a foreign
	// key, and a fixture that skipped it would pass in memory and fail there.
	setup := func(t *testing.T, ids ...string) (group.Repository, user.Repository) {
		t.Helper()
		groups, users := newRepos(t)
		for _, id := range ids {
			if err := users.Create(context.Background(), Account(id)); err != nil {
				t.Fatalf("seed account %q: %v", id, err)
			}
		}
		return groups, users
	}

	tests := []struct {
		name string
		run  func(t *testing.T, groups group.Repository, users user.Repository)
	}{
		{
			name: "create seats the owner and loads back",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				g := groupAt("g1", "Wednesday Night", "alice", 100)
				if err := groups.Create(ctx, g, "alice"); err != nil {
					t.Fatalf("Create: %v", err)
				}

				loaded, err := groups.ByID(ctx, "g1")
				if err != nil {
					t.Fatalf("ByID: %v", err)
				}
				if loaded.Name != "Wednesday Night" {
					t.Errorf("name = %q, want %q", loaded.Name, "Wednesday Night")
				}
				if !loaded.CreatedAt.Equal(g.CreatedAt) {
					t.Errorf("created at = %v, want %v", loaded.CreatedAt, g.CreatedAt)
				}
				role, err := groups.MemberRole(ctx, "g1", "alice")
				if err != nil {
					t.Fatalf("MemberRole: %v", err)
				}
				if role != group.RoleOwner {
					t.Errorf("role = %q, want %q", role, group.RoleOwner)
				}
			},
		},
		{
			name: "create rejects a duplicate id",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				if err := groups.Create(ctx, groupAt("g1", "First", "alice", 100), "alice"); err != nil {
					t.Fatalf("Create: %v", err)
				}
				err := groups.Create(ctx, groupAt("g1", "Second", "alice", 200), "alice")
				var want *types.ValidationError
				if !errors.As(err, &want) {
					t.Fatalf("Create = %v, want a validation error", err)
				}
			},
		},
		{
			name: "byID of a missing group is not found",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				if _, err := groups.ByID(context.Background(), "nope"); !types.IsNotFound(err) {
					t.Fatalf("ByID = %v, want not found", err)
				}
			},
		},
		{
			name: "roster names every member and orders owner first",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "carol", group.RolePlayer, joined(300))
				mustAdd(t, groups, "g1", "bob", group.RoleDM, joined(200))

				members, err := groups.Members(ctx, "g1")
				if err != nil {
					t.Fatalf("Members: %v", err)
				}
				gotIDs := make([]string, 0, len(members))
				for _, m := range members {
					gotIDs = append(gotIDs, string(m.UserID))
					// The name comes from the account store, never from a copy
					// kept beside the membership.
					if m.DisplayName != string(m.UserID) {
						t.Errorf("member %q display name = %q, want %q",
							m.UserID, m.DisplayName, m.UserID)
					}
				}
				want := []string{"alice", "bob", "carol"}
				if len(gotIDs) != len(want) {
					t.Fatalf("roster = %v, want %v", gotIDs, want)
				}
				for i := range want {
					if gotIDs[i] != want[i] {
						t.Errorf("roster = %v, want %v", gotIDs, want)
						break
					}
				}
				if !members[0].JoinedAt.Equal(time.Unix(100, 0).UTC()) {
					t.Errorf("owner joined at = %v, want the group's creation time",
						members[0].JoinedAt)
				}
			},
		},
		{
			name: "roster of a missing group is not found",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				if _, err := groups.Members(context.Background(), "nope"); !types.IsNotFound(err) {
					t.Fatalf("Members = %v, want not found", err)
				}
			},
		},
		{
			name: "listFor returns only my groups, newest first",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("old", "Older", "alice", 100), "alice")
				mustCreate(t, groups, groupAt("new", "Newer", "alice", 200), "alice")
				mustCreate(t, groups, groupAt("theirs", "Theirs", "bob", 300), "bob")

				mine, err := groups.ListFor(ctx, "alice")
				if err != nil {
					t.Fatalf("ListFor: %v", err)
				}
				if len(mine) != 2 {
					t.Fatalf("ListFor returned %d groups, want 2", len(mine))
				}
				if mine[0].Group.ID != "new" || mine[1].Group.ID != "old" {
					t.Errorf("order = %q, %q; want newest first",
						mine[0].Group.ID, mine[1].Group.ID)
				}
				if mine[0].Role != group.RoleOwner {
					t.Errorf("role = %q, want %q", mine[0].Role, group.RoleOwner)
				}
			},
		},
		{
			name: "memberRole of a non-member is not found",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				_, err := groups.MemberRole(context.Background(), "g1", "bob")
				if !types.IsNotFound(err) {
					t.Fatalf("MemberRole = %v, want not found", err)
				}
			},
		},
		{
			name: "addMember rejects a duplicate and rejects an owner",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "bob", group.RolePlayer, joined(200))

				var validation *types.ValidationError
				if err := groups.AddMember(ctx, "g1", "bob", group.RolePlayer, joined(300)); !errors.As(err, &validation) {
					t.Errorf("AddMember(duplicate) = %v, want a validation error", err)
				}
				// An owner is made by Create and moved by TransferOwnership.
				// Postgres refuses a second one outright; this keeps the
				// in-memory adapter honest about the same rule.
				if err := groups.AddMember(ctx, "g1", "carol", group.RoleOwner, joined(300)); !errors.As(err, &validation) {
					t.Errorf("AddMember(owner) = %v, want a validation error", err)
				}
			},
		},
		{
			name: "addMember to a missing group is not found",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				err := groups.AddMember(context.Background(), "nope", "bob", group.RolePlayer, joined(200))
				if !types.IsNotFound(err) {
					t.Fatalf("AddMember = %v, want not found", err)
				}
			},
		},
		{
			name: "setRole moves a member and refuses the owner",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "bob", group.RolePlayer, joined(200))

				if err := groups.SetRole(ctx, "g1", "bob", group.RoleDM); err != nil {
					t.Fatalf("SetRole: %v", err)
				}
				if role, _ := groups.MemberRole(ctx, "g1", "bob"); role != group.RoleDM {
					t.Errorf("role = %q, want %q", role, group.RoleDM)
				}

				var validation *types.ValidationError
				if err := groups.SetRole(ctx, "g1", "alice", group.RolePlayer); !errors.As(err, &validation) {
					t.Errorf("SetRole(owner) = %v, want a validation error", err)
				}
				if err := groups.SetRole(ctx, "g1", "bob", group.RoleOwner); !errors.As(err, &validation) {
					t.Errorf("SetRole(to owner) = %v, want a validation error", err)
				}
				if err := groups.SetRole(ctx, "g1", "carol", group.RoleDM); !types.IsNotFound(err) {
					t.Errorf("SetRole(non-member) = %v, want not found", err)
				}
			},
		},
		{
			name: "removeMember unseats a member and refuses the owner",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "bob", group.RolePlayer, joined(200))

				if err := groups.RemoveMember(ctx, "g1", "bob"); err != nil {
					t.Fatalf("RemoveMember: %v", err)
				}
				if _, err := groups.MemberRole(ctx, "g1", "bob"); !types.IsNotFound(err) {
					t.Errorf("MemberRole after removal = %v, want not found", err)
				}
				if err := groups.RemoveMember(ctx, "g1", "bob"); !types.IsNotFound(err) {
					t.Errorf("RemoveMember(twice) = %v, want not found", err)
				}

				// Storage refuses this as well as the usecase, so that no
				// ordering of concurrent calls can leave a group ownerless.
				var validation *types.ValidationError
				if err := groups.RemoveMember(ctx, "g1", "alice"); !errors.As(err, &validation) {
					t.Errorf("RemoveMember(owner) = %v, want a validation error", err)
				}
			},
		},
		{
			name: "transfer leaves exactly one owner and demotes the old one",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "bob", group.RolePlayer, joined(200))

				if err := groups.TransferOwnership(ctx, "g1", "alice", "bob"); err != nil {
					t.Fatalf("TransferOwnership: %v", err)
				}
				assertOneOwner(t, groups, "g1", "bob")
				if role, _ := groups.MemberRole(ctx, "g1", "alice"); role != group.RoleDM {
					t.Errorf("outgoing owner is %q, want %q", role, group.RoleDM)
				}
			},
		},
		{
			name: "transfer twice in a row still leaves one owner",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				// A→B→C. This is what catches a promote-before-demote
				// implementation: the second hop starts from a group whose
				// owner row has already moved once, and against Postgres the
				// wrong order trips group_members_one_owner_idx.
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "bob", group.RolePlayer, joined(200))
				mustAdd(t, groups, "g1", "carol", group.RolePlayer, joined(300))

				if err := groups.TransferOwnership(ctx, "g1", "alice", "bob"); err != nil {
					t.Fatalf("TransferOwnership(alice->bob): %v", err)
				}
				if err := groups.TransferOwnership(ctx, "g1", "bob", "carol"); err != nil {
					t.Fatalf("TransferOwnership(bob->carol): %v", err)
				}
				assertOneOwner(t, groups, "g1", "carol")
			},
		},
		{
			name: "transfer refuses a non-owner and a non-member",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "bob", group.RolePlayer, joined(200))

				var validation *types.ValidationError
				if err := groups.TransferOwnership(ctx, "g1", "bob", "alice"); !errors.As(err, &validation) {
					t.Errorf("TransferOwnership(from a non-owner) = %v, want a validation error", err)
				}
				if err := groups.TransferOwnership(ctx, "g1", "alice", "carol"); !types.IsNotFound(err) {
					t.Errorf("TransferOwnership(to a non-member) = %v, want not found", err)
				}
				assertOneOwner(t, groups, "g1", "alice")
			},
		},
		{
			name: "rename and delete",
			run: func(t *testing.T, groups group.Repository, _ user.Repository) {
				ctx := context.Background()
				mustCreate(t, groups, groupAt("g1", "Before", "alice", 100), "alice")
				mustAdd(t, groups, "g1", "bob", group.RolePlayer, joined(200))

				if err := groups.Rename(ctx, "g1", "After"); err != nil {
					t.Fatalf("Rename: %v", err)
				}
				if loaded, _ := groups.ByID(ctx, "g1"); loaded.Name != "After" {
					t.Errorf("name = %q, want %q", loaded.Name, "After")
				}
				if err := groups.Rename(ctx, "nope", "After"); !types.IsNotFound(err) {
					t.Errorf("Rename(missing) = %v, want not found", err)
				}

				if err := groups.Delete(ctx, "g1"); err != nil {
					t.Fatalf("Delete: %v", err)
				}
				if _, err := groups.ByID(ctx, "g1"); !types.IsNotFound(err) {
					t.Errorf("ByID after delete = %v, want not found", err)
				}
				// The memberships went with it rather than being orphaned.
				if _, err := groups.MemberRole(ctx, "g1", "bob"); !types.IsNotFound(err) {
					t.Errorf("MemberRole after delete = %v, want not found", err)
				}
				if err := groups.Delete(ctx, "g1"); !types.IsNotFound(err) {
					t.Errorf("Delete(twice) = %v, want not found", err)
				}
			},
		},
		{
			name: "a guest is a member like anybody else",
			run: func(t *testing.T, groups group.Repository, users user.Repository) {
				ctx := context.Background()
				guestID := user.ID(user.AnonymousIDPrefix + "wanderer")
				err := users.EnsureGuest(ctx, user.User{
					ID: guestID, DisplayName: "Wintry Otter", CreatedAt: joined(150),
				})
				if err != nil {
					t.Fatalf("EnsureGuest: %v", err)
				}

				mustCreate(t, groups, groupAt("g1", "Table", "alice", 100), "alice")
				mustAdd(t, groups, "g1", guestID, group.RolePlayer, joined(200))

				members, err := groups.Members(ctx, "g1")
				if err != nil {
					t.Fatalf("Members: %v", err)
				}
				var found bool
				for _, m := range members {
					if m.UserID != guestID {
						continue
					}
					found = true
					if !m.Anonymous() {
						t.Error("guest member does not report as anonymous")
					}
					if m.DisplayName != "Wintry Otter" {
						t.Errorf("guest display name = %q, want %q", m.DisplayName, "Wintry Otter")
					}
				}
				if !found {
					t.Fatal("the guest is not in the roster")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups, users := setup(t, "alice", "bob", "carol")
			tc.run(t, groups, users)
		})
	}
}

// mustCreate seats an owner in a new group or fails the test.
func mustCreate(t *testing.T, groups group.Repository, g group.Group, owner user.ID) {
	t.Helper()
	if err := groups.Create(context.Background(), g, owner); err != nil {
		t.Fatalf("Create(%q): %v", g.ID, err)
	}
}

// mustAdd seats a member or fails the test.
func mustAdd(
	t *testing.T, groups group.Repository, id group.ID, u user.ID, role group.Role, at time.Time,
) {
	t.Helper()
	if err := groups.AddMember(context.Background(), id, u, role, at); err != nil {
		t.Fatalf("AddMember(%q, %q): %v", id, u, err)
	}
}

// assertOneOwner is the invariant the whole role model rests on.
func assertOneOwner(t *testing.T, groups group.Repository, id group.ID, want user.ID) {
	t.Helper()
	members, err := groups.Members(context.Background(), id)
	if err != nil {
		t.Fatalf("Members: %v", err)
	}
	owners := make([]user.ID, 0, 1)
	for _, m := range members {
		if m.Role == group.RoleOwner {
			owners = append(owners, m.UserID)
		}
	}
	if len(owners) != 1 {
		t.Fatalf("group %q has %d owners (%v), want exactly 1", id, len(owners), owners)
	}
	if owners[0] != want {
		t.Errorf("owner = %q, want %q", owners[0], want)
	}
}
