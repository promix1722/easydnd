package postgres_test

import (
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
)

// TestSharedRepository and TestGameRepository run the shared port contracts
// against a real database. Same skip as TestUserRepository. Groups are
// truncated too because both tables hang off them by foreign key, and the
// suites seed the ones they use.
func TestSharedRepository(t *testing.T) {
	cfg := testConfig(t)

	repotest.RunSharedRepository(t, func(t *testing.T) (domain.SharedRepository, group.Repository, user.Repository) {
		pool := testPool(t, cfg)
		if _, err := pool.Exec(context.Background(), `TRUNCATE users, groups CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewSharedRepository(pool), postgres.NewGroupRepository(pool), postgres.NewUserRepository(pool)
	})
}

func TestGameRepository(t *testing.T) {
	cfg := testConfig(t)

	repotest.RunGameRepository(t, func(t *testing.T) (domain.Repository, group.Repository, user.Repository) {
		pool := testPool(t, cfg)
		if _, err := pool.Exec(context.Background(), `TRUNCATE users, groups CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewGameRepository(pool), postgres.NewGroupRepository(pool), postgres.NewUserRepository(pool)
	})
}
