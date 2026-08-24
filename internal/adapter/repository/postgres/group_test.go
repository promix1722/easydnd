package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	group "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// TestGroupRepository runs the shared port contract against a real database.
//
// Same skip as TestUserRepository: without TEST_DATABASE_URL there is nothing
// to run against, and `make verify` has to stay green on a machine with no
// Postgres.
func TestGroupRepository(t *testing.T) {
	cfg := testConfig(t)

	repotest.RunGroupRepository(t, func(t *testing.T) (group.Repository, user.Repository) {
		pool := testPool(t, cfg)
		// CASCADE reaches group_members through both of its foreign keys, so
		// truncating users alone would leave rosters behind. Naming groups as
		// well makes that explicit rather than relying on the cascade.
		if _, err := pool.Exec(context.Background(), `TRUNCATE users, groups CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewGroupRepository(pool), postgres.NewUserRepository(pool)
	})
}

// TestAMemberMustBeARealAccount is the one thing the in-memory adapter cannot
// express, and therefore the one thing the shared contract cannot assert.
//
// It is what makes the group usecase's ensureStored load-bearing rather than
// decorative: a guest who has not been materialised has no users row, and the
// foreign key is what turns that into a failure instead of a roster entry with
// nobody behind it. The error is deliberately a server error -- a violation
// here means we forgot to write the row, which is our bug and not the
// caller's.
func TestAMemberMustBeARealAccount(t *testing.T) {
	cfg := testConfig(t)
	pool := testPool(t, cfg)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE users, groups CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	users := postgres.NewUserRepository(pool)
	groups := postgres.NewGroupRepository(pool)
	if err := users.Create(ctx, repotest.Account("alice")); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	created := group.Group{
		ID: "g1", Name: "Table", CreatedBy: "alice", CreatedAt: time.Unix(100, 0).UTC(),
	}
	if err := groups.Create(ctx, created, "alice"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	ghost := user.ID(user.AnonymousIDPrefix + "never-stored")
	err := groups.AddMember(ctx, "g1", ghost, group.RolePlayer, time.Unix(200, 0).UTC())
	if err == nil {
		t.Fatal("AddMember admitted a member with no account row")
	}
	if types.IsNotFound(err) {
		t.Errorf("AddMember = %v, want a server error rather than a 404", err)
	}
}
