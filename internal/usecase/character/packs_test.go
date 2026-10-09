package character_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func TestRulesMigrationPreviewCommitAndRollback(t *testing.T) {
	root := filepath.Join("..", "..", "..", "data")
	r, err := file.NewRegistry([]string{filepath.Join(root, "pack", "srd-5.1"), filepath.Join("..", "..", "adapter", "catalog", "file", "testdata", "tactician.json")}, []file.Dependency{{ID: "srd-2014", Version: "^2.0.0"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	repo := memory.NewCharacterRepository()
	s := charuc.NewService(repo, memory.NewFolderRepository(), r, nil, slog.New(slog.DiscardHandler))
	c := mustCreate(t, s)
	if c.Log.RulesLock().IsZero() {
		t.Fatal("created unpinned character")
	}
	target, err := r.Resolve([]file.Dependency{{ID: "example", Version: "2.0.0"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	preview, err := s.Migrate(ctx, testOwner, c.ID, rules.LocaleEN, c.Revision, target, charuc.Mappings{}, false)
	if err != nil || len(preview.Issues) > 0 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	unchanged, _ := s.Get(ctx, testOwner, c.ID)
	if unchanged.Revision != c.Revision || !unchanged.Log.RulesLock().Equal(c.Log.RulesLock()) {
		t.Fatal("preview mutated character")
	}
	bad := charuc.Mappings{Paths: map[domain.Path]domain.Path{c.Log.Events[0].Changes[0].Path: "invalid.path"}}
	rejected, err := s.Migrate(ctx, testOwner, c.ID, rules.LocaleEN, c.Revision, target, bad, false)
	if err != nil || len(rejected.Issues) == 0 {
		t.Fatalf("bad migration preview: %+v %v", rejected, err)
	}
	if _, err := s.Migrate(ctx, testOwner, c.ID, rules.LocaleEN, c.Revision, target, bad, true); err == nil {
		t.Fatal("invalid migration committed")
	}
	stillOld, _ := s.Get(ctx, testOwner, c.ID)
	if stillOld.Revision != c.Revision || len(stillOld.Checkpoints) != 0 {
		t.Fatal("failed migration changed stored build")
	}
	applied, err := s.Migrate(ctx, testOwner, c.ID, rules.LocaleEN, c.Revision, target, charuc.Mappings{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Revision <= c.Revision {
		t.Fatal("same-length migration did not advance revision")
	}
	migrated, _ := s.Get(ctx, testOwner, c.ID)
	if len(migrated.Checkpoints) != 1 || !migrated.Log.RulesLock().Equal(target) {
		t.Fatal("missing checkpoint or new lock")
	}
	if _, err = s.Apply(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.LocaleEN, c.Log.LastSeq(), domain.Event{Type: domain.EventNote, Note: "stale"}); err == nil {
		t.Fatal("stale same-length write accepted")
	}
	rollback, err := s.RestoreCheckpoint(ctx, testOwner, c.ID, rules.LocaleEN, migrated.Revision, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !rollback.Lock.Equal(c.Log.RulesLock()) {
		t.Fatal("rollback did not restore old lock")
	}
}

func TestRevisionTokenRejectsSameLengthRewrite(t *testing.T) {
	s := newService(t)
	c := mustCreate(t, s)
	ctx := context.Background()
	event := c.Log.Events[0]
	event.Changes[0].Value = domain.StringValue("First edit")
	result, err := s.Revise(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.LocaleEN, c.Log.LastSeq(), 1, &event, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Seq != c.Log.LastSeq() || result.Revision <= c.Revision {
		t.Fatal("invalid revision/sequence semantics")
	}
	event.Changes[0].Value = domain.StringValue("Stale edit")
	if _, err = s.Revise(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.LocaleEN, c.Log.LastSeq(), 1, &event, true); err == nil {
		t.Fatal("stale rewrite accepted")
	}
}
