package character_test

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

const testOwner domain.OwnerID = "test-owner"

// One Source for every service these tests build. Source.Load caches per
// locale, so the ~47 services this package constructs now share a single read
// of the 1.55 MB compendium instead of paying for one each. Sharing is safe
// for the reason the cache is: a Catalog is immutable, and Load is
// mutex-guarded. The repositories stay per-service -- those are the state a
// test is entitled to have to itself.
var catalogSource = catalogfile.NewSource(filepath.Join("..", "..", "..", "data", "srd_5.1"))

func newService(t *testing.T) *charuc.Service {
	t.Helper()
	return charuc.NewService(
		memory.NewCharacterRepository(),
		memory.NewFolderRepository(),
		catalogSource,
		nil,
		nil,
		slog.New(slog.DiscardHandler),
	)
}

func opening() charuc.NewCharacter {
	return charuc.NewCharacter{Name: "Сахарок", Alignment: "neutral"}
}

func mustCreate(t *testing.T, s *charuc.Service) domain.Character {
	t.Helper()
	c, err := s.Create(context.Background(), testOwner, "", opening())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return c
}

// scoresEvent answers character/abilities: the base array as generated, plus
// the method it was generated with. Creation no longer carries any of it, so
// every test that needs a scored character sends this.
func scoresEvent() domain.Event {
	return domain.Event{
		Type: domain.EventChange,
		Changes: []domain.Change{
			{Path: "abilities.method", Op: domain.OpSet, Value: domain.SlugValue("point-buy")},
			{Path: "abilities.str", Op: domain.OpSet, Value: domain.IntValue(10)},
			{Path: "abilities.dex", Op: domain.OpSet, Value: domain.IntValue(15)},
			{Path: "abilities.con", Op: domain.OpSet, Value: domain.IntValue(13)},
			{Path: "abilities.int", Op: domain.OpSet, Value: domain.IntValue(10)},
			{Path: "abilities.wis", Op: domain.OpSet, Value: domain.IntValue(12)},
			{Path: "abilities.cha", Op: domain.OpSet, Value: domain.IntValue(12)},
		},
	}
}

// mustCreateScored is the old opening state, reached the way a client reaches
// it now: create with a name, then answer the abilities prompt.
func mustCreateScored(t *testing.T, s *charuc.Service) domain.Character {
	t.Helper()
	c := mustCreate(t, s)
	if _, err := s.Apply(context.Background(), testOwner, c.ID, rules.DefaultLocale, 1, scoresEvent()); err != nil {
		t.Fatalf("Apply(scores) error = %v", err)
	}
	got, err := s.Get(context.Background(), testOwner, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	return got
}

func hasOpenPrompt(prompts []domain.Prompt, id rules.Slug) bool {
	for _, p := range prompts {
		if p.Choice.Prompt == id {
			return true
		}
	}
	return false
}

// Creation writes one entry holding one selection: the name. The scores are
// not in it, and the proof is that the character is still being asked for
// them.
func TestCreateSeedsANameOnlyInitEvent(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreate(t, s)

	if c.Log.Len() != 1 {
		t.Fatalf("log length = %d, want 1", c.Log.Len())
	}
	init := c.Log.Events[0]
	if init.Type != domain.EventInit {
		t.Errorf("first event = %s, want init", init.Type)
	}
	if init.Source != domain.GroupIdentity {
		t.Errorf("source = %s, want identity", init.Source)
	}
	for _, change := range init.Changes {
		switch change.Path {
		case "identity.name", "identity.alignment":
		default:
			t.Errorf("the init event still bundles %q", change.Path)
		}
	}

	sheet, err := s.Sheet(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}
	if sheet.Identity.Name != "Сахарок" {
		t.Errorf("name = %q, want Сахарок", sheet.Identity.Name)
	}

	prompts, err := s.Prompts(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Prompts() error = %v", err)
	}
	if !hasOpenPrompt(prompts, "character/abilities") {
		t.Error("a freshly created character is not being asked for its ability scores")
	}
}

func TestCreateRequiresAName(t *testing.T) {
	s := newService(t)
	_, err := s.Create(context.Background(), testOwner, "", charuc.NewCharacter{})
	var fieldErr *types.FieldValidationError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("Create() error = %v, want a FieldValidationError", err)
	}
	if len(fieldErr.Fields) != 1 || fieldErr.Fields[0].Field != "name" {
		t.Errorf("fields = %+v, want one naming name", fieldErr.Fields)
	}
}

