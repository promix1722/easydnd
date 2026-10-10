package agent_test

import (
	"log/slog"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file/filetest"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

const testOwner domain.OwnerID = "test-owner"

// One Source for every service these tests build, as in the character
// package's tests and for the same reason: a Catalog is immutable, and reading
// the compendium once is most of what a service costs.
var catalogSource = filetest.SRD()

func newService(t *testing.T) *charuc.Service {
	t.Helper()
	return charuc.NewService(
		memory.NewCharacterRepository(),
		memory.NewFolderRepository(),
		catalogSource,
		nil,
		slog.New(slog.DiscardHandler),
	)
}
