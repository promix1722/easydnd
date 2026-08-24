package hexsheet

import (
	"context"
	"path/filepath"
	"testing"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// loadTestCatalog loads the committed compendium for the internal tests.
//
// The real data is used rather than a stub for the same reason the domain's
// tests use it: the names these tests resolve are the compendium's own, and a
// stub would only prove the resolver agrees with itself.
func loadTestCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "data", "srd_5.1")
	c, err := catalogfile.NewSource(dir).Load(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("loading the compendium: %v", err)
	}
	return c
}
