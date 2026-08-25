package memory_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	group "github.com/promix1722/easydnd/internal/domain/group"
	user "github.com/promix1722/easydnd/internal/domain/user"
)

// TestGroupRepository runs the shared port contract against the in-process
// store. The Postgres adapter runs the identical table, which is the only
// thing that keeps the two able to stand in for one another.
func TestGroupRepository(t *testing.T) {
	repotest.RunGroupRepository(t, func(*testing.T) (group.Repository, user.Repository) {
		// One user store, shared with the group store: a roster is a join, and
		// two instances would render every member nameless.
		users := memory.NewUserRepository()
		return memory.NewGroupRepository(users), users
	})
}

// Compile-time proof, kept beside the contract that exercises it.
var _ group.Repository = (*memory.GroupRepository)(nil)
