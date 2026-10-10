package memory_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// TestCharacterRepository runs the shared port contract against the
// in-process store. The Postgres adapter runs the identical table.
func TestCharacterRepository(t *testing.T) {
	repotest.RunCharacterRepository(t, func(*testing.T) domain.Repository {
		return memory.NewCharacterRepository()
	})
}

// Compile-time proof, kept beside the contract that exercises it.
var _ domain.Repository = (*memory.CharacterRepository)(nil)
