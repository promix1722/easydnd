package pack_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	uc "github.com/promix1722/easydnd/internal/usecase/pack"
)

func fixture(t *testing.T) (context.Context, *uc.Service, *file.Authoring, *memory.PackRepository, *memory.GroupRepository) {
	t.Helper()
	ctx := context.Background()
	base, err := file.NewRegistry([]string{"../../../data/pack/srd-5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	repo := memory.NewPackRepository()
	users := memory.NewUserRepository()
	groups := memory.NewGroupRepository(users)
	engine := file.NewAuthoring(base, repo)
	return ctx, uc.NewService(repo, engine, groups, users), engine, repo, groups
}
func create(t *testing.T, ctx context.Context, s *uc.Service, who string) pack.Record {
	t.Helper()
	r, err := s.Create(ctx, user.User{ID: user.ID(who), Anonymous: true, DisplayName: who}, "Test pack", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func publish(t *testing.T, ctx context.Context, s *uc.Service, r pack.Record) pack.Record {
	t.Helper()
	r, err := s.Publish(ctx, r.Owner, r.ID, r.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestDraftReleasesAndImportIsolation(t *testing.T) {
	ctx, s, engine, repo, _ := fixture(t)
	r := create(t, ctx, s, "anon:alice")
	r = publish(t, ctx, s, r)
	first := r.Releases[0]
	lock, err := s.Resolve(ctx, r.Owner, []pack.Release{first.Release})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resolve(ctx, "anon:bob", []pack.Release{first.Release}); err == nil {
		t.Fatal("private release resolved")
	}
	if _, err = s.Export(ctx, "anon:bob", r.ID, "1.0.0"); err == nil {
		t.Fatal("private release exported")
	}
	if err = s.AuthorizeLock(ctx, "anon:bob", lock, pack.Lock{}); err == nil {
		t.Fatal("forged lock accepted")
	}
	if _, err = s.Publish(ctx, r.Owner, r.ID, r.Revision); err == nil {
		t.Fatal("immutable release overwritten")
	}
	var doc map[string]any
	_ = json.Unmarshal(r.Draft, &doc)
	doc["manifest"].(map[string]any)["version"] = "1.1.0"
	b, _ := json.Marshal(doc)
	newer, err := s.Save(ctx, r.Owner, r.ID, r.Title, r.Revision, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Save(ctx, r.Owner, r.ID, r.Title, r.Revision, b, nil); err == nil {
		t.Fatal("stale save succeeded")
	}
	newer = publish(t, ctx, s, newer)
	if newer.Releases[0].Release != first.Release {
		t.Fatal("old release changed")
	}
	// A fresh adapter can reconstruct pinned contexts from the repository.
	base, err := file.NewRegistry([]string{"../../../data/pack/srd-5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	fresh := file.NewAuthoring(base, repo)
	if _, err = fresh.LoadLocked(ctx, "en", lock); err != nil {
		t.Fatal(err)
	}
	copy, err := s.Create(ctx, user.User{ID: "anon:bob", Anonymous: true, DisplayName: "Bob"}, "Imported", first.Data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if copy.ID == r.ID {
		t.Fatal("import retained identity")
	}
	copy = publish(t, ctx, s, copy)
	if copy.Releases[0].Release.Version != "1.0.0" {
		t.Fatal("incorrect initial version")
	}
	if _, err = engine.LoadLocked(ctx, "en", lock); err != nil {
		t.Fatal(err)
	}
}
func TestSharingAndRetainedCharacterAccess(t *testing.T) {
	ctx, s, _, _, groups := fixture(t)
	r := publish(t, ctx, s, create(t, ctx, s, "anon:alice"))
	_ = create(t, ctx, s, "anon:bob")
	_ = create(t, ctx, s, "anon:carol")
	if err := groups.Create(ctx, group.Group{ID: "table", Name: "Table", CreatedAt: time.Now()}, "anon:carol"); err != nil {
		t.Fatal(err)
	}
	for _, u := range []user.ID{"anon:alice", "anon:bob"} {
		if err := groups.AddMember(ctx, "table", u, group.RolePlayer, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Share(ctx, r.Owner, "table", r.ID, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	lock, err := s.Resolve(ctx, "anon:bob", []pack.Release{r.Releases[0].Release})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Unshare(ctx, "anon:bob", "table", r.ID); err == nil {
		t.Fatal("player removed somebody else's share")
	}
	if err = s.Unshare(ctx, "anon:carol", "table", r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resolve(ctx, "anon:bob", []pack.Release{r.Releases[0].Release}); err == nil {
		t.Fatal("unshared release selectable")
	}
	if err = s.AuthorizeLock(ctx, "anon:bob", lock, lock); err != nil {
		t.Fatalf("existing character lost access: %v", err)
	}
	if err = s.AuthorizeLock(ctx, "anon:bob", lock, pack.Lock{}); err == nil {
		t.Fatal("new character accepted removed release")
	}
}
func TestInvalidDraftCanSaveButNotPublish(t *testing.T) {
	ctx, s, _, _, _ := fixture(t)
	r := create(t, ctx, s, "anon:alice")
	var doc map[string]any
	_ = json.Unmarshal(r.Draft, &doc)
	doc["manifest"].(map[string]any)["dependencies"] = []any{map[string]any{"id": "missing", "version": "1.0.0"}}
	b, _ := json.Marshal(doc)
	r, err := s.Save(ctx, r.Owner, r.ID, r.Title, r.Revision, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, r.Owner, r.ID, r.Revision); err == nil {
		t.Fatal("missing dependency published")
	}
	if _, err = s.Create(ctx, user.User{ID: r.Owner}, "Bad", []byte(`{"manifest":{},"manifest":{}}`), nil); err == nil {
		t.Fatal("duplicate key accepted")
	}
}
