package character_test

import (
	"context"
	"errors"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// wantLimit fails unless err is the refusal of the named limit.
func wantLimit(t *testing.T, err error, what string) {
	t.Helper()
	var refused *types.ValidationError
	if !errors.As(err, &refused) || refused.Reason != "limit."+what {
		t.Fatalf("error = %v, want limit.%s", err, what)
	}
}

func limited(t *testing.T, change func(*types.Limits)) *charuc.Service {
	t.Helper()
	s := newService(t)
	l := types.DefaultLimits
	change(&l)
	s.SetLimits(l)
	return s
}

func TestCharacterAndFolderLimits(t *testing.T) {
	ctx := context.Background()
	s := limited(t, func(l *types.Limits) { l.Characters, l.Folders = 1, 2 })

	c := mustCreate(t, s)
	_, err := s.Create(ctx, testOwner, "", opening())
	wantLimit(t, err, "characters")
	_, err = s.CopyCharacter(ctx, testOwner, c.ID, "", rules.DefaultLocale)
	wantLimit(t, err, "characters")
	if _, err := s.Create(ctx, "somebody-else", "", opening()); err != nil {
		t.Fatalf("another owner's first character: %v", err)
	}

	// The default folder that first character made is one of the two.
	if _, err := s.CreateFolder(ctx, testOwner, "Second"); err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	_, err = s.CreateFolder(ctx, testOwner, "Third")
	wantLimit(t, err, "folders")
}

func TestSheetLimitsRefuseOnlyGrowth(t *testing.T) {
	ctx := context.Background()
	s := limited(t, func(l *types.Limits) { l.CharacterNotes, l.CharacterItems = 1, 1 })
	c := mustCreate(t, s)

	note := func(id string) (charuc.Revision, error) {
		c, err := s.Get(ctx, testOwner, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		return s.UpsertCustomOption(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.DefaultLocale,
			domain.CustomOption{ID: id, Kind: "note", Name: "Title", Description: "Text"})
	}
	if _, err := note("one"); err != nil {
		t.Fatalf("first note: %v", err)
	}
	_, err := note("two")
	wantLimit(t, err, "characterNotes")
	if _, err := note("one"); err != nil {
		t.Fatalf("editing the note at the limit: %v", err)
	}

	item := func(slug string, count int) error {
		c, err := s.Get(ctx, testOwner, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Apply(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.DefaultLocale, c.Log.LastSeq(), domain.Event{
			Type:    domain.EventChange,
			Changes: []domain.Change{{Path: domain.Path("equipment.backpack." + slug), Op: domain.OpSet, Value: domain.IntValue(count)}},
		})
		return err
	}
	if err := item("dagger", 1); err != nil {
		t.Fatalf("first item: %v", err)
	}
	wantLimit(t, item("torch", 1), "characterItems")
	if err := item("dagger", 3); err != nil {
		t.Fatalf("changing a stack at the limit: %v", err)
	}
}
