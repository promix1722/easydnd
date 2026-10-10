package game_test

import (
	"context"
	"errors"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

func wantLimit(t *testing.T, err error, what string) {
	t.Helper()
	var refused *types.ValidationError
	if !errors.As(err, &refused) || refused.Reason != "limit."+what {
		t.Fatalf("error = %v, want limit.%s", err, what)
	}
}

func TestGameLimits(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	l := types.DefaultLimits
	l.GroupGames, l.GroupCharacters, l.GameEntries = 1, 1, 2
	f.svc.SetLimits(l)
	f.table(t, "grp_1", "alice", nil)

	g, err := f.svc.Create(ctx, "alice", "grp_1", "Friday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err = f.svc.Create(ctx, "alice", "grp_1", "Saturday")
	wantLimit(t, err, "groupGames")

	first, second := f.character(t, "alice"), f.character(t, "alice")
	if err := f.svc.Share(ctx, "alice", "grp_1", first); err != nil {
		t.Fatalf("Share() error = %v", err)
	}
	wantLimit(t, f.svc.Share(ctx, "alice", "grp_1", second), "groupCharacters")
	// Seating an unshared character shares it, and is held to the same table.
	wantLimit(t, f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{second}), "groupCharacters")

	for range 2 {
		if err := f.svc.AddMonster(ctx, "alice", g.ID, "", rules.DefaultLocale); err != nil {
			t.Fatalf("AddMonster() error = %v", err)
		}
	}
	wantLimit(t, f.svc.AddMonster(ctx, "alice", g.ID, "", rules.DefaultLocale), "gameEntries")
	wantLimit(t, f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{first}), "gameEntries")
}
