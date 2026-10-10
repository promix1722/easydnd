package repotest

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// NewCharacterRepository builds an empty character store for one subtest.
type NewCharacterRepository func(t *testing.T) domain.Repository

const (
	charOwner  domain.OwnerID  = "usr_1"
	charFolder domain.FolderID = "fld_000001"
)

// RunCharacterRepository runs the whole port contract against one
// implementation. It never asserts the shape of an id -- the in-memory store
// counts and Postgres draws from a sequence -- only that ids are distinct and
// that a listing comes back in creation order.
func RunCharacterRepository(t *testing.T, newRepo NewCharacterRepository) {
	t.Helper()

	tests := []struct {
		name string
		run  func(t *testing.T, repo domain.Repository)
	}{
		{
			name: "create assigns distinct ids and an empty log",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				first, err := repo.Create(ctx, charOwner, charFolder)
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				second, err := repo.Create(ctx, charOwner, charFolder)
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				if first.ID == second.ID {
					t.Errorf("Create() issued duplicate ID %q", first.ID)
				}
				if first.Owner != charOwner || first.Folder != charFolder {
					t.Errorf("Create() = %+v, want owner %q in %q", first, charOwner, charFolder)
				}
				if first.Log.Len() != 0 || first.Revision != 0 {
					t.Errorf("Create() log length = %d, revision = %d, want 0 and 0", first.Log.Len(), first.Revision)
				}
			},
		},
		{
			name: "get reports not found",
			run: func(t *testing.T, repo domain.Repository) {
				if _, err := repo.Get(context.Background(), "chr_missing"); !types.IsNotFound(err) {
					t.Errorf("Get() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			name: "list filters by owner and keeps creation order",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				var ids []domain.ID
				for range 3 {
					c, err := repo.Create(ctx, charOwner, charFolder)
					if err != nil {
						t.Fatalf("Create() error = %v", err)
					}
					ids = append(ids, c.ID)
				}
				if _, err := repo.Create(ctx, "usr_2", charFolder); err != nil {
					t.Fatalf("Create() error = %v", err)
				}

				got, err := repo.List(ctx, charOwner)
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if len(got) != 3 {
					t.Fatalf("List() length = %d, want 3", len(got))
				}
				for i, c := range got {
					if c.ID != ids[i] || c.Owner != charOwner {
						t.Errorf("List()[%d] = %q owned by %q, want %q owned by %q", i, c.ID, c.Owner, ids[i], charOwner)
					}
				}
				none, err := repo.List(ctx, "usr_nobody")
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if none == nil || len(none) != 0 {
					t.Errorf("List() for nobody = %v, want an empty, non-nil slice", none)
				}
			},
		},
		{
			name: "get returns a copy",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				c, err := repo.Create(ctx, charOwner, charFolder)
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				if err := Append(ctx, repo, c.ID, domain.Event{Type: domain.EventInit, Note: "original"}); err != nil {
					t.Fatalf("Append() error = %v", err)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				got.Log.Events[0].Note = "mutated"
				again, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if again.Log.Events[0].Note != "original" {
					t.Errorf("note = %q, want %q: Get must not share its backing array", again.Log.Events[0].Note, "original")
				}
			},
		},
		{
			name: "delete removes and reports not found",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				c, err := repo.Create(ctx, charOwner, charFolder)
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				if err := repo.Delete(ctx, c.ID); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				if _, err := repo.Get(ctx, c.ID); !types.IsNotFound(err) {
					t.Errorf("Get() after Delete error = %v, want a NotFoundError", err)
				}
				if err := repo.Delete(ctx, c.ID); !types.IsNotFound(err) {
					t.Errorf("second Delete() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			// Exercised by `go test -race`, which is how the Makefile runs
			// the suite.
			name: "concurrent writers do not lose characters",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				var wg sync.WaitGroup
				for range 8 {
					wg.Add(1)
					go func() {
						defer wg.Done()
						c, err := repo.Create(ctx, charOwner, charFolder)
						if err != nil {
							return
						}
						_ = Append(ctx, repo, c.ID, domain.Event{Type: domain.EventInit})
						_, _ = repo.Get(ctx, c.ID)
						_, _ = repo.List(ctx, charOwner)
					}()
				}
				wg.Wait()
				got, err := repo.List(ctx, charOwner)
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if len(got) != 8 {
					t.Errorf("List() length = %d, want 8", len(got))
				}
			},
		},
		{
			name: "set folder moves a character and nothing else",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				c := seeded(t, repo, domain.EventInit)
				if err := repo.SetFolder(ctx, c.ID, "fld_000002"); err != nil {
					t.Fatalf("SetFolder() error = %v", err)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if got.Folder != "fld_000002" {
					t.Errorf("Get() folder = %q, want fld_000002", got.Folder)
				}
				if got.Log.Len() != 1 || got.Revision != 1 {
					t.Errorf("SetFolder() changed the log to %d events at revision %d, want 1 and 1", got.Log.Len(), got.Revision)
				}
				if err := repo.SetFolder(ctx, "chr_missing", charFolder); !types.IsNotFound(err) {
					t.Errorf("SetFolder() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			name: "set public opens a character and nothing else",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				c := seeded(t, repo, domain.EventInit)
				if c.Public {
					t.Fatal("a new character is public")
				}
				if err := repo.SetPublic(ctx, c.ID, true); err != nil {
					t.Fatalf("SetPublic() error = %v", err)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if !got.Public {
					t.Error("Get() public = false after SetPublic(true)")
				}
				if got.Log.Len() != 1 || got.Revision != 1 {
					t.Errorf("SetPublic() changed the log to %d events at revision %d, want 1 and 1", got.Log.Len(), got.Revision)
				}
				if err := repo.SetPublic(ctx, "chr_missing", true); !types.IsNotFound(err) {
					t.Errorf("SetPublic() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			// Commit is the write every application mutation goes through.
			// The revision formula is repeated by usecases that compute the
			// revision they will answer with, so both adapters must land on
			// the same number.
			name: "commit advances the revision by the events added, at least one",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				c, err := repo.Create(ctx, charOwner, charFolder)
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				two := logOf(t, domain.EventInit, domain.EventRace)
				if err := repo.Commit(ctx, c.ID, 0, two, nil); err != nil {
					t.Fatalf("Commit() error = %v", err)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if got.Revision != 2 {
					t.Fatalf("revision = %d, want 2 after committing two events", got.Revision)
				}
				for i, e := range got.Log.Events {
					if e.ID == "" || e.SchemaVersion != 1 {
						t.Errorf("event %d = id %q schema %d, want a stamped id at schema 1", i, e.ID, e.SchemaVersion)
					}
				}
				// Same length: still a change, still one revision.
				same := logOf(t, domain.EventInit, domain.EventBackground)
				if err := repo.Commit(ctx, c.ID, 2, same, nil); err != nil {
					t.Fatalf("Commit() error = %v", err)
				}
				// Shorter: one revision, not a negative one.
				one := logOf(t, domain.EventInit)
				if err := repo.Commit(ctx, c.ID, 3, one, nil); err != nil {
					t.Fatalf("Commit() error = %v", err)
				}
				got, err = repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if got.Revision != 4 || got.Log.Len() != 1 {
					t.Errorf("revision = %d with %d events, want 4 and 1", got.Revision, got.Log.Len())
				}
			},
		},
		{
			name: "commit rejects a stale revision, a repeated command and a bad log",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				c, err := repo.Create(ctx, charOwner, charFolder)
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				log := logOf(t, domain.EventInit)
				if err := repo.Commit(ctx, c.ID, 0, log, nil); err != nil {
					t.Fatalf("Commit() error = %v", err)
				}
				var invalid *types.ValidationError
				if err := repo.Commit(ctx, c.ID, 0, log, nil); !errors.As(err, &invalid) {
					t.Errorf("Commit() at a stale revision error = %v, want a ValidationError", err)
				}
				bad := domain.Log{Events: []domain.Event{{Seq: 4, Type: domain.EventInit}}}
				if err := repo.Commit(ctx, c.ID, 1, bad, nil); !errors.As(err, &invalid) {
					t.Errorf("Commit() of a malformed log error = %v, want a ValidationError", err)
				}
				if err := repo.Commit(ctx, "chr_missing", 0, log, nil); !types.IsNotFound(err) {
					t.Errorf("Commit() error = %v, want a NotFoundError", err)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if got.Revision != 1 {
					t.Errorf("revision = %d, want 1 after three refusals", got.Revision)
				}
			},
		},
		{
			name: "commit keeps a checkpoint beside the log",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				c := seeded(t, repo, domain.EventInit, domain.EventRace)
				before, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				cp := domain.Checkpoint{Revision: before.Revision, Log: before.Log, Reason: "rules-migration"}
				if err := repo.Commit(ctx, c.ID, before.Revision, logOf(t, domain.EventInit), &cp); err != nil {
					t.Fatalf("Commit() error = %v", err)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if len(got.Checkpoints) != 1 {
					t.Fatalf("checkpoints = %d, want 1", len(got.Checkpoints))
				}
				kept := got.Checkpoints[0]
				if kept.Reason != "rules-migration" || kept.Revision != before.Revision || kept.Log.Len() != 2 {
					t.Errorf("checkpoint = %+v, want the two-event log at revision %d", kept, before.Revision)
				}
				if got.Log.Len() != 1 {
					t.Errorf("log length = %d, want the committed single event", got.Log.Len())
				}
			},
		},
		{
			name: "create with log stores the first log in one write",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				log := logOf(t, domain.EventInit, domain.EventRace, domain.EventClass)
				c, err := repo.CreateWithLog(ctx, charOwner, charFolder, log)
				if err != nil {
					t.Fatalf("CreateWithLog() error = %v", err)
				}
				if c.ID == "" || c.Owner != charOwner || c.Folder != charFolder {
					t.Errorf("CreateWithLog() = %+v, want an id owned by %q in %q", c, charOwner, charFolder)
				}
				if c.Revision != 3 {
					t.Errorf("revision = %d, want 3: the revision Create then Commit would reach", c.Revision)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if got.Log.Len() != 3 || got.Revision != 3 {
					t.Errorf("stored %d events at revision %d, want 3 and 3", got.Log.Len(), got.Revision)
				}
				for i, e := range got.Log.Events {
					if e.ID == "" || e.SchemaVersion != 1 {
						t.Errorf("event %d = id %q schema %d, want a stamped id at schema 1", i, e.ID, e.SchemaVersion)
					}
				}
				bad := domain.Log{Events: []domain.Event{{Seq: 2, Type: domain.EventInit}}}
				var invalid *types.ValidationError
				if _, err := repo.CreateWithLog(ctx, charOwner, charFolder, bad); !errors.As(err, &invalid) {
					t.Errorf("CreateWithLog() of a malformed log error = %v, want a ValidationError", err)
				}
			},
		},
		{
			// Every field an event can carry, through the store and back.
			// The in-memory store copies and Postgres serialises, and a
			// field either one drops is a choice the character silently
			// loses.
			name: "a rich log round-trips intact",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				level, hitDie := 3, 8
				lock := pack.Lock{Edition: "2014", Semantics: pack.Semantics, Packs: []pack.Release{{ID: "srd-5.1", Version: "1.0.0", Digest: strings.Repeat("ab", 32)}}}
				events := []domain.Event{
					{Type: domain.EventInit, At: time.Unix(1_700_000_000, 123456000).UTC(), RulesLock: lock, Changes: []domain.Change{
						{Path: "identity.name", Op: domain.OpSet, Value: domain.Value{Kind: domain.ValueString, Str: "Tam"}},
						{Path: "abilities.str", Op: domain.OpIncrement, Value: domain.Value{Kind: domain.ValueInt, Int: 2}},
						{Path: "skills", Op: domain.OpAdd, Value: domain.Value{Kind: domain.ValueSlugList, Slugs: []rules.Slug{"stealth", "arcana"}}},
						{Path: "hp.dice", Op: domain.OpSet, Value: domain.Value{Kind: domain.ValueDice, Dice: rules.Dice{Terms: []rules.DiceTerm{{Count: 2, Faces: 6}}, Bonus: 1}}},
					}},
					{Type: domain.EventClass, Ref: rules.Ref{Kind: rules.RefClass, Slug: "rogue"}, Level: 1, Source: domain.GroupClass,
						Choices:  []domain.Answer{{Prompt: "rogue-skills", Picks: []rules.Slug{"stealth"}}},
						Custom:   &domain.CustomOption{Name: "Knife Saint", Level: &level, HitDie: &hitDie},
						Observed: true, Evidence: "page 3"},
					{Type: domain.EventResourceSpent, Resource: "hit-dice", Amount: 1, Trigger: "short-rest", Allocations: map[rules.Slug]int{"spell-slot-1": 1}},
					{Type: domain.EventNote, Note: "Has a \"tab\"\there and a é"},
				}
				c, err := repo.CreateWithLog(ctx, charOwner, charFolder, mustRebuild(t, events...))
				if err != nil {
					t.Fatalf("CreateWithLog() error = %v", err)
				}
				got, err := repo.Get(ctx, c.ID)
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if got.Log.Len() != len(events) {
					t.Fatalf("log length = %d, want %d", got.Log.Len(), len(events))
				}
				init, class, spent, note := got.Log.Events[0], got.Log.Events[1], got.Log.Events[2], got.Log.Events[3]
				if !init.At.Equal(events[0].At) {
					t.Errorf("init.At = %v, want %v", init.At, events[0].At)
				}
				if init.RulesLock.Edition != "2014" || len(init.RulesLock.Packs) != 1 || init.RulesLock.Packs[0].Digest != lock.Packs[0].Digest {
					t.Errorf("init.RulesLock = %+v, want %+v", init.RulesLock, lock)
				}
				if len(init.Changes) != 4 {
					t.Fatalf("init.Changes = %d, want 4", len(init.Changes))
				}
				if v := init.Changes[0].Value; v.Kind != domain.ValueString || v.Str != "Tam" {
					t.Errorf("name change = %+v", v)
				}
				if v := init.Changes[1]; v.Op != domain.OpIncrement || v.Value.Int != 2 {
					t.Errorf("ability change = %+v", v)
				}
				if v := init.Changes[2].Value; len(v.Slugs) != 2 || v.Slugs[1] != "arcana" {
					t.Errorf("skills change = %+v", v)
				}
				if v := init.Changes[3].Value.Dice; len(v.Terms) != 1 || v.Terms[0].Faces != 6 || v.Bonus != 1 {
					t.Errorf("dice change = %+v", v)
				}
				if class.Ref != (rules.Ref{Kind: rules.RefClass, Slug: "rogue"}) || class.Level != 1 || class.Source != domain.GroupClass || !class.Observed || class.Evidence != "page 3" {
					t.Errorf("class event = %+v", class)
				}
				if len(class.Choices) != 1 || class.Choices[0].Prompt != "rogue-skills" || class.Choices[0].Picks[0] != "stealth" {
					t.Errorf("class choices = %+v", class.Choices)
				}
				if class.Custom == nil || class.Custom.Name != "Knife Saint" || class.Custom.Level == nil || *class.Custom.Level != 3 || *class.Custom.HitDie != 8 {
					t.Errorf("class custom = %+v", class.Custom)
				}
				if spent.Resource != "hit-dice" || spent.Amount != 1 || spent.Trigger != "short-rest" || spent.Allocations["spell-slot-1"] != 1 {
					t.Errorf("resource event = %+v", spent)
				}
				if note.Note != events[3].Note {
					t.Errorf("note = %q, want %q", note.Note, events[3].Note)
				}
			},
		},
		{
			name: "search filters across owners, pages newest first and counts every match",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				var ids []domain.ID
				for _, owner := range []domain.OwnerID{charOwner, "usr_2", charOwner} {
					c, err := repo.Create(ctx, owner, charFolder)
					if err != nil {
						t.Fatalf("Create() error = %v", err)
					}
					ids = append(ids, c.ID)
				}
				if err := repo.SetPublic(ctx, ids[1], true); err != nil {
					t.Fatalf("SetPublic() error = %v", err)
				}
				public := true

				tests := []struct {
					name  string
					query domain.Query
					want  []domain.ID
					total int
				}{
					{"everything", domain.Query{Limit: 10}, []domain.ID{ids[2], ids[1], ids[0]}, 3},
					{"second page", domain.Query{Limit: 2, Offset: 2}, []domain.ID{ids[0]}, 3},
					{"past the end", domain.Query{Limit: 2, Offset: 5}, nil, 3},
					{"one owner", domain.Query{Owners: []domain.OwnerID{charOwner}, Limit: 10}, []domain.ID{ids[2], ids[0]}, 2},
					{"public only", domain.Query{Public: &public, Limit: 10}, []domain.ID{ids[1]}, 1},
					{"by id", domain.Query{ID: ids[0].String(), Limit: 10}, []domain.ID{ids[0]}, 1},
					{"nobody's", domain.Query{Owners: []domain.OwnerID{"usr_9"}, Limit: 10}, nil, 0},
				}
				for _, tc := range tests {
					found, total, err := repo.Search(ctx, tc.query)
					if err != nil {
						t.Fatalf("%s: Search() error = %v", tc.name, err)
					}
					var got []domain.ID
					for _, c := range found {
						got = append(got, c.ID)
					}
					if !slices.Equal(got, tc.want) || total != tc.total {
						t.Errorf("%s: Search() = %v of %d, want %v of %d", tc.name, got, total, tc.want, tc.total)
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newRepo(t))
		})
	}
}

// seeded creates a character and appends one event of each type, or fails
// the test.
// Append adds events to a stored character's log through Commit, the one
// write the port has for a log. It is how a test seeds a character.
func Append(ctx context.Context, repo domain.Repository, id domain.ID, events ...domain.Event) error {
	c, err := repo.Get(ctx, id)
	if err != nil {
		return err
	}
	log := c.Log.Clone()
	if err := log.Append(events...); err != nil {
		return err
	}
	return repo.Commit(ctx, id, c.Revision, log, nil)
}

func seeded(t *testing.T, repo domain.Repository, types ...domain.EventType) domain.Character {
	t.Helper()
	ctx := context.Background()
	c, err := repo.Create(ctx, charOwner, charFolder)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	events := make([]domain.Event, 0, len(types))
	for _, typ := range types {
		events = append(events, domain.Event{Type: typ})
	}
	if err := Append(ctx, repo, c.ID, events...); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	return c
}

// logOf builds a log with one event of each type, or fails the test.
func logOf(t *testing.T, types ...domain.EventType) domain.Log {
	t.Helper()
	events := make([]domain.Event, 0, len(types))
	for _, typ := range types {
		events = append(events, domain.Event{Type: typ})
	}
	return mustRebuild(t, events...)
}

// mustRebuild numbers events into a log, or fails the test.
func mustRebuild(t *testing.T, events ...domain.Event) domain.Log {
	t.Helper()
	log, err := domain.Rebuild(events)
	if err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	return log
}