// The bound on a score moved with the scores. It is checked where they now
// arrive rather than where they used to.
func TestApplyRejectsAnImpossibleScore(t *testing.T) {
	s := newService(t)
	c := mustCreate(t, s)

	bad := scoresEvent()
	bad.Changes[1].Value = domain.IntValue(99)

	_, err := s.Apply(context.Background(), testOwner, c.ID, rules.DefaultLocale, 1, bad)
	var fieldErr *types.FieldValidationError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("Apply() error = %v, want a FieldValidationError", err)
	}
	if len(fieldErr.Fields) != 1 || fieldErr.Fields[0].Rule != "range" {
		t.Errorf("fields = %+v, want one with rule range", fieldErr.Fields)
	}
}

// The scores are an ordinary answer now: their own entry, filed under their
// own group, and the prompt closes behind them.
func TestScoresAreTheirOwnEntry(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	if c.Log.Len() != 2 {
		t.Fatalf("log length = %d, want 2", c.Log.Len())
	}
	if got := c.Log.Events[1].Source; got != domain.GroupAbilities {
		t.Errorf("source = %s, want abilities", got)
	}

	prompts, err := s.Prompts(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Prompts() error = %v", err)
	}
	if hasOpenPrompt(prompts, "character/abilities") {
		t.Error("the abilities prompt is still open after being answered")
	}
	sheet, err := s.Sheet(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}
	if got := sheet.Abilities.Score(rules.Dexterity); got != 15 {
		t.Errorf("Dexterity = %d, want the base 15", got)
	}
	if got := sheet.Abilities.Method; got != "point-buy" {
		t.Errorf("method = %q, want point-buy: it travels with the scores now", got)
	}
}

// Every entry records the group of the prompt it answers, and the server is
// what writes it -- a client cannot file an answer under a category of its
// own choosing, because nothing it sends is read for one.
func TestAppendRecordsTheSource(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	if _, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2,
		domain.Event{
			Type:   domain.EventRace,
			Ref:    rules.NewRef(rules.RefRace, "half-elf"),
			Source: domain.GroupClass, // a lie the server must not repeat
		},
		domain.Event{Type: domain.EventNote, Note: "a thought"},
	); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	after, err := s.Get(ctx, testOwner, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got := after.Log.Events[2].Source; got != domain.GroupRace {
		t.Errorf("race source = %s, want race", got)
	}
	// A note answers nothing, so it belongs to no question.
	if got := after.Log.Events[3].Source; got != domain.PromptGroupNone {
		t.Errorf("note source = %s, want none", got)
	}
}

// The bug answersAnOpenPrompt closes: subrace:hill-dwarf resolves perfectly
// well in the compendium, and nothing was asking a half-elf for a subrace.
func TestApplyRejectsAnEntryNothingOffered(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	if _, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2,
		domain.Event{Type: domain.EventRace, Ref: rules.NewRef(rules.RefRace, "half-elf")}); err != nil {
		t.Fatalf("Apply(race) error = %v", err)
	}

	_, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 3,
		domain.Event{Type: domain.EventSubrace, Ref: rules.NewRef(rules.RefSubrace, "hill-dwarf")})
	var fieldErr *types.FieldValidationError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("Apply() error = %v, want a FieldValidationError", err)
	}
	if len(fieldErr.Fields) != 1 || fieldErr.Fields[0].Rule != "not-offered" {
		t.Errorf("fields = %+v, want one with rule not-offered", fieldErr.Fields)
	}

	sheet, err := s.Sheet(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}
	if !sheet.Identity.Subrace.IsZero() {
		t.Errorf("subrace = %q, want none: a half-elf has no subraces", sheet.Identity.Subrace)
	}
}

