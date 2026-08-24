package character_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/sheet/hexsheet"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// The importing service is wired over the real adapter rather than a stub.
// A stub would prove the service calls something; these tests are about
// whether an imported character is actually usable afterwards, which only the
// real importer can answer.
func newImportingService(t *testing.T) *charuc.Service {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "data", "srd_5.1")
	return charuc.NewService(
		memory.NewCharacterRepository(),
		catalogfile.NewSource(dir),
		hexsheet.NewImporter(),
		slog.New(slog.DiscardHandler),
	)
}

func referenceExport(t *testing.T) *os.File {
	t.Helper()
	path := filepath.Join("..", "..", "..",
		"docs", "reference_hexsheet", "rouge_3_level.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening the reference export: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestImportCreatesAnOwnedCharacter(t *testing.T) {
	s := newImportingService(t)
	ctx := context.Background()

	imported, report, err := s.Import(ctx, "owner-1", rules.DefaultLocale, referenceExport(t))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if imported.ID.IsZero() {
		t.Error("the imported character has no id")
	}
	if imported.Owner != "owner-1" {
		t.Errorf("owner = %q, want owner-1", imported.Owner)
	}
	if imported.Log.Len() == 0 {
		t.Fatal("the imported log is empty")
	}
	if len(report.Unresolved) == 0 {
		t.Error("the report should name Urchin at least")
	}

	// The character has to be readable through the ordinary path, not just
	// exist in the store: an import that produces a log Sheet cannot project
	// is worse than one that fails.
	sheet, err := s.Sheet(ctx, "owner-1", imported.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}
	if sheet.Identity.Name != "Сахарок" {
		t.Errorf("name = %q, want Сахарок", sheet.Identity.Name)
	}
	if sheet.Status.ArmorClass != 14 {
		t.Errorf("armor class = %d, want 14", sheet.Status.ArmorClass)
	}
}

// The whole point of answering no prompts is that the build screen still has
// work to do. This is what proves an imported character can be finished.
func TestImportLeavesPromptsOpen(t *testing.T) {
	s := newImportingService(t)
	ctx := context.Background()

	imported, _, err := s.Import(ctx, "owner-1", rules.DefaultLocale, referenceExport(t))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	prompts, err := s.Prompts(ctx, "owner-1", imported.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Prompts() error = %v", err)
	}
	if len(prompts) == 0 {
		t.Fatal("an import answers nothing, so prompts must remain")
	}

	var found bool
	for _, p := range prompts {
		if p.Choice.Prompt == "half-elf/ability-bonus/0" {
			found = true
		}
	}
	if !found {
		t.Error("the half-elf's ability bonuses should still be unanswered")
	}
}

// An imported character must accept events like any other, or the import has
// produced a dead end.
func TestImportedCharacterAcceptsAnswers(t *testing.T) {
	s := newImportingService(t)
	ctx := context.Background()

	imported, _, err := s.Import(ctx, "owner-1", rules.DefaultLocale, referenceExport(t))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	seq, err := s.Apply(ctx, "owner-1", imported.ID, rules.DefaultLocale, imported.Log.LastSeq(),
		domain.Event{
			Type: domain.EventChange,
			Choices: []domain.Answer{{
				Prompt: "half-elf/ability-bonus/0",
				Picks:  []rules.Slug{"dex", "con"},
			}},
		})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if seq <= imported.Log.LastSeq() {
		t.Errorf("sequence = %d, want it past %d", seq, imported.Log.LastSeq())
	}
}

func TestImportRejectsRubbish(t *testing.T) {
	s := newImportingService(t)
	tests := []struct {
		name, body string
	}{
		{"malformed", "{not json"},
		{"another tool", `{"exportedFrom":"Roll20","character":{"name":"x"}}`},
		{"no name", `{"exportedFrom":"HexSheet","character":{"gameSystem":"dnd5e"}}`},
		{"2024 rules", `{"exportedFrom":"HexSheet","character":{"name":"x","ruleset":"2024"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := s.Import(context.Background(), "owner-1",
				rules.DefaultLocale, strings.NewReader(tt.body))
			if err == nil {
				t.Fatal("Import() should have failed")
			}
			var fields *types.FieldValidationError
			if !errors.As(err, &fields) {
				t.Errorf("error = %T, want a *types.FieldValidationError", err)
			}
		})
	}
}

// A service built without an importer answers the one route that needs it,
// rather than panicking somewhere deeper.
func TestImportWithoutAnImporter(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "data", "srd_5.1")
	s := charuc.NewService(
		memory.NewCharacterRepository(), catalogfile.NewSource(dir), nil,
		slog.New(slog.DiscardHandler),
	)
	_, _, err := s.Import(context.Background(), "owner-1",
		rules.DefaultLocale, strings.NewReader("{}"))

	var notImplemented *types.NotImplementedError
	if !errors.As(err, &notImplemented) {
		t.Errorf("error = %v, want a *types.NotImplementedError", err)
	}
}
