package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
)

func TestPackRepository(t *testing.T) {
	cfg := testConfig(t)
	repotest.RunPackRepository(t, func(t *testing.T) pack.Repository {
		pool := testPool(t, cfg)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `TRUNCATE users, groups CASCADE`); err != nil {
			t.Fatal(err)
		}
		users := postgres.NewUserRepository(pool)
		if err := users.EnsureGuest(ctx, user.User{ID: "anon:author", Anonymous: true, DisplayName: "Author", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		groups := postgres.NewGroupRepository(pool)
		if err := groups.Create(ctx, group.Group{ID: "test-table", Name: "Table", CreatedBy: "anon:author", CreatedAt: time.Now()}, "anon:author"); err != nil {
			t.Fatal(err)
		}
		return postgres.NewPackRepository(pool)
	})
}
