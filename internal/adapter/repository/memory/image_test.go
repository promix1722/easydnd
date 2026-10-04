package memory_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/imageasset"
)

func TestImageRepository(t *testing.T) {
	repotest.RunImageRepository(t, func(*testing.T) imageasset.Repository {
		return memory.NewImageRepository()
	})
}
