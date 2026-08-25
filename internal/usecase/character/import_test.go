package character_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	return charuc.NewService(
		memory.NewCharacterRepository(),
		memory.NewFolderRepository(),
		catalogSource,
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

	imported, report, err := s.Import(ctx, "owner-1", "", rules.DefaultLocale, referenceExport(t))
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

	imported, _, err := s.Import(ctx, "owner-1", "", rules.DefaultLocale, referenceExport(t))
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

	imported, _, err := s.Import(ctx, "owner-1", "", rules.DefaultLocale, referenceExport(t))
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
			_, _, err := s.Import(context.Background(), "owner-1", "",
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
	s := charuc.NewService(
		memory.NewCharacterRepository(),
		memory.NewFolderRepository(), catalogSource, nil,
		slog.New(slog.DiscardHandler),
	)
	_, _, err := s.Import(context.Background(), "owner-1", "",
		rules.DefaultLocale, strings.NewReader("{}"))

	var notImplemented *types.NotImplementedError
	if !errors.As(err, &notImplemented) {
		t.Errorf("error = %v, want a *types.NotImplementedError", err)
	}
}

// One entry per selection binds an import vacuously, and this is what says so
// out loud: an import makes no selections at all. Not one typed event carries
// an answer, so there is nothing bundled and nothing for a player to be
// unable to change -- every prompt is still theirs to answer.
func TestImportedLogSelectsNothing(t *testing.T) {
	s := newImportingService(t)

	imported, _, err := s.Import(context.Background(), "owner-1", "", rules.DefaultLocale, referenceExport(t))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if err := imported.Log.Validate(); err != nil {
		t.Fatalf("the imported log does not validate: %v", err)
	}
	for _, e := range imported.Log.Events {
		if len(e.Choices) > 0 {
			t.Errorf("entry %d (%s) carries %d answers; an import answers nothing",
				e.Seq, e.Type, len(e.Choices))
		}
		// Nor is any of it attributable: no prompt was answered, so no entry
		// belongs to a group.
		if e.Source != domain.PromptGroupNone {
			t.Errorf("entry %d (%s) claims source %s", e.Seq, e.Type, e.Source)
		}
	}
	// The opening state is the one entry that carries anything, and what it
	// carries is an assertion rather than a set of choices.
	if len(imported.Log.Events[0].Changes) == 0 {
		t.Error("the init event carries nothing, so the export's numbers went nowhere")
	}
}

// And an imported character is revisable like any other: the replay walks a
// log full of entries that answer nothing, and must keep every one of them.
func TestImportedLogSurvivesARevision(t *testing.T) {
	ctx := context.Background()
	s := newImportingService(t)

	imported, _, err := s.Import(ctx, "owner-1", "", rules.DefaultLocale, referenceExport(t))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	before := imported.Log.Len()

	revision, err := s.Revise(ctx, "owner-1", imported.ID, rules.DefaultLocale,
		imported.Log.LastSeq(), 1, &domain.Event{
			Type: domain.EventInit,
			Changes: []domain.Change{
				{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("Рурик")},
			},
		}, true)
	if err != nil {
		t.Fatalf("Revise() error = %v", err)
	}
	if len(revision.Dropped) != 0 {
		t.Errorf("dropped = %+v, want nothing", revision.Dropped)
	}
	if revision.Seq != before {
		t.Errorf("seq = %d, want %d", revision.Seq, before)
	}
	if revision.Sheet.Identity.Name != "Рурик" {
		t.Errorf("name = %q, want Рурик", revision.Sheet.Identity.Name)
	}
	if revision.Sheet.Identity.Race != "half-elf" {
		t.Errorf("race = %q, want the import's own half-elf still there", revision.Sheet.Identity.Race)
	}
}
