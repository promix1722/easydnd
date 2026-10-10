package memory_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
)

// TestSharedRepository and TestGameRepository run the shared port contracts
// against the in-process stores. The Postgres adapter runs the identical
// tables.
func TestSharedRepository(t *testing.T) {
	repotest.RunSharedRepository(t, func(*testing.T) (domain.SharedRepository, group.Repository, user.Repository) {
		users := memory.NewUserRepository()
		return memory.NewSharedRepository(), memory.NewGroupRepository(users), users
	})
}

func TestGameRepository(t *testing.T) {
	repotest.RunGameRepository(t, func(*testing.T) (domain.Repository, group.Repository, user.Repository) {
		users := memory.NewUserRepository()
		return memory.NewGameRepository(), memory.NewGroupRepository(users), users
	})
}

// Compile-time proof, kept beside the contracts that exercise it.
var (
	_ domain.SharedRepository = (*memory.SharedRepository)(nil)
	_ domain.Repository       = (*memory.GameRepository)(nil)
)
