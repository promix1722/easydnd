package memory_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// TestFolderRepository runs the shared port contract against the in-process
// store. The Postgres adapter runs the identical table.
func TestFolderRepository(t *testing.T) {
	repotest.RunFolderRepository(t, func(*testing.T) domain.FolderRepository {
		return memory.NewFolderRepository()
	})
}

// Compile-time proof, kept beside the contract that exercises it.
var _ domain.FolderRepository = (*memory.FolderRepository)(nil)
