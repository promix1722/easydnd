package memory_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/pack"
)

func TestPackRepository(t *testing.T) {
	repotest.RunPackRepository(t, func(*testing.T) pack.Repository { return memory.NewPackRepository() })
}