// The core of the append-per-step flow: post an answer, get the sheet back
// changed by it.
func TestApplyAdvancesTheCharacter(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	seq, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2, domain.Event{
		Type: domain.EventRace,
		Ref:  rules.NewRef(rules.RefRace, "half-elf"),
		Choices: []domain.Answer{
			{Prompt: "half-elf/ability-bonus/0", Picks: []rules.Slug{"dex", "con"}},
		},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if seq != 3 {
		t.Errorf("sequence = %d, want 3", seq)
	}

	sheet, err := s.Sheet(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}
	if got := sheet.Abilities.Score(rules.Dexterity); got != 16 {
		t.Errorf("Dexterity = %d, want 16 (15 base + 1 chosen)", got)
	}
	if got := sheet.Abilities.Score(rules.Charisma); got != 14 {
		t.Errorf("Charisma = %d, want 14 (12 base + the half-elf's fixed 2)", got)
	}
}

// A batch may answer a prompt that the same batch opened -- choosing a race
// and its ability bonuses in one request is a reasonable thing to send.
func TestApplyValidatesABatchAgainstItself(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	_, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2,
		domain.Event{Type: domain.EventRace, Ref: rules.NewRef(rules.RefRace, "half-elf")},
		domain.Event{Type: domain.EventChange, Choices: []domain.Answer{
			// Skill Versatility is a trait; its prompt did not exist before
			// the event just ahead of this one in the same batch.
			{Prompt: "skill-versatility/proficiency/0", Picks: []rules.Slug{
				"skill-perception", "skill-acrobatics",
			}},
		}},
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRejectsAStaleSequence(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreate(t, s)

	_, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 7, domain.Event{Type: domain.EventNote, Note: "x"})
	var validation *types.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Apply() error = %v, want a ValidationError", err)
	}
}

func TestApplyRejectsBadAnswers(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		event domain.Event
		rule  string
	}{
		{
			name: "a prompt the character does not have open",
			event: domain.Event{Type: domain.EventRace, Ref: rules.NewRef(rules.RefRace, "half-elf"),
				Choices: []domain.Answer{{Prompt: "wizard/proficiency/0", Picks: []rules.Slug{"skill-arcana"}}}},
			rule: "unknown",
		},
		{
			name: "an option the prompt does not offer",
			event: domain.Event{Type: domain.EventRace, Ref: rules.NewRef(rules.RefRace, "half-elf"),
				// Charisma is the half-elf's fixed bonus and is not on offer.
				Choices: []domain.Answer{{Prompt: "half-elf/ability-bonus/0", Picks: []rules.Slug{"dex", "cha"}}}},
			rule: "option",
		},
		{
			name: "more picks than the prompt asks for",
			event: domain.Event{Type: domain.EventRace, Ref: rules.NewRef(rules.RefRace, "half-elf"),
				Choices: []domain.Answer{{Prompt: "half-elf/ability-bonus/0",
					Picks: []rules.Slug{"dex", "con", "str"}}}},
			rule: "choose",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newService(t)
			c := mustCreateScored(t, s)

			_, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2, tt.event)
			var fieldErr *types.FieldValidationError
			if !errors.As(err, &fieldErr) {
				t.Fatalf("Apply() error = %v, want a FieldValidationError", err)
			}
			found := false
			for _, f := range fieldErr.Fields {
				if f.Rule == tt.rule {
					found = true
				}
			}
			if !found {
				t.Errorf("fields = %+v, want one with rule %q", fieldErr.Fields, tt.rule)
			}
		})
	}
}

// A reference the compendium does not have would project as a character who
// simply has no race, with nothing saying why.
func TestApplyRejectsAnUnknownReference(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	_, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2,
		domain.Event{Type: domain.EventBackground, Ref: rules.NewRef(rules.RefBackground, "urchin")})
	var fieldErr *types.FieldValidationError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("Apply() error = %v, want a FieldValidationError", err)
	}
}

// Truncate is the Back button. It must undo, must not drop the init event,
// and must respect the same concurrency check as Append.
func TestTruncateUndoesAStep(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	if _, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2,
		domain.Event{Type: domain.EventRace, Ref: rules.NewRef(rules.RefRace, "half-elf")}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if err := s.Truncate(ctx, testOwner, c.ID, 3, 2); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}
	after, err := s.Get(ctx, testOwner, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if after.Log.Len() != 2 {
		t.Errorf("log length = %d, want 2", after.Log.Len())
	}

	sheet, err := s.Sheet(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}
	if !sheet.Identity.Race.IsZero() {
		t.Errorf("race = %q, want it undone", sheet.Identity.Race)
	}

	// The init event is not a step you can go back past.
	if err := s.Truncate(ctx, testOwner, c.ID, 2, 0); err == nil {
		t.Error("Truncate() dropped the init event")
	}
	// And a stale sequence is rejected exactly as it is for an append.
	if err := s.Truncate(ctx, testOwner, c.ID, 99, 1); err == nil {
		t.Error("Truncate() accepted a stale sequence")
	}
}

func TestListSummarisesWithoutProjecting(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreateScored(t, s)

	if _, err := s.Apply(ctx, testOwner, c.ID, rules.DefaultLocale, 2,
		domain.Event{Type: domain.EventClass, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	got, err := s.List(ctx, testOwner, "", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("summaries = %d, want 1", len(got))
	}
	// The fields the port's own doc promised and the adapter could never
	// supply, because a repository has no catalogue to project against.
	if got[0].Name != "Сахарок" {
		t.Errorf("name = %q, want Сахарок", got[0].Name)
	}
	if got[0].Level != 1 {
		t.Errorf("level = %d, want 1", got[0].Level)
	}
	if len(got[0].Classes) != 1 || got[0].Classes[0].Class != "rogue" {
		t.Errorf("classes = %+v, want one rogue", got[0].Classes)
	}

	// Another owner's list is empty, which is the seam ownership will use.
	other, err := s.List(ctx, "somebody-else", "", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(other) != 0 {
		t.Errorf("another owner sees %d characters, want 0", len(other))
	}
}

func TestDeleteRemovesTheCharacter(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreate(t, s)

	if err := s.Delete(ctx, testOwner, c.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Get(ctx, testOwner, c.ID); !types.IsNotFound(err) {
		t.Errorf("Get() error = %v, want a NotFoundError", err)
	}
}

// A character belongs to somebody, and every path has to say so.
//
// The refusal is a not-found rather than a forbidden, deliberately: answering
// 403 on somebody else's id would confirm that the id exists, which turns a
// guessable identifier into an enumeration oracle.
func TestAnotherOwnerCannotReachTheCharacter(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	c := mustCreate(t, s)

	const intruder domain.OwnerID = "somebody-else"

	if _, err := s.Get(ctx, intruder, c.ID); !types.IsNotFound(err) {
		t.Errorf("Get() error = %v, want a NotFoundError", err)
	}
	if _, err := s.Sheet(ctx, intruder, c.ID, rules.DefaultLocale); !types.IsNotFound(err) {
		t.Errorf("Sheet() error = %v, want a NotFoundError", err)
	}
	if _, err := s.Prompts(ctx, intruder, c.ID, rules.DefaultLocale); !types.IsNotFound(err) {
		t.Errorf("Prompts() error = %v, want a NotFoundError", err)
	}
	_, err := s.Apply(ctx, intruder, c.ID, rules.DefaultLocale, 1,
		domain.Event{Type: domain.EventNote, Note: "mine now"})
	if !types.IsNotFound(err) {
		t.Errorf("Apply() error = %v, want a NotFoundError", err)
	}
	if err := s.Truncate(ctx, intruder, c.ID, 1, 1); !types.IsNotFound(err) {
		t.Errorf("Truncate() error = %v, want a NotFoundError", err)
	}
	if err := s.Delete(ctx, intruder, c.ID); !types.IsNotFound(err) {
		t.Errorf("Delete() error = %v, want a NotFoundError", err)
	}

	// And none of it did anything: the character is still there, unchanged.
	after, err := s.Get(ctx, testOwner, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if after.Log.Len() != 1 {
		t.Errorf("log length = %d, want 1", after.Log.Len())
	}
}
